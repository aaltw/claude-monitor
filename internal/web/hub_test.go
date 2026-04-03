package web

import (
	"testing"
	"time"
)

func TestHubRegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch := make(chan []byte, 16)
	hub.Register(ch)
	time.Sleep(10 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Unregister(ch)
	time.Sleep(10 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch := make(chan []byte, 16)
	hub.Register(ch)
	time.Sleep(10 * time.Millisecond)

	msg := []byte(`{"type":"test"}`)
	hub.Broadcast(msg)

	select {
	case received := <-ch:
		if string(received) != string(msg) {
			t.Errorf("expected %q, got %q", msg, received)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}
