package room

import (
	"testing"
	"time"
)

func TestRoomAddRemoveClient(t *testing.T) {
	r := NewRoom("test-room", 0)
	c1 := &Client{}
	c2 := &Client{}

	r.AddClient(c1)
	if r.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", r.ClientCount())
	}

	r.AddClient(c2)
	if r.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", r.ClientCount())
	}

	r.RemoveClient(c1)
	if r.ClientCount() != 1 {
		t.Fatalf("expected 1 client after remove, got %d", r.ClientCount())
	}
}

func TestRoomText(t *testing.T) {
	r := NewRoom("test-room", 0)
	r.SetText("hello")
	if r.GetText() != "hello" {
		t.Fatalf("expected 'hello', got '%s'", r.GetText())
	}
}

func TestRoomExpiry(t *testing.T) {
	r := NewRoom("test-room", 0)
	r.Expiry = time.Now().Add(-1 * time.Minute)

	if time.Now().Before(r.Expiry) {
		t.Fatal("expiry should be in the past")
	}
}

func TestManagerGetOrCreate(t *testing.T) {
	m := NewManager(0, 0)

	r1 := m.GetOrCreate("room-a")
	r2 := m.GetOrCreate("room-a")
	if r1 != r2 {
		t.Fatal("GetOrCreate should return same room for same ID")
	}

	if m.RoomCount() != 1 {
		t.Fatalf("expected 1 room, got %d", m.RoomCount())
	}
}

func TestManagerDelete(t *testing.T) {
	m := NewManager(0, 0)

	m.GetOrCreate("room-b")
	m.Delete("room-b")
	if m.RoomCount() != 0 {
		t.Fatalf("expected 0 rooms after delete, got %d", m.RoomCount())
	}
}
