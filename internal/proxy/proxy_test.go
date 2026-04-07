package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestProxy_InterceptsMessages(t *testing.T) {
	// Mock upstream API
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Hello!"},
			},
			"usage": map[string]any{
				"input_tokens":  1000,
				"output_tokens": 50,
			},
		})
	}))
	defer upstream.Close()

	// Capture what gets pushed
	var pushed ContextSnapshot
	var pushWg sync.WaitGroup
	pushWg.Add(1)
	pushServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &pushed)
		w.WriteHeader(http.StatusOK)
		pushWg.Done()
	}))
	defer pushServer.Close()

	// Create proxy pointing at mock upstream
	p := NewProxy(upstream.URL, pushServer.URL)
	proxyServer := httptest.NewServer(p.Handler())
	defer proxyServer.Close()

	// Send a Messages API request through the proxy
	reqBody := []byte(`{
		"model": "claude-sonnet-4-6",
		"system": "You are helpful.",
		"tools": [{"name": "Read", "description": "Reads files", "input_schema": {"type": "object"}}],
		"messages": [{"role": "user", "content": "Hello"}]
	}`)
	resp, err := http.Post(proxyServer.URL+"/v1/messages", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Wait for the goroutine to push (with timeout)
	done := make(chan struct{})
	go func() { pushWg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for composition push")
	}

	// Verify composition was pushed
	if pushed.TotalInputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", pushed.TotalInputTokens)
	}
	if pushed.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %s", pushed.Model)
	}
	if len(pushed.Categories) != 5 {
		t.Errorf("expected 5 categories, got %d", len(pushed.Categories))
	}
	if pushed.Categories[CatSystem].Tokens == 0 {
		t.Error("expected non-zero system tokens")
	}
}

func TestProxy_NonMessagesPassthrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer upstream.Close()

	p := NewProxy(upstream.URL, "http://127.0.0.1:1") // push addr irrelevant
	proxyServer := httptest.NewServer(p.Handler())
	defer proxyServer.Close()

	resp, err := http.Get(proxyServer.URL + "/v1/models")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
