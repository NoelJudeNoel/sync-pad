package room

import (
	"log/slog"
	"sync"
	"time"
)

type Manager struct {
	rooms           map[string]*Room
	mu              sync.RWMutex
	roomTTL         time.Duration
	cleanupInterval time.Duration
}

func NewManager(roomTTL, cleanupInterval time.Duration) *Manager {
	m := &Manager{
		rooms:           make(map[string]*Room),
		roomTTL:         roomTTL,
		cleanupInterval: cleanupInterval,
	}
	if roomTTL <= 0 {
		m.roomTTL = 30 * time.Minute
	}
	if cleanupInterval <= 0 {
		m.cleanupInterval = 5 * time.Minute
	}
	go m.cleanupLoop()
	return m
}

func (m *Manager) GetOrCreate(roomID string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.rooms[roomID]; ok {
		return r
	}

	r := NewRoom(roomID, m.roomTTL)
	m.rooms[roomID] = r
	slog.Info("room created", "room", roomID)
	return r
}

func (m *Manager) Delete(roomID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, roomID)
	slog.Info("room deleted", "room", roomID)
}

func (m *Manager) RoomCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}

func (m *Manager) TotalClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := 0
	for _, r := range m.rooms {
		total += r.ClientCount()
	}
	return total
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	now := time.Now()
	expired := []string{}
	for id, r := range m.rooms {
		r.mu.RLock()
		expiry := r.Expiry
		r.mu.RUnlock()
		if now.After(expiry) {
			expired = append(expired, id)
		}
	}
	for _, id := range expired {
		delete(m.rooms, id)
	}
	m.mu.Unlock()

	if len(expired) > 0 {
		slog.Info("cleanup expired rooms", "count", len(expired))
	}
}
