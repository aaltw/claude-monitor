# Web Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an HTTP/WebSocket web dashboard to claude-monitor, serving the same data as the TUI with added charts, per-model breakdown, and event log.

**Architecture:** Go HTTP server (`internal/web/`) polls `internal/data/` on the same intervals as the TUI, broadcasts JSON via WebSocket to a static frontend (`web/`). Static files embedded via `go:embed`, with `--dev` flag for disk serving. Entry point extended with `web` subcommand.

**Tech Stack:** Go stdlib + nhooyr.io/websocket, Chart.js (CDN), Tailwind CSS (CDN), Space Grotesk + JetBrains Mono (Google Fonts)

---

## File Structure

### New files

| File | Responsibility |
|------|---------------|
| `internal/web/server.go` | HTTP server setup, static file serving, WebSocket upgrade |
| `internal/web/hub.go` | WebSocket client registry, broadcast to all clients |
| `internal/web/hub_test.go` | Hub register/unregister/broadcast tests |
| `internal/web/poller.go` | Data polling goroutine, state/history/event message assembly |
| `internal/web/poller_test.go` | Poller message assembly tests |
| `internal/web/history.go` | Ring buffer of history points for chart backfill |
| `internal/web/history_test.go` | History buffer tests |
| `internal/web/messages.go` | JSON message types for WebSocket protocol |
| `internal/web/embed.go` | `go:embed` directive for `web/` directory |
| `web/index.html` | Dashboard HTML shell with bento grid layout |
| `web/style.css` | Catppuccin Mocha theme, grid, progress bars, animations |
| `web/app.js` | WebSocket client, DOM updates, Chart.js charts |

### Modified files

| File | Change |
|------|--------|
| `cmd/monitor/main.go` | Add `web` subcommand with `--port` and `--dev` flags |
| `go.mod` | Add `nhooyr.io/websocket` dependency |
| `internal/data/types.go` | Add `Model` field to `SessionInfo` |
| `internal/data/sessions.go` | Populate `Model` field from bridge data during merge |

---

### Task 1: Add WebSocket dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add nhooyr.io/websocket**

Run:
```bash
go get nhooyr.io/websocket@latest
```

- [ ] **Step 2: Verify module downloads**

Run:
```bash
go mod tidy
```
Expected: Clean exit, `go.sum` updated with websocket entries.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add nhooyr.io/websocket dependency"
```

---

### Task 2: Add Model field to SessionInfo

**Files:**
- Modify: `internal/data/types.go`
- Modify: `internal/data/sessions.go`
- Test: `internal/data/sessions_test.go`

- [ ] **Step 1: Write failing test for Model field population**

Add to `internal/data/sessions_test.go`:

```go
func TestMergeSessionsPopulatesModel(t *testing.T) {
	registries := []SessionRegistry{
		{PID: 100, SessionID: "sess-1", Cwd: "/tmp/test", StartedAt: 1000},
	}
	states := map[int]SessionState{}
	bridges := []BridgeData{
		{SessionID: "sess-1", Timestamp: 1000, Model: "Opus 4.6"},
	}

	sessions := MergeSessions(registries, states, bridges)

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Model != "Opus 4.6" {
		t.Errorf("expected model 'Opus 4.6', got %q", sessions[0].Model)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/data/ -run TestMergeSessionsPopulatesModel -v`
Expected: FAIL — `sessions[0].Model` does not exist.

- [ ] **Step 3: Add Model field to SessionInfo**

In `internal/data/types.go`, add `Model` to `SessionInfo`:

```go
type SessionInfo struct {
	PID         int
	SessionID   string
	Name        string
	Cwd         string
	Model       string // model name from bridge file, e.g. "Opus 4.6"
	Status      SessionStatus
	LastBridge  time.Time
	StartedAt   time.Time
	Alive       bool
	ZombieSince time.Time
}
```

- [ ] **Step 4: Populate Model in MergeSessions**

In `internal/data/sessions.go`, inside the `MergeSessions` loop, after the `LastBridge` assignment (line ~102), add:

```go
		// Latency and model from bridge file
		if b, ok := bridgeBySession[reg.SessionID]; ok {
			info.LastBridge = time.Unix(b.Timestamp, 0)
			info.Model = b.Model
		}
```

Remove the existing duplicate block that only sets `LastBridge`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/data/ -v`
Expected: All tests pass including the new one.

- [ ] **Step 6: Commit**

```bash
git add internal/data/types.go internal/data/sessions.go internal/data/sessions_test.go
git commit -m "feat: add Model field to SessionInfo from bridge data"
```

---

### Task 3: WebSocket message types

**Files:**
- Create: `internal/web/messages.go`

- [ ] **Step 1: Create the messages file**

Create `internal/web/messages.go`:

```go
package web

import "time"

// StateMsg is the full dashboard state pushed to clients.
type StateMsg struct {
	Type     string           `json:"type"`
	Usage    UsageMsg         `json:"usage"`
	BurnRate BurnRateMsg      `json:"burn_rate"`
	Sessions []SessionMsg     `json:"sessions"`
	Models   map[string]ModelMsg `json:"models"`
}

type UsageMsg struct {
	HasData  bool       `json:"has_data"`
	IsStale  bool       `json:"is_stale"`
	FiveHour WindowMsg  `json:"five_hour"`
	SevenDay WindowMsg  `json:"seven_day"`
	TotalTokens int    `json:"total_tokens"`
}

type WindowMsg struct {
	UsedPct  float64 `json:"used_pct"`
	ResetsAt string  `json:"resets_at"`
	Severity string  `json:"severity"`
}

type BurnRateMsg struct {
	HasData      bool    `json:"has_data"`
	PctPerHour   float64 `json:"pct_per_hour"`
	TokensPerHour float64 `json:"tokens_per_hour"`
	TTEMinutes   float64 `json:"tte_minutes"`
}

type SessionMsg struct {
	PID    int    `json:"pid"`
	HexID  string `json:"hex_id"`
	Name   string `json:"name"`
	Model  string `json:"model"`
	Status string `json:"status"`
	Latency string `json:"latency"`
	Cwd    string `json:"cwd"`
}

type ModelMsg struct {
	TotalTokens int     `json:"total_tokens"`
	Pct         float64 `json:"pct"`
}

// HistoryMsg is a chart data point pushed periodically.
type HistoryMsg struct {
	Type          string            `json:"type"`
	Timestamp     time.Time         `json:"timestamp"`
	FiveHourPct   float64           `json:"five_hour_pct"`
	SevenDayPct   float64           `json:"seven_day_pct"`
	BurnRatePct   float64           `json:"burn_rate_pct_per_hour"`
	TotalTokens   int               `json:"total_tokens"`
	TokensByModel map[string]int    `json:"tokens_by_model"`
}

// EventMsg is a session state change event.
type EventMsg struct {
	Type      string `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	PID       int    `json:"pid"`
	Session   string `json:"session"`
	Model     string `json:"model"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/web/...`
Expected: Success (no tests yet, just compilation).

- [ ] **Step 3: Commit**

```bash
git add internal/web/messages.go
git commit -m "feat: define WebSocket message types for web dashboard"
```

---

### Task 4: History buffer

**Files:**
- Create: `internal/web/history.go`
- Create: `internal/web/history_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/web/history_test.go`:

```go
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
	msg := HistoryMsg{
		Type:        "history",
		Timestamp:   time.Now(),
		FiveHourPct: 45.0,
		TotalTokens: 1000,
	}
	hb.Add(msg)

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
		hb.Add(HistoryMsg{
			Type:        "history",
			Timestamp:   time.Now(),
			TotalTokens: i * 100,
		})
	}

	entries := hb.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should contain items 2, 3, 4 (oldest dropped)
	if entries[0].TotalTokens != 200 {
		t.Errorf("expected first entry tokens=200, got %d", entries[0].TotalTokens)
	}
	if entries[2].TotalTokens != 400 {
		t.Errorf("expected last entry tokens=400, got %d", entries[2].TotalTokens)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web/ -run TestHistoryBuffer -v`
Expected: FAIL — `NewHistoryBuffer` not defined.

- [ ] **Step 3: Implement HistoryBuffer**

Create `internal/web/history.go`:

```go
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

// NewHistoryBuffer creates a buffer with the given capacity.
func NewHistoryBuffer(capacity int) *HistoryBuffer {
	return &HistoryBuffer{
		buf: make([]HistoryMsg, capacity),
		cap: capacity,
	}
}

// Add appends a history message, overwriting the oldest if full.
func (hb *HistoryBuffer) Add(msg HistoryMsg) {
	hb.mu.Lock()
	defer hb.mu.Unlock()

	hb.buf[hb.head] = msg
	hb.head = (hb.head + 1) % hb.cap
	if hb.len < hb.cap {
		hb.len++
	}
}

// Entries returns all history messages in chronological order.
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/web/ -run TestHistoryBuffer -v`
Expected: All 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/web/history.go internal/web/history_test.go
git commit -m "feat: implement history buffer for chart data backfill"
```

---

### Task 5: WebSocket hub

**Files:**
- Create: `internal/web/hub.go`
- Create: `internal/web/hub_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/web/hub_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web/ -run TestHub -v`
Expected: FAIL — `NewHub` not defined.

- [ ] **Step 3: Implement Hub**

Create `internal/web/hub.go`:

```go
package web

import "sync"

// Hub manages WebSocket client connections and broadcasts messages.
type Hub struct {
	mu         sync.RWMutex
	clients    map[chan []byte]struct{}
	register   chan chan []byte
	unregister chan chan []byte
	broadcast  chan []byte
	stop       chan struct{}
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan []byte]struct{}),
		register:   make(chan chan []byte),
		unregister: make(chan chan []byte),
		broadcast:  make(chan []byte, 64),
		stop:       make(chan struct{}),
	}
}

