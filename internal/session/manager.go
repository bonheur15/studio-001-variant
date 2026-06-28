package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	ID          string
	Connections map[string]any
	CreatedAt   time.Time
	LastActive  time.Time
	mu          sync.RWMutex
}

func (s *Session) AddConnection(name string, conn any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Connections[name] = conn
}

func (s *Session) GetConnection(name string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.Connections[name]
	return v, ok
}

func (s *Session) RemoveConnection(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Connections, name)
}

type Manager struct {
	sessions sync.Map
	stopCh   chan struct{}
}

func NewManager() *Manager {
	m := &Manager{
		stopCh: make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) Create(id string) *Session {
	s := &Session{
		ID:          id,
		Connections: make(map[string]any),
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
	}
	m.sessions.Store(id, s)
	return s
}

func (m *Manager) Get(id string) (*Session, bool) {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	s := v.(*Session)
	s.LastActive = time.Now()
	return s, true
}

func (m *Manager) GetOrCreate(id string) *Session {
	if s, ok := m.Get(id); ok {
		return s
	}
	return m.Create(id)
}

func (m *Manager) Delete(id string) {
	if v, ok := m.sessions.Load(id); ok {
		s := v.(*Session)
		s.mu.Lock()
		for name, conn := range s.Connections {
			if closer, ok := conn.(interface{ Close() error }); ok {
				closer.Close()
			}
			delete(s.Connections, name)
		}
		s.mu.Unlock()
		m.sessions.Delete(id)
	}
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.sessions.Range(func(key, value any) bool {
				s := value.(*Session)
				if time.Since(s.LastActive) > 30*time.Minute {
					m.Delete(key.(string))
				}
				return true
			})
		case <-m.stopCh:
			return
		}
	}
}

func GenerateID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
