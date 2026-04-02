package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadBridgeFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	bridges, err := ReadBridgeFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bridges) != 0 {
		t.Fatalf("expected 0 bridges, got %d", len(bridges))
	}
}

func TestReadBridgeFiles_Valid(t *testing.T) {
	dir := t.TempDir()

	bd := BridgeData{
		SessionID: "test-session-1",
		Timestamp: time.Now().Unix(),
		RateLimits: RateLimits{
			FiveHour: WindowLimit{UsedPercentage: 25.0, ResetsAt: float64(time.Now().Add(3 * time.Hour).Unix())},
			SevenDay: WindowLimit{UsedPercentage: 60.0, ResetsAt: float64(time.Now().Add(5 * 24 * time.Hour).Unix())},
		},
		Tokens: Tokens{Input: 100, Output: 200, TotalInput: 5000, TotalOutput: 3000},
		Model:  "Claude Sonnet 4",
		Cwd:    "/tmp/test",
	}
	data, _ := json.Marshal(bd)
	os.WriteFile(filepath.Join(dir, "claude-monitor-abc123.json"), data, 0644)

	bridges, err := ReadBridgeFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bridges) != 1 {
		t.Fatalf("expected 1 bridge, got %d", len(bridges))
	}
	if bridges[0].SessionID != "test-session-1" {
		t.Fatalf("expected session test-session-1, got %s", bridges[0].SessionID)
	}
	if !bridges[0].HasRateLimits() {
		t.Fatal("expected HasRateLimits=true")
	}
}

func TestReadBridgeFiles_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "claude-monitor-bad.json"), []byte("{invalid"), 0644)

	bridges, err := ReadBridgeFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Malformed files are silently skipped
	if len(bridges) != 0 {
		t.Fatalf("expected 0 bridges (malformed skipped), got %d", len(bridges))
	}
}

func TestLatestUsage_MostRecent(t *testing.T) {
	now := time.Now()
	bridges := []BridgeData{
		{SessionID: "old", Timestamp: now.Add(-5 * time.Minute).Unix(), RateLimits: RateLimits{FiveHour: WindowLimit{UsedPercentage: 10.0, ResetsAt: 1}}},
		{SessionID: "new", Timestamp: now.Unix(), RateLimits: RateLimits{FiveHour: WindowLimit{UsedPercentage: 50.0, ResetsAt: 1}}},
	}
	usage := LatestUsage(bridges)
	if !usage.HasData {
		t.Fatal("expected HasData=true")
	}
	if usage.FiveHour.UsedPercentage != 50.0 {
		t.Fatalf("expected 50%% from most recent bridge, got %.1f", usage.FiveHour.UsedPercentage)
	}
}

func TestLatestUsage_NoRateLimits(t *testing.T) {
	bridges := []BridgeData{
		{SessionID: "api-session", Timestamp: time.Now().Unix(), RateLimits: RateLimits{}},
	}
	usage := LatestUsage(bridges)
	if usage.HasData {
		t.Fatal("expected HasData=false for bridge without rate limits")
	}
}
