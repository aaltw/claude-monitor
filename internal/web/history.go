package web

import "sync"

// HistoryBuffer is a thread-safe circular buffer of history messages.
type HistoryBuffer struct {
	mu   sync.RWMutex
	buf  []HistoryMsg
	head int
	len  int
	cap  int
}

func NewHistoryBuffer(capacity int) *HistoryBuffer {
	return &HistoryBuffer{
		buf: make([]HistoryMsg, capacity),
		cap: capacity,
	}
}

func (hb *HistoryBuffer) Add(msg HistoryMsg) {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.buf[hb.head] = msg
	hb.head = (hb.head + 1) % hb.cap
	if hb.len < hb.cap {
		hb.len++
	}
}

func (hb *HistoryBuffer) Entries() []HistoryMsg {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	if hb.len == 0 {
		return nil
	}
	result := make([]HistoryMsg, hb.len)
	start := (hb.head - hb.len + hb.cap) % hb.cap
	for i := 0; i < hb.len; i++ {
		result[i] = hb.buf[(start+i)%hb.cap]
	}
	return result
}
