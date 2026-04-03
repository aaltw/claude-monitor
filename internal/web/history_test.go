package web

import (
	"testing"
	"time"
)

func TestHistoryBufferEmpty(t *testing.T) {
	hb := NewHistoryBuffer(10)
	entries := hb.Entries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestHistoryBufferAdd(t *testing.T) {
	hb := NewHistoryBuffer(10)
	hb.Add(HistoryMsg{Type: "history", Timestamp: time.Now(), FiveHourPct: 45.0, TotalTokens: 1000})
	entries := hb.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].FiveHourPct != 45.0 {
		t.Errorf("expected 45.0, got %f", entries[0].FiveHourPct)
	}
}

func TestHistoryBufferWraps(t *testing.T) {
	hb := NewHistoryBuffer(3)
	for i := 0; i < 5; i++ {
		hb.Add(HistoryMsg{Type: "history", Timestamp: time.Now(), TotalTokens: i * 100})
	}
	entries := hb.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].TotalTokens != 200 {
		t.Errorf("expected first entry tokens=200, got %d", entries[0].TotalTokens)
	}
	if entries[2].TotalTokens != 400 {
		t.Errorf("expected last entry tokens=400, got %d", entries[2].TotalTokens)
	}
}
