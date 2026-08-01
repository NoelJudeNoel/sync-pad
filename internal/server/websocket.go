package server

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/NoelJudeNoel/sync-pad/internal/config"
	"github.com/NoelJudeNoel/sync-pad/internal/ratelimit"
	"github.com/NoelJudeNoel/sync-pad/internal/room"
	"github.com/gorilla/websocket"
)

var roomIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

type Server struct {
	rooms    *room.Manager
	cfg      config.Config
	msgLimit *ratelimit.Limiter
	upgrader websocket.Upgrader
}

func New(rooms *room.Manager, cfg config.Config, msgLimit *ratelimit.Limiter) *Server {
	return &Server{
		rooms:    rooms,
		cfg:      cfg,
		msgLimit: msgLimit,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return cfg.IsOriginAllowed(r.Header.Get("Origin"))
			},
		},
	}
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	if s.rooms.TotalClientCount() >= s.cfg.MaxConnections {
		slog.Warn("max connections reached", "total", s.rooms.TotalClientCount())
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
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
	ip := ratelimit.ExtractIP(r)
	client := &room.Client{
		Room:     rm,
		Send:     make(chan []byte, 16),
		LastPong: time.Now(),
		IP:       ip,
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

	conn.SetReadLimit(s.cfg.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(s.cfg.PongWait))
	conn.SetPongHandler(func(string) error {
		c.LastPong = time.Now()
		conn.SetReadDeadline(time.Now().Add(s.cfg.PongWait))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if !s.msgLimit.Allow(c.IP) {
			slog.Warn("rate limit hit (msg)", "room", c.Room.ID, "ip", c.IP)
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(1008, "Policy Violation: rate limit"))
			return
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
	ticker := time.NewTicker(s.cfg.PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.Send:
			conn.SetWriteDeadline(time.Now().Add(s.cfg.WriteTimeout))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(s.cfg.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			if time.Since(c.LastPong) > s.cfg.PongWait {
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

// roomIDEncoding produces lowercase, unpadded base32 room IDs. Unlike a
// naive modulo-36 mapping over random bytes (256 % 36 != 0, which biases
// the first few characters of the alphabet by ~11%), base32 maps 5 bits of
// entropy directly onto each output character with no bias.
var roomIDEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

func generateRoomID() string {
	// 10 random bytes -> 16 base32 characters, matching the previous ID length.
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read only fails if the OS entropy source is
		// unavailable, which is unrecoverable; fail loudly rather than
		// silently handing out a predictable room ID.
		panic("sync-pad: failed to read random bytes for room ID: " + err.Error())
	}
	return roomIDEncoding.EncodeToString(b)
}
