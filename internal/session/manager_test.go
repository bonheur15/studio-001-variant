package session

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("GenerateID returned empty string")
	}
	if len(id1) != 32 {
		t.Errorf("GenerateID length = %d, want 32", len(id1))
	}
	if id1 == id2 {
		t.Error("GenerateID returned duplicate IDs")
	}
}

func TestCreateAndGet(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	s := m.Create("test-session")
	if s == nil {
		t.Fatal("Create returned nil")
	}
	if s.ID != "test-session" {
		t.Errorf("Session.ID = %q, want %q", s.ID, "test-session")
	}

	got, ok := m.Get("test-session")
	if !ok {
		t.Fatal("Get returned not ok")
	}
	if got.ID != s.ID {
		t.Errorf("Get returned session with ID %q, want %q", got.ID, s.ID)
	}
}

func TestGetNonExistent(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	_, ok := m.Get("nonexistent")
	if ok {
		t.Error("Get returned ok for non-existent session")
	}
}

func TestGetOrCreate(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	s1 := m.GetOrCreate("test-session")
	s2 := m.GetOrCreate("test-session")

	if s1.ID != s2.ID {
		t.Error("GetOrCreate returned different sessions for same ID")
	}
}

func TestDelete(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	m.Create("test-session")

	m.Delete("test-session")
	_, ok := m.Get("test-session")
	if ok {
		t.Error("Get returned ok after Delete")
	}
}

func TestSessionConnections(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	s := m.Create("test-session")

	s.AddConnection("conn1", "mock-connection-1")
	s.AddConnection("conn2", "mock-connection-2")

	v, ok := s.GetConnection("conn1")
	if !ok {
		t.Error("GetConnection returned not ok")
	}
	if v.(string) != "mock-connection-1" {
		t.Errorf("GetConnection = %v, want %v", v, "mock-connection-1")
	}

	s.RemoveConnection("conn1")
	_, ok = s.GetConnection("conn1")
	if ok {
		t.Error("GetConnection returned ok after RemoveConnection")
	}
}

func TestDeleteClosesConnections(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	closed := false
	mockConn := &mockCloser{closeFn: func() { closed = true }}

	s := m.Create("test-session")
	s.AddConnection("mock", mockConn)

	m.Delete("test-session")

	if !closed {
		t.Error("Delete did not close connections")
	}
}

type mockCloser struct {
	closeFn func()
}

func (m *mockCloser) Close() error {
	m.closeFn()
	return nil
}

func TestLastActiveUpdated(t *testing.T) {
	m := NewManager()
	defer m.Stop()

	s := m.Create("test-session")
	initial := s.LastActive

	time.Sleep(time.Millisecond)

	m.Get("test-session")
	if !s.LastActive.After(initial) {
		t.Error("Get did not update LastActive")
	}
}

func TestGenerateIDUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateID()
		if ids[id] {
			t.Error("Duplicate ID generated")
		}
		ids[id] = true
	}
}
