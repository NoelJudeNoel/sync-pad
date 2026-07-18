package room

import (
	"sync"
	"time"
)

type Client struct {
	Room     *Room
	Send     chan []byte
	LastPong time.Time
}

type Room struct {
	ID      string
	clients map[*Client]struct{}
	Text    string
	mu      sync.RWMutex
	Expiry  time.Time
}

func NewRoom(id string) *Room {
	return &Room{
		ID:      id,
		clients: make(map[*Client]struct{}),
		Expiry:  time.Now().Add(30 * time.Minute),
	}
}

func (r *Room) AddClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = struct{}{}
	r.Expiry = time.Now().Add(30 * time.Minute)
}

func (r *Room) RemoveClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
	r.Expiry = time.Now().Add(30 * time.Minute)
}

func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *Room) GetText() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Text
}

func (r *Room) SetText(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Text = text
	r.Expiry = time.Now().Add(30 * time.Minute)
}

func (r *Room) Broadcast(sender *Client, msg []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for c := range r.clients {
		if c != sender {
			select {
			case c.Send <- msg:
			default:
			}
		}
	}
}

func (r *Room) BroadcastAll(msg []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for c := range r.clients {
		select {
		case c.Send <- msg:
		default:
		}
	}
}


