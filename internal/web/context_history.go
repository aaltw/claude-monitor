package web

import "sync"

// ContextHistoryBuffer is a thread-safe circular buffer of context messages.
type ContextHistoryBuffer struct {
	mu   sync.RWMutex
	buf  []ContextMsg
	head int
	len  int
	cap  int
}

// NewContextHistoryBuffer creates a new ContextHistoryBuffer with the given capacity.
func NewContextHistoryBuffer(capacity int) *ContextHistoryBuffer {
	return &ContextHistoryBuffer{
		buf: make([]ContextMsg, capacity),
		cap: capacity,
	}
}

// Add appends a ContextMsg to the circular buffer, overwriting the oldest entry when full.
func (cb *ContextHistoryBuffer) Add(msg ContextMsg) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.buf[cb.head] = msg
	cb.head = (cb.head + 1) % cb.cap
	if cb.len < cb.cap {
		cb.len++
	}
}

// Entries returns all stored messages in chronological order.
func (cb *ContextHistoryBuffer) Entries() []ContextMsg {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if cb.len == 0 {
		return nil
	}
	result := make([]ContextMsg, cb.len)
	start := (cb.head - cb.len + cb.cap) % cb.cap
	for i := 0; i < cb.len; i++ {
		result[i] = cb.buf[(start+i)%cb.cap]
	}
	return result
}
