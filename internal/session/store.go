package session

import "sync"

// Store is a thread-safe in-memory store for sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewStore creates an empty session store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

// Get retrieves a session by ID. Returns nil if not found.
func (st *Store) Get(id string) *Session {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.sessions[id]
}

// GetOrCreate retrieves a session by ID, creating a new one if it doesn't exist.
func (st *Store) GetOrCreate(id string) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()

	if s, ok := st.sessions[id]; ok {
		return s
	}

	s := New(id)
	st.sessions[id] = s
	return s
}
