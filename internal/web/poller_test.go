package web

import (
	"testing"
	"time"

	"github.com/aaltwesthuis/claude-monitor/internal/data"
)

func TestBuildStateMsg(t *testing.T) {
	usage := data.UsageSnapshot{
		HasData: true,
		FiveHour: data.WindowUsage{
			UsedPercentage: 45.0,
			ResetsAt:       time.Date(2026, 4, 3, 21, 0, 0, 0, time.UTC),
			Severity:       "Nominal",
		},
		SevenDay: data.WindowUsage{
			UsedPercentage: 12.0,
			ResetsAt:       time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC),
			Severity:       "Nominal",
		},
		TotalTokens: 500000,
	}
	burnRate := data.BurnRate{
		HasData: true, PercentPerHour: 4.2,
		TokensPerHour: 12000, TimeToExhaust: 2*time.Hour + 15*time.Minute,
	}
	sessions := []data.SessionInfo{
		{PID: 86792, SessionID: "sess-1", Name: "claude-monitor",
			Model: "Opus 4.6", Status: data.StatusWorking,
			LastBridge: time.Now(), Cwd: "/tmp/test"},
	}
	bridges := []data.BridgeData{
		{SessionID: "sess-1", Model: "Opus 4.6", Tokens: data.Tokens{TotalInput: 400000, TotalOutput: 87000}},
		{SessionID: "sess-2", Model: "Sonnet 4.6", Tokens: data.Tokens{TotalInput: 100000, TotalOutput: 89000}},
	}

	msg := BuildStateMsg(usage, burnRate, sessions, bridges)

	if msg.Type != "state" {
		t.Errorf("expected type 'state', got %q", msg.Type)
	}
	if msg.Usage.FiveHour.UsedPct != 45.0 {
		t.Errorf("expected 45.0, got %f", msg.Usage.FiveHour.UsedPct)
	}
	if len(msg.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(msg.Sessions))
	}
	if msg.Sessions[0].Model != "Opus 4.6" {
		t.Errorf("expected model 'Opus 4.6', got %q", msg.Sessions[0].Model)
	}
	if msg.Sessions[0].Status != "working" {
		t.Errorf("expected status 'working', got %q", msg.Sessions[0].Status)
	}
	if len(msg.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(msg.Models))
	}
	opus, ok := msg.Models["Opus 4.6"]
	if !ok {
		t.Fatal("expected 'Opus 4.6' in models map")
	}
	if opus.TotalTokens != 487000 {
		t.Errorf("expected opus tokens 487000, got %d", opus.TotalTokens)
	}
}

func TestBuildHistoryMsg(t *testing.T) {
	usage := data.UsageSnapshot{
		HasData:     true,
		FiveHour:    data.WindowUsage{UsedPercentage: 45.0},
		SevenDay:    data.WindowUsage{UsedPercentage: 12.0},
		TotalTokens: 500000,
	}
	burnRate := data.BurnRate{HasData: true, PercentPerHour: 4.2}
	bridges := []data.BridgeData{
		{Model: "Opus 4.6", Tokens: data.Tokens{TotalInput: 400000, TotalOutput: 87000}},
		{Model: "Sonnet 4.6", Tokens: data.Tokens{TotalInput: 100000, TotalOutput: 89000}},
	}

	msg := BuildHistoryMsg(usage, burnRate, bridges)
	if msg.Type != "history" {
		t.Errorf("expected type 'history', got %q", msg.Type)
	}
	if msg.FiveHourPct != 45.0 {
		t.Errorf("expected 45.0, got %f", msg.FiveHourPct)
	}
	if msg.TokensByModel["Opus 4.6"] != 487000 {
		t.Errorf("expected opus tokens 487000, got %d", msg.TokensByModel["Opus 4.6"])
	}
}

func TestDetectStateChanges(t *testing.T) {
	prev := map[int]string{100: "idle"}
	curr := []data.SessionInfo{
		{PID: 100, Name: "test", Model: "Opus 4.6", Status: data.StatusWorking},
	}
	events := DetectStateChanges(prev, curr)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Detail != "idle → working" {
		t.Errorf("expected 'idle → working', got %q", events[0].Detail)
	}
}

func TestDetectStateChanges_NewSession(t *testing.T) {
	prev := map[int]string{}
	curr := []data.SessionInfo{
		{PID: 200, Name: "new-session", Model: "Sonnet 4.6", Status: data.StatusIdle},
	}
	events := DetectStateChanges(prev, curr)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Action != "session_started" {
		t.Errorf("expected action 'session_started', got %q", events[0].Action)
	}
}