// Run starts the hub event loop. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case ch := <-h.register:
			h.mu.Lock()
			h.clients[ch] = struct{}{}
			h.mu.Unlock()

		case ch := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, ch)
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for ch := range h.clients {
				select {
				case ch <- msg:
				default:
					// Client too slow, skip
				}
			}
			h.mu.RUnlock()

		case <-h.stop:
			return
		}
	}
}

// Register adds a client channel.
func (h *Hub) Register(ch chan []byte) {
	h.register <- ch
}

// Unregister removes a client channel.
func (h *Hub) Unregister(ch chan []byte) {
	h.unregister <- ch
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Stop shuts down the hub.
func (h *Hub) Stop() {
	close(h.stop)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/web/ -run TestHub -v`
Expected: Both tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/web/hub.go internal/web/hub_test.go
git commit -m "feat: implement WebSocket hub for client broadcast"
```

---

### Task 6: Data poller

**Files:**
- Create: `internal/web/poller.go`
- Create: `internal/web/poller_test.go`

- [ ] **Step 1: Write test for state message assembly**

Create `internal/web/poller_test.go`:

```go
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
		HasData:        true,
		PercentPerHour: 4.2,
		TokensPerHour:  12000,
		TimeToExhaust:  2*time.Hour + 15*time.Minute,
	}
	sessions := []data.SessionInfo{
		{
			PID:       86792,
			SessionID: "sess-1",
			Name:      "claude-monitor",
			Model:     "Opus 4.6",
			Status:    data.StatusWorking,
			LastBridge: time.Now(),
			Cwd:       "/tmp/test",
		},
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
	opusModel, ok := msg.Models["Opus 4.6"]
	if !ok {
		t.Fatal("expected 'Opus 4.6' in models map")
	}
	if opusModel.TotalTokens != 487000 {
		t.Errorf("expected opus tokens 487000, got %d", opusModel.TotalTokens)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/web/ -run "TestBuild|TestDetect" -v`
Expected: FAIL — `BuildStateMsg` not defined.

- [ ] **Step 3: Implement poller functions**

Create `internal/web/poller.go`:

```go
package web

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
	"github.com/aaltwesthuis/claude-monitor/internal/data"
)

// Poller reads data files and pushes messages to the hub.
type Poller struct {
	hub        *Hub
	history    *HistoryBuffer
	ringBuffer *data.RingBuffer
	prevStatus map[int]string
	stop       chan struct{}
}

// NewPoller creates a new data poller.
func NewPoller(hub *Hub, history *HistoryBuffer) *Poller {
	return &Poller{
		hub:        hub,
		history:    history,
		ringBuffer: data.NewRingBuffer(config.RingBufferSize),
		prevStatus: make(map[int]string),
		stop:       make(chan struct{}),
	}
}

// Run starts the polling loop. Call in a goroutine.
func (p *Poller) Run() {
	stateTicker := time.NewTicker(config.PollBridge)
	historyTicker := time.NewTicker(5 * time.Second)
	defer stateTicker.Stop()
	defer historyTicker.Stop()

	// Initial poll
	p.pollAndBroadcastState()

	for {
		select {
		case <-stateTicker.C:
			p.pollAndBroadcastState()
		case <-historyTicker.C:
			p.pollAndBroadcastHistory()
		case <-p.stop:
			return
		}
	}
}

// Stop shuts down the poller.
func (p *Poller) Stop() {
	close(p.stop)
}

func (p *Poller) pollAndBroadcastState() {
	bridges, _ := data.ReadBridgeFiles(config.BridgeDir)
	states, _ := data.ReadStateFiles(config.BridgeDir)
	registries, _ := data.ReadSessionRegistry(config.SessionsDir)

	usage := data.LatestUsage(bridges)
	sessions := data.MergeSessions(registries, states, bridges)

	// Feed ring buffer for burn rate
	if usage.HasData {
		p.ringBuffer.Add(data.Observation{
			Timestamp:   time.Now(),
			FiveHourPct: usage.FiveHour.UsedPercentage,
			SevenDayPct: usage.SevenDay.UsedPercentage,
			TotalTokens: usage.TotalTokens,
		})
	}
	burnRate := p.ringBuffer.CalcBurnRate(time.Duration(config.BurnRateWindowMin) * time.Minute)

	// Detect state changes and emit events
	events := DetectStateChanges(p.prevStatus, sessions)
	for _, evt := range events {
		evtJSON, err := json.Marshal(evt)
		if err == nil {
			p.hub.Broadcast(evtJSON)
		}
	}

	// Update previous status
	for _, s := range sessions {
		p.prevStatus[s.PID] = statusString(s.Status)
	}

	// Build and broadcast state
	msg := BuildStateMsg(usage, burnRate, sessions, bridges)
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		log.Printf("web: marshal state: %v", err)
		return
	}
	p.hub.Broadcast(msgJSON)
}

func (p *Poller) pollAndBroadcastHistory() {
	bridges, _ := data.ReadBridgeFiles(config.BridgeDir)
	usage := data.LatestUsage(bridges)
	burnRate := p.ringBuffer.CalcBurnRate(time.Duration(config.BurnRateWindowMin) * time.Minute)

	msg := BuildHistoryMsg(usage, burnRate, bridges)
	p.history.Add(msg)

	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return
	}
	p.hub.Broadcast(msgJSON)
}

// HistoryEntries returns the buffered history for new client backfill.
func (p *Poller) HistoryEntries() []HistoryMsg {
	return p.history.Entries()
}

// BuildStateMsg assembles a StateMsg from data layer types.
func BuildStateMsg(usage data.UsageSnapshot, burnRate data.BurnRate, sessions []data.SessionInfo, bridges []data.BridgeData) StateMsg {
	msg := StateMsg{Type: "state"}

	msg.Usage = UsageMsg{
		HasData:     usage.HasData,
		IsStale:     usage.IsStale,
		TotalTokens: usage.TotalTokens,
	}
	if usage.HasData {
		msg.Usage.FiveHour = WindowMsg{
			UsedPct:  usage.FiveHour.UsedPercentage,
			ResetsAt: usage.FiveHour.ResetsAt.Format(time.RFC3339),
			Severity: usage.FiveHour.Severity,
		}
		msg.Usage.SevenDay = WindowMsg{
			UsedPct:  usage.SevenDay.UsedPercentage,
			ResetsAt: usage.SevenDay.ResetsAt.Format(time.RFC3339),
			Severity: usage.SevenDay.Severity,
		}
	}

	var tteMinutes float64
	if burnRate.TimeToExhaust > 0 {
		tteMinutes = burnRate.TimeToExhaust.Minutes()
	}
	msg.BurnRate = BurnRateMsg{
		HasData:       burnRate.HasData,
		PctPerHour:    burnRate.PercentPerHour,
		TokensPerHour: burnRate.TokensPerHour,
		TTEMinutes:    tteMinutes,
	}

	for _, s := range sessions {
		msg.Sessions = append(msg.Sessions, SessionMsg{
			PID:     s.PID,
			HexID:   data.SessionHexID(s.PID),
			Name:    s.Name,
			Model:   s.Model,
			Status:  statusString(s.Status),
			Latency: data.FormatLatency(s.LastBridge),
			Cwd:     s.Cwd,
		})
	}

	// Per-model token breakdown
	modelTokens := make(map[string]int)
	for _, b := range bridges {
		if b.Model != "" {
			modelTokens[b.Model] += b.Tokens.Total()
		}
	}
	totalAllModels := 0
	for _, t := range modelTokens {
		totalAllModels += t
	}
	msg.Models = make(map[string]ModelMsg)
	for model, tokens := range modelTokens {
		pct := 0.0
		if totalAllModels > 0 {
			pct = float64(tokens) / float64(totalAllModels) * 100
		}
		msg.Models[model] = ModelMsg{TotalTokens: tokens, Pct: pct}
	}

	return msg
}

// BuildHistoryMsg assembles a HistoryMsg from current data.
func BuildHistoryMsg(usage data.UsageSnapshot, burnRate data.BurnRate, bridges []data.BridgeData) HistoryMsg {
	msg := HistoryMsg{
		Type:      "history",
		Timestamp: time.Now(),
	}

	if usage.HasData {
		msg.FiveHourPct = usage.FiveHour.UsedPercentage
		msg.SevenDayPct = usage.SevenDay.UsedPercentage
		msg.TotalTokens = usage.TotalTokens
	}
	if burnRate.HasData {
		msg.BurnRatePct = burnRate.PercentPerHour
	}

	msg.TokensByModel = make(map[string]int)
	for _, b := range bridges {
		if b.Model != "" {
			msg.TokensByModel[b.Model] += b.Tokens.Total()
		}
	}

	return msg
}

// DetectStateChanges compares previous and current session states, returns events.
func DetectStateChanges(prev map[int]string, current []data.SessionInfo) []EventMsg {
	var events []EventMsg
	for _, s := range current {
		currStatus := statusString(s.Status)
		prevStatus, existed := prev[s.PID]

		if !existed {
			events = append(events, EventMsg{
				Type:      "event",
				Timestamp: time.Now(),
				PID:       s.PID,
				Session:   s.Name,
				Model:     s.Model,
				Action:    "session_started",
				Detail:    "session started",
			})
		} else if prevStatus != currStatus {
			events = append(events, EventMsg{
				Type:      "event",
				Timestamp: time.Now(),
				PID:       s.PID,
				Session:   s.Name,
				Model:     s.Model,
				Action:    "state_change",
				Detail:    fmt.Sprintf("%s → %s", prevStatus, currStatus),
			})
		}
	}
	return events
}

func statusString(s data.SessionStatus) string {
	return strings.ToLower(s.String())
}
```

Note: `statusString` will return lowercase versions of the status names. The existing `String()` method returns "WORKING", "IDLE", etc. We need lowercase for the web protocol. Update `statusString` if `String()` was changed to title case already — check the actual return values. Currently `String()` returns "WORKING", "WAITING", "BLOCKED", "ZOMBIE_STATE", "IDLE". The `statusString` wrapper lowercases these to "working", "waiting", "blocked", "zombie_state", "idle".

- [ ] **Step 4: Run tests**

Run: `go test ./internal/web/ -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/web/poller.go internal/web/poller_test.go
git commit -m "feat: implement data poller with state/history/event message assembly"
```

---

### Task 7: HTTP server and WebSocket handler

**Files:**
- Create: `internal/web/embed.go`
- Create: `internal/web/server.go`

- [ ] **Step 1: Create embed.go**

Create `internal/web/embed.go`:

```go
package web

import "embed"

//go:embed all:../../web
var EmbeddedFS embed.FS
```

Note: This will fail to compile until `web/` directory exists (Task 9). That's fine — we'll create a placeholder in this step.

- [ ] **Step 2: Create web/ directory with placeholder**

Create `web/.gitkeep` (empty file) so the embed directive has something to reference.

Actually, create a minimal `web/index.html` placeholder:

```html
<!DOCTYPE html>
<html><head><title>claude-monitor</title></head>
<body><p>Loading...</p></body></html>
```

- [ ] **Step 3: Create server.go**

Create `internal/web/server.go`:

```go
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"nhooyr.io/websocket"
)

// Server is the web dashboard HTTP server.
type Server struct {
	hub     *Hub
	poller  *Poller
	history *HistoryBuffer
	mux     *http.ServeMux
	addr    string
	dev     bool
}

// NewServer creates a new web server.
func NewServer(addr string, dev bool) *Server {
	hub := NewHub()
	history := NewHistoryBuffer(720) // 1 hour at 5s intervals
	poller := NewPoller(hub, history)

	s := &Server{
		hub:     hub,
		poller:  poller,
		history: history,
		mux:     http.NewServeMux(),
		addr:    addr,
		dev:     dev,
	}

	s.mux.HandleFunc("/ws", s.handleWS)
	s.mux.Handle("/", s.staticHandler())

	return s
}

// Run starts the hub, poller, and HTTP server.
func (s *Server) Run() error {
	go s.hub.Run()
	go s.poller.Run()

	log.Printf("claude-monitor web dashboard: http://%s", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) staticHandler() http.Handler {
	if s.dev {
		log.Println("dev mode: serving web/ from disk")
		return http.FileServer(http.Dir("web"))
	}

	sub, err := fs.Sub(EmbeddedFS, "web")
	if err != nil {
		log.Fatalf("embedded fs: %v", err)
	}
	return http.FileServer(http.FS(sub))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}
	defer conn.CloseNow()

	// Send history backfill
	entries := s.poller.HistoryEntries()
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if err := conn.Write(r.Context(), websocket.MessageText, data); err != nil {
			return
		}
	}

	// Register client channel
	ch := make(chan []byte, 64)
	s.hub.Register(ch)
	defer s.hub.Unregister(ch)

	// Read loop (handles ping/pong and detects disconnect)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				cancel()
				return
			}
			// Handle ping
			if string(msg) == `{"type":"ping"}` {
				conn.Write(ctx, websocket.MessageText, []byte(`{"type":"pong"}`))
			}
		}
	}()

	// Write loop
	for {
		select {
		case msg := <-ch:
			err := conn.Write(ctx, websocket.MessageText, msg)
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
```

- [ ] **Step 4: Fix embed path**

The `embed.go` path `all:../../web` is relative to the file's package directory (`internal/web/`). This should point to `../../web` which resolves to the project root `web/` directory. Verify this compiles:

Run: `go build ./internal/web/...`

If the embed path fails, the correct path may need adjusting. The `go:embed` directive path is relative to the source file. Since `embed.go` is at `internal/web/embed.go`, the path `../../web` points to `<project-root>/web/`.

Note: `go:embed` cannot use `..` paths. We need to move the embed directive to the project root level instead.

Delete `internal/web/embed.go` and update approach: put the embed in `cmd/monitor/` and pass the `fs.FS` to the server.

Update `internal/web/embed.go` — actually remove it entirely. Instead:

Create `cmd/monitor/embed.go`:

```go
package main

import "embed"

//go:embed all:web
var webFS embed.FS
```

Wait — this also won't work because `cmd/monitor/` is at `cmd/monitor/` and `web/` is at the project root. The embed path `all:web` would look for `cmd/monitor/web/`.

The correct approach: put `embed.go` at the project root in a dedicated package, or restructure. Simplest: move the embed to `web/embed.go`:

Create `web/embed.go`:

```go
package web

import "embed"

//go:embed all:*
var FS embed.FS
```

This embeds everything in the `web/` directory. Then `internal/web/server.go` imports `github.com/aaltwesthuis/claude-monitor/web` and uses `web.FS`.

But this creates a circular-ish naming issue (`internal/web` vs `web`). Rename the static package to `webstatic`:

Create `web/webstatic/embed.go`:

No — keep it simple. The `web/` directory is for static files. Put the embed in its own tiny package:

Create `web/embed.go`:

```go
package webfs

import "embed"

//go:embed all:*
var FS embed.FS
```

Package name `webfs` so `internal/web/server.go` does `import webfs "github.com/aaltwesthuis/claude-monitor/web"` and uses `webfs.FS`.

Actually this won't work either — the `web/` directory has `.html`, `.js`, `.css` files AND a `.go` file, so Go will try to build the package from everything in that directory.

Cleanest solution: put static files in `web/static/` and the embed in `web/embed.go`:

```
web/
  embed.go          # package webfs, embeds static/*
  static/
    index.html
    app.js
    style.css
```

Create `web/embed.go`:

```go
package webfs

import "embed"

//go:embed all:static
var FS embed.FS
```

Then in `server.go`, use `fs.Sub(webfs.FS, "static")`.

Update the Server constructor to accept an `fs.FS` parameter instead of using a package-level var:

```go
func NewServer(addr string, dev bool, staticFS fs.FS) *Server {
```

- [ ] **Step 5: Revise server.go to accept fs.FS**

Replace the `internal/web/embed.go` approach. Delete that file if created.

Update `internal/web/server.go` constructor:

```go
// NewServer creates a new web server.
func NewServer(addr string, dev bool, staticFS fs.FS) *Server {
	hub := NewHub()
	history := NewHistoryBuffer(720)
	poller := NewPoller(hub, history)

	s := &Server{
		hub:      hub,
		poller:   poller,
		history:  history,
		mux:      http.NewServeMux(),
		addr:     addr,
		dev:      dev,
		staticFS: staticFS,
	}

	s.mux.HandleFunc("/ws", s.handleWS)
	s.mux.Handle("/", s.staticHandler())

	return s
}
```

Add `staticFS fs.FS` to the Server struct. Update `staticHandler()`:

```go
func (s *Server) staticHandler() http.Handler {
	if s.dev {
		log.Println("dev mode: serving web/static/ from disk")
		return http.FileServer(http.Dir("web/static"))
	}
	return http.FileServer(http.FS(s.staticFS))
}
```

- [ ] **Step 6: Create web/embed.go and web/static/ placeholder**

Create `web/embed.go`:

```go
package webfs

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embedFS embed.FS

// FS returns the static files filesystem.
func FS() (fs.FS, error) {
	return fs.Sub(embedFS, "static")
}
```

Create `web/static/index.html`:

```html
<!DOCTYPE html>
<html><head><title>claude-monitor</title></head>
<body><p>Loading...</p></body></html>
```

- [ ] **Step 7: Verify compilation**

Run: `go build ./...`
Expected: Success.

- [ ] **Step 8: Commit**

```bash
git add internal/web/server.go web/embed.go web/static/index.html
git commit -m "feat: implement HTTP server with WebSocket handler and embedded static files"
```

---

### Task 8: Wire up main.go with web subcommand

**Files:**
- Modify: `cmd/monitor/main.go`

- [ ] **Step 1: Add web subcommand**

Replace `cmd/monitor/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
	"github.com/aaltwesthuis/claude-monitor/internal/web"
	webfs "github.com/aaltwesthuis/claude-monitor/web"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 && os.Args[0] != "" && os.Args[1] == "web" {
		runWeb(os.Args[2:])
		return
	}
	runTUI()
}

func runTUI() {
	p := tea.NewProgram(
		tui.NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "claude-monitor v%s: %v\n", config.Version, err)
		os.Exit(1)
	}
}

func runWeb(args []string) {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	port := fs.Int("p", 3000, "HTTP server port")
	dev := fs.Bool("dev", false, "serve static files from disk (hot reload)")
	fs.Parse(args)

	staticFS, err := webfs.FS()
	if err != nil {
		log.Fatalf("embedded static files: %v", err)
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := web.NewServer(addr, *dev, staticFS)
	if err := srv.Run(); err != nil {
		log.Fatalf("web server: %v", err)
	}
}
```

- [ ] **Step 2: Verify it compiles and TUI still works**

Run: `go build -o claude-monitor ./cmd/monitor/`
Expected: Success.

Run: `./claude-monitor web -p 3001 &` then `curl -s http://localhost:3001/ | head -1`
Expected: `<!DOCTYPE html>`

Kill the background process, then verify TUI:
Run: `./claude-monitor` (press `q` to quit)
Expected: TUI renders normally.

- [ ] **Step 3: Commit**

```bash
git add cmd/monitor/main.go
git commit -m "feat: add web subcommand to main entry point"
```

---

### Task 9: Frontend — HTML shell and CSS theme

**Files:**
- Create: `web/static/index.html`
- Create: `web/static/style.css`

- [ ] **Step 1: Create style.css**

Create `web/static/style.css`:

```css
:root {
  --bg: #1e1e2e;
  --surface: #181825;
  --surface-container: #11111b;
  --surface-high: #313244;
  --primary: #cba6f7;
  --on-primary: #1e1e2e;
  --secondary: #fab387;
  --tertiary: #89b4fa;
  --error: #f38ba8;
  --green: #a6e3a1;
  --yellow: #f9e2af;
  --on-surface: #cdd6f4;
  --on-surface-variant: #bac2de;
  --outline: #45475a;
  --text-dim: #a6adc8;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  background: var(--bg);
  color: var(--on-surface);
  font-family: 'JetBrains Mono', monospace;
  min-height: 100vh;
  overflow-x: hidden;
}

.dashboard {
  display: grid;
  grid-template-columns: repeat(12, 1fr);
  gap: 12px;
  padding: 16px;
  max-width: 1600px;
  margin: 0 auto;
}

.panel {
  background: var(--surface);
  border: 1px solid var(--outline);
  padding: 20px;
}

/* Header bar */
.header-bar {
  grid-column: span 12;
  background: var(--surface);
  border: 1px solid var(--outline);
  padding: 12px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-bar h1 {
  font-family: 'Space Grotesk', sans-serif;
  color: var(--primary);
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.05em;
}
.header-bar .meta {
  color: var(--text-dim);
  font-size: 10px;
}

/* Usage panel */
.usage-panel { grid-column: span 8; }
.burn-panel { grid-column: span 4; background: var(--surface-high); border-color: rgba(203,166,247,0.2); }

.section-label {
  color: var(--primary);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}
.section-title {
  font-family: 'Space Grotesk', sans-serif;
  color: var(--on-surface);
  font-weight: 900;
  font-size: 18px;
}

/* Progress bars */
.progress-bar-container {
  height: 22px;
  background: var(--surface-container);
  border: 1px solid var(--outline);
  position: relative;
  overflow: hidden;
}
.progress-bar-fill {
  height: 100%;
  transition: width 0.5s ease;
}
.progress-bar-label {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: 700;
  color: white;
  mix-blend-mode: difference;
}

/* Burn rate metrics */
.metric-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  border-bottom: 1px solid var(--outline);
  padding-bottom: 10px;
  margin-bottom: 10px;
}
.metric-label { color: var(--on-surface-variant); font-size: 10px; }
.metric-value { font-weight: 700; font-size: 16px; }

.tte-box {
  background: rgba(250,179,135,0.05);
  border: 1px solid rgba(250,179,135,0.2);
  padding: 10px;
  margin-top: 12px;
}
.tte-box .label { color: var(--secondary); font-size: 10px; }
.tte-box .value { color: var(--secondary); font-weight: 700; font-size: 14px; }
.tte-box.safe { border-color: rgba(166,227,161,0.2); background: rgba(166,227,161,0.05); }
.tte-box.safe .label, .tte-box.safe .value { color: var(--green); }

/* Model cards */
.model-cards {
  display: flex;
  gap: 16px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--outline);
}
.model-card {
  flex: 1;
  background: var(--surface-container);
  border: 1px solid var(--outline);
  padding: 12px;
}
.model-card .dot {
  width: 8px;
  height: 8px;
  border-radius: 2px;
  display: inline-block;
}
.model-card .tokens {
  font-size: 20px;
  font-weight: 700;
}
.model-bar {
  height: 4px;
  background: var(--surface-high);
  border-radius: 2px;
  overflow: hidden;
  margin-top: 8px;
}
.model-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.5s ease;
}

/* Charts */
.chart-panel { grid-column: span 6; }
.chart-container {
  background: var(--surface-container);
  border: 1px solid var(--outline);
  padding: 4px;
  height: 150px;
  position: relative;
}
.chart-legend {
  font-size: 9px;
  color: var(--on-surface-variant);
  margin-top: 4px;
  display: flex;
  gap: 12px;
}

/* Session table */
.session-panel {
  grid-column: span 12;
  padding: 0;
  overflow: hidden;
}
.session-header {
  background: var(--surface-container);
  padding: 12px 20px;
  border-bottom: 1px solid var(--outline);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.session-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.session-table th {
  text-align: left;
  padding: 10px 20px;
  border-bottom: 1px solid var(--outline);
  color: var(--on-surface-variant);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
}
.session-table th:last-child { text-align: right; }
.session-table td {
  padding: 10px 20px;
  border-bottom: 1px solid rgba(69,71,90,0.3);
}
.session-table td:last-child { text-align: right; }
.session-table tr:hover { background: rgba(203,166,247,0.03); }

.session-id {
  color: var(--primary);
  cursor: pointer;
  text-decoration: underline dotted;
}
.model-badge {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
}
.model-badge.opus { color: var(--primary); background: rgba(203,166,247,0.1); }
.model-badge.sonnet { color: var(--tertiary); background: rgba(137,180,250,0.1); }
.model-badge.haiku { color: var(--green); background: rgba(166,227,161,0.1); }

.status-chip {
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 9999px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.status-chip.working { color: var(--tertiary); background: rgba(137,180,250,0.1); border: 1px solid rgba(137,180,250,0.2); }
.status-chip.waiting { color: var(--secondary); background: rgba(250,179,135,0.1); border: 1px solid rgba(250,179,135,0.2); }
.status-chip.blocked { color: var(--error); background: rgba(243,139,168,0.1); border: 1px solid rgba(243,139,168,0.2); }
.status-chip.idle { color: var(--on-surface-variant); }
.status-chip.zombie { color: var(--text-dim); opacity: 0.5; }

/* Event log */
.event-panel {
  grid-column: span 12;
  max-height: 180px;
  overflow-y: auto;
}
.event-entry {
  font-size: 11px;
  line-height: 2;
  color: var(--on-surface-variant);
}
.event-entry .time { color: var(--outline); }
.event-entry .session-ref { color: var(--primary); }
.event-entry .model-ref { color: var(--outline); }

/* Footer */
.footer {
  grid-column: span 12;
  background: var(--surface-container);
  border: 1px solid var(--outline);
  padding: 8px 20px;
  display: flex;
  justify-content: space-between;
  font-size: 10px;
  color: var(--text-dim);
}

/* Disconnected overlay */
.disconnected {
  position: fixed;
  top: 12px;
  right: 12px;
  background: rgba(243,139,168,0.15);
  border: 1px solid rgba(243,139,168,0.3);
  color: var(--error);
  padding: 8px 16px;
  font-size: 11px;
  font-weight: 700;
  border-radius: 4px;
  display: none;
  z-index: 100;
}
.disconnected.show { display: block; }
```

- [ ] **Step 2: Create index.html**

Replace `web/static/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>claude-monitor</title>
  <link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;700;900&family=JetBrains+Mono:wght@400;700&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <div id="disconnected" class="disconnected">Disconnected — reconnecting...</div>

  <div class="dashboard">
    <!-- Header -->
    <div class="header-bar">
      <h1>claude-monitor</h1>
      <div class="meta">
        <span id="header-version"></span> //
        <span id="header-time"></span>
      </div>
    </div>

    <!-- Usage Panel -->
    <div class="panel usage-panel">
      <div style="display:flex;justify-content:space-between;margin-bottom:16px;">
        <div>
          <div class="section-label">Compute Metrics</div>
          <div class="section-title">Usage Statistics</div>
        </div>
        <div style="text-align:right;">
          <span class="metric-label" id="stale-indicator"></span>
        </div>
      </div>

      <!-- 5h bar -->
      <div style="margin-bottom:20px;">
        <div style="display:flex;justify-content:space-between;font-size:11px;margin-bottom:6px;">
          <span style="font-weight:600;">5h Window Usage</span>
          <span class="metric-label" id="five-hour-resets"></span>
        </div>
        <div class="progress-bar-container">
          <div class="progress-bar-fill" id="five-hour-bar"></div>
          <div class="progress-bar-label" id="five-hour-label"></div>
        </div>
      </div>

      <!-- 7d bar -->
      <div style="margin-bottom:20px;">
        <div style="display:flex;justify-content:space-between;font-size:11px;margin-bottom:6px;">
          <span style="font-weight:600;">7d Window Usage</span>
          <span class="metric-label" id="seven-day-resets"></span>
        </div>
        <div class="progress-bar-container">
          <div class="progress-bar-fill" id="seven-day-bar"></div>
          <div class="progress-bar-label" id="seven-day-label"></div>
        </div>
      </div>

      <!-- Model breakdown -->
      <div class="model-cards" id="model-cards"></div>
    </div>

    <!-- Burn Rate Panel -->
    <div class="panel burn-panel">
      <div class="section-title" style="margin-bottom:20px;">Burn Rate Analysis</div>
      <div class="metric-row">
        <span class="metric-label">Rate/Hour</span>
        <span class="metric-value" id="burn-rate" style="color:var(--primary);">--</span>
      </div>
      <div class="metric-row">
        <span class="metric-label">Token Velocity</span>
        <span class="metric-value" id="token-velocity" style="color:var(--green);">--</span>
      </div>
      <div class="tte-box" id="tte-box">
        <div class="label">Time to Exhaustion</div>
        <div class="value" id="tte-value">--</div>
      </div>
    </div>

    <!-- Charts -->
    <div class="panel chart-panel">
      <div class="section-label" style="margin-bottom:10px;">Burn Rate Over Time</div>
      <div class="chart-container">
        <canvas id="burn-chart"></canvas>
      </div>
      <div class="chart-legend">Last 60 minutes</div>
    </div>

    <div class="panel chart-panel">
      <div class="section-label" style="margin-bottom:10px;">Token Usage Timeline</div>
      <div class="chart-container">
        <canvas id="token-chart"></canvas>
      </div>
      <div class="chart-legend">
        <span><span style="color:var(--tertiary);">&#9632;</span> Input</span>
        <span><span style="color:var(--primary);">&#9632;</span> Output</span>
      </div>
    </div>

    <!-- Session Table -->
    <div class="panel session-panel">
      <div class="session-header">
        <span class="section-label" style="font-size:12px;">Active Process Matrix</span>
        <div style="font-size:10px;color:var(--on-surface-variant);display:flex;gap:16px;">
          <span id="session-total"></span>
          <span id="session-load"></span>
        </div>
      </div>
      <table class="session-table">
        <thead>
          <tr>
            <th>Session</th>
            <th>Task</th>
            <th>Model</th>
            <th>Latency</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody id="session-body"></tbody>
      </table>
    </div>

    <!-- Event Log -->
    <div class="panel event-panel">
      <div class="section-label" style="margin-bottom:10px;">Session Event Log</div>
      <div id="event-log"></div>
    </div>

    <!-- Footer -->
    <div class="footer">
      <span id="footer-left"></span>
      <span id="footer-right"></span>
    </div>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
  <script src="app.js"></script>
</body>
</html>
```

- [ ] **Step 3: Verify it compiles with embedded files**

Run: `go build -o claude-monitor ./cmd/monitor/`
Expected: Success.

- [ ] **Step 4: Commit**

```bash
git add web/static/index.html web/static/style.css
git commit -m "feat: add dashboard HTML shell and CSS theme"
```

---

### Task 10: Frontend — JavaScript WebSocket client and DOM updates

**Files:**
- Create: `web/static/app.js`

- [ ] **Step 1: Create app.js**

Create `web/static/app.js`:

```javascript
(function () {
  'use strict';

  // --- State ---
  let burnChart = null;
  let tokenChart = null;
  const MAX_CHART_POINTS = 720;
  const MAX_EVENTS = 100;

  // --- WebSocket ---
  let ws = null;
  let reconnectTimer = null;

  function connect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${proto}//${location.host}/ws`);

    ws.onopen = () => {
      document.getElementById('disconnected').classList.remove('show');
      if (reconnectTimer) { clearInterval(reconnectTimer); reconnectTimer = null; }
    };

    ws.onclose = () => {
      document.getElementById('disconnected').classList.add('show');
      if (!reconnectTimer) {
        reconnectTimer = setInterval(connect, 3000);
      }
    };

    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      switch (msg.type) {
        case 'state': handleState(msg); break;
        case 'history': handleHistory(msg); break;
        case 'event': handleEvent(msg); break;
      }
    };
  }

  // --- Gradient color ---
  function gradientColor(pct) {
    if (pct >= 90) return '#f38ba8';
    if (pct >= 70) return '#fab387';
    if (pct >= 50) return '#f9e2af';
    return '#a6e3a1';
  }

  function gradientCSS(pct) {
    if (pct >= 70) return 'linear-gradient(90deg, #f9e2af, #fab387, #f38ba8)';
    if (pct >= 50) return 'linear-gradient(90deg, #a6e3a1, #f9e2af, #fab387)';
    return 'linear-gradient(90deg, #a6e3a1, #f9e2af)';
  }

  // --- State handler ---
  function handleState(msg) {
    updateUsage(msg.usage);
    updateBurnRate(msg.burn_rate);
    updateModels(msg.models);
    updateSessions(msg.sessions);
    updateFooter();
  }

  function updateUsage(usage) {
    if (!usage.has_data) return;

    const stale = document.getElementById('stale-indicator');
    stale.textContent = usage.is_stale ? '[STALE]' : '';
    stale.style.color = usage.is_stale ? '#fab387' : '';

    // 5h bar
    const fivePct = usage.five_hour.used_pct;
    const fiveBar = document.getElementById('five-hour-bar');
    fiveBar.style.width = fivePct + '%';
    fiveBar.style.background = gradientCSS(fivePct);
    document.getElementById('five-hour-label').textContent =
      `${Math.round(fivePct)}% ${usage.five_hour.severity}`;
    document.getElementById('five-hour-resets').textContent =
      'Resets at: ' + new Date(usage.five_hour.resets_at).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});

    // 7d bar
    const sevenPct = usage.seven_day.used_pct;
    const sevenBar = document.getElementById('seven-day-bar');
    sevenBar.style.width = sevenPct + '%';
    sevenBar.style.background = gradientCSS(sevenPct);
    document.getElementById('seven-day-label').textContent =
      `${Math.round(sevenPct)}% ${usage.seven_day.severity}`;
    document.getElementById('seven-day-resets').textContent =
      'Resets at: ' + new Date(usage.seven_day.resets_at).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});
  }

  function updateBurnRate(br) {
    const rateEl = document.getElementById('burn-rate');
    const velEl = document.getElementById('token-velocity');
    const tteBox = document.getElementById('tte-box');
    const tteVal = document.getElementById('tte-value');

    if (!br.has_data) {
      rateEl.textContent = '--';
      velEl.textContent = '--';
      tteVal.textContent = '--';
      return;
    }

    rateEl.textContent = br.pct_per_hour.toFixed(1) + '%';
    rateEl.style.color = gradientColor(br.pct_per_hour * 5); // scale for color

    const tph = br.tokens_per_hour;
    velEl.textContent = (tph >= 1e6 ? (tph/1e6).toFixed(1)+'M' : tph >= 1e3 ? Math.round(tph/1e3)+'k' : Math.round(tph)) + ' t/h';

    if (br.tte_minutes <= 0) {
      tteVal.textContent = 'Safe';
      tteBox.className = 'tte-box safe';
    } else {
      const h = Math.floor(br.tte_minutes / 60);
      const m = Math.round(br.tte_minutes % 60);
      tteVal.textContent = h > 0 ? `Limit in ~${h}h ${m}m` : `Limit in ~${m}m`;
      tteBox.className = br.tte_minutes < 30 ? 'tte-box' : 'tte-box';
    }
  }

  function updateModels(models) {
    const container = document.getElementById('model-cards');
    if (!models || Object.keys(models).length === 0) {
      container.innerHTML = '';
      return;
    }

    const modelColors = { 'Opus': '#cba6f7', 'Sonnet': '#89b4fa', 'Haiku': '#a6e3a1' };
    function colorFor(name) {
      for (const [key, col] of Object.entries(modelColors)) {
        if (name.includes(key)) return col;
      }
      return '#cdd6f4';
    }

    container.innerHTML = Object.entries(models).map(([name, info]) => {
      const col = colorFor(name);
      const tokens = info.total_tokens >= 1e6
        ? (info.total_tokens/1e6).toFixed(1)+'M'
        : info.total_tokens >= 1e3
          ? Math.round(info.total_tokens/1e3)+'k'
          : info.total_tokens;
      return `
        <div class="model-card">
          <div style="display:flex;align-items:center;gap:6px;margin-bottom:8px;">
            <span class="dot" style="background:${col};"></span>
            <span style="font-size:11px;font-weight:600;">${name}</span>
          </div>
          <div class="tokens" style="color:${col};">${tokens}</div>
          <div style="color:var(--on-surface-variant);font-size:10px;">tokens (${info.pct.toFixed(0)}%)</div>
          <div class="model-bar">
            <div class="model-bar-fill" style="width:${info.pct}%;background:${col};"></div>
          </div>
        </div>`;
    }).join('');
  }

  function updateSessions(sessions) {
    const body = document.getElementById('session-body');
    const totalEl = document.getElementById('session-total');
    const loadEl = document.getElementById('session-load');

    totalEl.textContent = `Total: ${String(sessions.length).padStart(2,'0')}`;

    const hasBlocked = sessions.some(s => s.status === 'blocked');
    const hasWaiting = sessions.some(s => s.status === 'waiting');
    if (hasBlocked) {
      loadEl.innerHTML = '<span style="color:var(--error);">Load: Critical</span>';
    } else if (hasWaiting) {
      loadEl.innerHTML = '<span style="color:var(--secondary);">Load: Attention</span>';
    } else {
      loadEl.innerHTML = '<span style="color:var(--green);">Load: Nominal</span>';
    }

    body.innerHTML = sessions.map(s => {
      const modelClass = s.model.includes('Opus') ? 'opus'
        : s.model.includes('Sonnet') ? 'sonnet'
        : s.model.includes('Haiku') ? 'haiku' : '';
      const shortModel = s.model.split(' ')[0] || s.model;
      const statusIcons = {
        working: '&#x2022;', waiting: '&#x25CF;', blocked: '&#x26A0;',
        idle: '&#x25CB;', zombie_state: ''
      };
      const icon = statusIcons[s.status] || '';
      const label = s.status.replace('_', ' ').replace(/\b\w/g, c => c.toUpperCase());

      return `<tr>
        <td><span class="session-id" title="${s.cwd}">${s.hex_id}</span></td>
        <td>${s.name}</td>
        <td><span class="model-badge ${modelClass}">${shortModel}</span></td>
        <td style="color:var(--on-surface-variant);">${s.latency}</td>
        <td><span class="status-chip ${s.status}">${icon} ${label}</span></td>
      </tr>`;
    }).join('');
  }

  function updateFooter() {
    const now = new Date();
    document.getElementById('footer-left').textContent =
      `claude-monitor v1.0.0 // ${now.toISOString().slice(0,19)}Z`;
    document.getElementById('footer-right').textContent =
      `Connected`;
    document.getElementById('header-time').textContent =
      now.toLocaleTimeString();
    document.getElementById('header-version').textContent = 'v1.0.0';
  }

  // --- History handler (charts) ---
  function handleHistory(msg) {
    const time = new Date(msg.timestamp);
    const label = time.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'});

    // Burn rate chart
    if (burnChart) {
      burnChart.data.labels.push(label);
      burnChart.data.datasets[0].data.push(msg.burn_rate_pct_per_hour);
      if (burnChart.data.labels.length > MAX_CHART_POINTS) {
        burnChart.data.labels.shift();
        burnChart.data.datasets[0].data.shift();
      }
      burnChart.update('none');
    }

    // Token chart
    if (tokenChart) {
      tokenChart.data.labels.push(label);
      tokenChart.data.datasets[0].data.push(msg.total_tokens);
      if (tokenChart.data.labels.length > MAX_CHART_POINTS) {
        tokenChart.data.labels.shift();
        tokenChart.data.datasets[0].data.shift();
      }
      tokenChart.update('none');
    }
  }

  // --- Event handler ---
  function handleEvent(msg) {
    const log = document.getElementById('event-log');
    const time = new Date(msg.timestamp).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'});
    const modelTag = msg.model ? ` <span class="model-ref">(${msg.model.split(' ')[0]})</span>` : '';
    const entry = document.createElement('div');
    entry.className = 'event-entry';
    entry.innerHTML = `<span class="time">${time}</span> &nbsp; <span class="session-ref">0x${(msg.pid & 0xFFFF).toString(16).toUpperCase().padStart(4,'0')}</span> ${msg.session}${modelTag} &nbsp; ${msg.detail}`;
    log.insertBefore(entry, log.firstChild);

    // Prune
    while (log.children.length > MAX_EVENTS) {
      log.removeChild(log.lastChild);
    }
  }

  // --- Charts init ---
  function initCharts() {
    const chartDefaults = {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: {
          display: false,
          ticks: { color: '#45475a', maxTicksLimit: 6 },
          grid: { color: 'rgba(69,71,90,0.2)' },
        },
        y: {
          ticks: { color: '#45475a', font: { size: 9 } },
          grid: { color: 'rgba(69,71,90,0.2)' },
        },
      },
      plugins: { legend: { display: false } },
    };

    burnChart = new Chart(document.getElementById('burn-chart'), {
      type: 'line',
      data: {
        labels: [],
        datasets: [{
          data: [],
          borderColor: '#cba6f7',
          backgroundColor: 'rgba(203,166,247,0.1)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 1.5,
        }],
      },
      options: {
        ...chartDefaults,
        scales: {
          ...chartDefaults.scales,
          y: { ...chartDefaults.scales.y, beginAtZero: true },
        },
      },
    });

    tokenChart = new Chart(document.getElementById('token-chart'), {
      type: 'line',
      data: {
        labels: [],
        datasets: [{
          data: [],
          borderColor: '#89b4fa',
          backgroundColor: 'rgba(137,180,250,0.1)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 1.5,
        }],
      },
      options: chartDefaults,
    });
  }

  // --- Init ---
  document.addEventListener('DOMContentLoaded', () => {
    initCharts();
    connect();
  });
})();
```

- [ ] **Step 2: Verify build**

Run: `go build -o claude-monitor ./cmd/monitor/`
Expected: Success.

- [ ] **Step 3: Start server and test in browser**

Run: `./claude-monitor web --dev -p 3001`

Open `http://localhost:3001` in browser. Verify:
- Dashboard layout renders with all panels
- WebSocket connects (no "Disconnected" banner)
- Usage bars update with real data
- Burn rate panel shows values
- Session table populates
- Charts start collecting data points

