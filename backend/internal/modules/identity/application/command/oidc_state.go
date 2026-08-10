package command

import (
	"sync"
	"time"
)

type oidcStateEntry struct {
	expiresAt time.Time
}

type OIDCStateStore struct {
	mu    sync.Mutex
	items map[string]oidcStateEntry
}

func NewOIDCStateStore() *OIDCStateStore {
	return &OIDCStateStore{items: make(map[string]oidcStateEntry)}
}

func (s *OIDCStateStore) Put(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[state] = oidcStateEntry{expiresAt: time.Now().Add(10 * time.Minute)}
}

func (s *OIDCStateStore) Consume(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.items[state]
	if !ok {
		return false
	}
	delete(s.items, state)
	return time.Now().Before(entry.expiresAt)
}
