package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadSessionRegistry_Empty(t *testing.T) {
	dir := t.TempDir()
	regs, err := ReadSessionRegistry(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(regs) != 0 {
		t.Fatalf("expected 0, got %d", len(regs))
	}
}

func TestReadSessionRegistry_Valid(t *testing.T) {
	dir := t.TempDir()

	reg := SessionRegistry{
		PID:       12345,
		SessionID: "sess-abc",
		Cwd:       "/Users/test/project",
		Name:      "my-session",
		StartedAt: time.Now().UnixMilli(),
		Kind:      "interactive",
	}
	data, _ := json.Marshal(reg)
	os.WriteFile(filepath.Join(dir, "12345.json"), data, 0644)

	regs, err := ReadSessionRegistry(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(regs) != 1 {
		t.Fatalf("expected 1, got %d", len(regs))
	}
	if regs[0].Name != "my-session" {
		t.Fatalf("expected name my-session, got %s", regs[0].Name)
	}
}

func TestMergeSessions(t *testing.T) {
	pid := os.Getpid() // use current PID so it's alive

	registries := []SessionRegistry{
		{PID: pid, SessionID: "sess-1", Cwd: "/home/user/project-alpha", Name: "alpha", StartedAt: time.Now().UnixMilli()},
	}
	states := map[int]SessionState{
		pid: {PID: pid, SessionID: "sess-1", State: "working", UpdatedAt: time.Now().Unix()},
	}
	bridges := []BridgeData{
		{SessionID: "sess-1", Timestamp: time.Now().Unix()},
	}

	sessions := MergeSessions(registries, states, bridges)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Status != StatusWorking {
		t.Fatalf("expected WORKING, got %v", sessions[0].Status)
	}
	if sessions[0].Name != "alpha" {
		t.Fatalf("expected name alpha, got %s", sessions[0].Name)
	}
	if !sessions[0].Alive {
		t.Fatal("expected alive=true for current process PID")
	}
}

func TestMergeSessions_NoName_UsesCwd(t *testing.T) {
	pid := os.Getpid()
	registries := []SessionRegistry{
		{PID: pid, SessionID: "sess-2", Cwd: "/home/user/my-project"},
	}

	sessions := MergeSessions(registries, nil, nil)
	if sessions[0].Name != "my-project" {
		t.Fatalf("expected name from cwd 'my-project', got %s", sessions[0].Name)
	}
}

func TestMergeSessions_DeadPID(t *testing.T) {
	// Use a PID that definitely doesn't exist
	deadPID := 99999999
	registries := []SessionRegistry{
		{PID: deadPID, SessionID: "sess-dead", Cwd: "/tmp/dead", Name: "dead-session"},
	}

	sessions := MergeSessions(registries, nil, nil)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Status != StatusZombie {
		t.Fatalf("expected ZOMBIE, got %v", sessions[0].Status)
	}
	if sessions[0].Alive {
		t.Fatal("expected alive=false for dead PID")
	}
}

func TestSessionHexID(t *testing.T) {
	result := SessionHexID(44946)
	expected := "0xAF92"
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestMergeSessionsPopulatesModel(t *testing.T) {
	pid := os.Getpid()
	registries := []SessionRegistry{
		{PID: pid, SessionID: "sess-1", Cwd: "/tmp/test", StartedAt: 1000},
	}
	bridges := []BridgeData{
		{SessionID: "sess-1", Timestamp: time.Now().Unix(), Model: "Opus 4.6"},
	}

	sessions := MergeSessions(registries, nil, bridges)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Model != "Opus 4.6" {
		t.Errorf("expected model 'Opus 4.6', got %q", sessions[0].Model)
	}
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		since time.Duration
		want  string
	}{
		{0, "INF"},
		{500 * time.Millisecond, "500ms"},
		{2 * time.Second, "2s"},
		{90 * time.Second, "1m"},
		{5 * time.Minute, "5m"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.since), func(t *testing.T) {
			var lastBridge time.Time
			if tt.since > 0 {
				lastBridge = time.Now().Add(-tt.since)
			}
			got := FormatLatency(lastBridge)
			if got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}