- [ ] **Step 4: Commit**

```bash
git add web/static/app.js
git commit -m "feat: implement WebSocket client with live DOM updates and Chart.js charts"
```

---

### Task 11: End-to-end verification

**Files:** None (testing only)

- [ ] **Step 1: Run all Go tests**

Run: `go test ./... -v`
Expected: All tests pass (existing + new hub, history, poller tests).

- [ ] **Step 2: Build clean binary**

Run: `go build -o claude-monitor ./cmd/monitor/`
Expected: Success.

- [ ] **Step 3: Test TUI mode**

Run: `./claude-monitor` — press `q` to quit.
Expected: TUI renders identically to before.

- [ ] **Step 4: Test web mode with embedded files**

Run: `./claude-monitor web -p 3001`
Open `http://localhost:3001`.
Expected: Dashboard loads from embedded files (not `--dev` mode). All panels work.

- [ ] **Step 5: Test web mode dev flag**

Run: `./claude-monitor web --dev -p 3002`
Edit `web/static/style.css` (change `--bg` to `#ff0000`), refresh browser.
Expected: Background turns red without rebuilding binary.

- [ ] **Step 6: Test WebSocket reconnection**

With web server running, kill and restart it. Browser should show "Disconnected" then auto-reconnect.

- [ ] **Step 7: Commit all remaining changes**

```bash
git add -A
git commit -m "feat: complete web dashboard with charts, per-model breakdown, and event log"
```
