package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPushComposition_Success(t *testing.T) {
	var received ContextSnapshot
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/internal/context" {
			t.Errorf("expected /api/internal/context, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	snap := ContextSnapshot{
		SessionID:        "test-session",
		Timestamp:        time.Now(),
		Model:            "claude-sonnet-4-6",
		TotalInputTokens: 5000,
		Categories: map[string]CategoryTokens{
			CatSystem: {Tokens: 500, Pct: 10.0, Bytes: 1500},
		},
	}

	err := PushComposition(ts.URL, snap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.SessionID != "test-session" {
		t.Errorf("expected session test-session, got %s", received.SessionID)
	}
}

func TestPushComposition_ServerDown(t *testing.T) {
	snap := ContextSnapshot{SessionID: "test"}
	// Use an address that will refuse connection
	err := PushComposition("http://127.0.0.1:1", snap)
	if err == nil {
		t.Error("expected error when server is down")
	}
}
