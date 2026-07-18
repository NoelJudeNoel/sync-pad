package server

import (
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/eu-as/sync-speech/internal/config"
	"github.com/eu-as/sync-speech/internal/room"
	"github.com/gorilla/websocket"
)

var roomIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || origin == "https://eu-as.cn" || origin == "http://localhost"
	},
}

type Server struct {
	rooms *room.Manager
}

func New(rooms *room.Manager) *Server {
	return &Server{rooms: rooms}
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "error", err)
		return
	}

	rawRoom := r.URL.Query().Get("room")
	roomID := rawRoom
	if !roomIDPattern.MatchString(rawRoom) {
		roomID = generateRoomID()
	}

	rm := s.rooms.GetOrCreate(roomID)
	client := &room.Client{
		Room:     rm,
		Send:     make(chan []byte, 16),
		LastPong: time.Now(),
	}

	rm.AddClient(client)

	slog.Info("client joined", "room", roomID, "clients", rm.ClientCount())

	if rm.ClientCount() > 1 {
		notify, _ := json.Marshal(map[string]string{"t": "j"})
		rm.BroadcastAll(notify)
	}

	initMsg, _ := json.Marshal(map[string]string{"d": rm.GetText()})
	select {
	case client.Send <- initMsg:
	default:
	}

	go s.writePump(client, conn)
	s.readPump(client, conn)
}

func (s *Server) readPump(c *room.Client, conn *websocket.Conn) {
	defer s.disconnect(c)

	conn.SetReadLimit(config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(config.PongWait))
	conn.SetPongHandler(func(string) error {
		c.LastPong = time.Now()
		conn.SetReadDeadline(time.Now().Add(config.PongWait))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg struct {
			D string `json:"d"`
			P *int   `json:"p"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("invalid message", "room", c.Room.ID, "error", err)
			continue
		}

		c.Room.SetText(msg.D)

		resp := map[string]any{"d": msg.D}
		if msg.P != nil {
			resp["p"] = *msg.P
		}
		respData, _ := json.Marshal(resp)
		c.Room.Broadcast(c, respData)
	}
}

func (s *Server) writePump(c *room.Client, conn *websocket.Conn) {
	ticker := time.NewTicker(config.PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.Send:
			conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			if time.Since(c.LastPong) > config.PongWait {
				return
			}
		}
	}
}

func (s *Server) disconnect(c *room.Client) {
	c.Room.RemoveClient(c)
	close(c.Send)

	remaining := c.Room.ClientCount()
	slog.Info("client left", "room", c.Room.ID, "remaining", remaining)

	if remaining == 0 {
		s.rooms.Delete(c.Room.ID)
	} else {
		notify, _ := json.Marshal(map[string]any{"t": "l", "c": remaining})
		c.Room.BroadcastAll(notify)
	}
}

func generateRoomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 16)
	for i := range b {
		result[i] = chars[b[i]%byte(len(chars))]
	}
	return string(result)
}
