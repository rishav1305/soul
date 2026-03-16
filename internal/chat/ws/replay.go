package ws

import "sync"

type replayEntry struct {
	messageID string
	data      []byte
}

// ReplayBuffer maintains a bounded per-session message buffer for replay after reconnect.
type ReplayBuffer struct {
	mu            sync.Mutex
	maxPerSession int
	maxSessions   int
	sessions      map[string][]replayEntry
	sessionOrder  []string // LRU order
}

func NewReplayBuffer(maxPerSession, maxSessions int) *ReplayBuffer {
	return &ReplayBuffer{
		maxPerSession: maxPerSession,
		maxSessions:   maxSessions,
		sessions:      make(map[string][]replayEntry),
	}
}

func (rb *ReplayBuffer) Store(sessionID, messageID string, data []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, ok := rb.sessions[sessionID]; !ok {
		if len(rb.sessionOrder) >= rb.maxSessions {
			oldest := rb.sessionOrder[0]
			rb.sessionOrder = rb.sessionOrder[1:]
			delete(rb.sessions, oldest)
		}
		rb.sessionOrder = append(rb.sessionOrder, sessionID)
		rb.sessions[sessionID] = nil
	}

	entries := rb.sessions[sessionID]
	entries = append(entries, replayEntry{messageID: messageID, data: data})
	if len(entries) > rb.maxPerSession {
		entries = entries[len(entries)-rb.maxPerSession:]
	}
	rb.sessions[sessionID] = entries

	// Move to end of LRU
	for i, s := range rb.sessionOrder {
		if s == sessionID {
			rb.sessionOrder = append(rb.sessionOrder[:i], rb.sessionOrder[i+1:]...)
			rb.sessionOrder = append(rb.sessionOrder, sessionID)
			break
		}
	}
}

// Replay returns all messages after the anchor, plus a bool indicating whether
// the session+anchor was found. (found=true, msgs=nil) means "caught up, nothing missed".
// (found=false) means the session was evicted or the anchor expired.
func (rb *ReplayBuffer) Replay(sessionID, afterMessageID string) ([][]byte, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	entries, ok := rb.sessions[sessionID]
	if !ok || afterMessageID == "" {
		return nil, false
	}

	anchorIdx := -1
	for i, e := range entries {
		if e.messageID == afterMessageID {
			anchorIdx = i
			break
		}
	}
	if anchorIdx == -1 {
		return nil, false // anchor evicted
	}

	var result [][]byte
	for i := anchorIdx + 1; i < len(entries); i++ {
		cp := make([]byte, len(entries[i].data))
		copy(cp, entries[i].data)
		result = append(result, cp)
	}
	return result, true
}

func (rb *ReplayBuffer) Clear(sessionID string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	delete(rb.sessions, sessionID)
	for i, s := range rb.sessionOrder {
		if s == sessionID {
			rb.sessionOrder = append(rb.sessionOrder[:i], rb.sessionOrder[i+1:]...)
			break
		}
	}
}
