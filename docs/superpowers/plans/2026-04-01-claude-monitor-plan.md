# claude-monitor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a persistent Go TUI dashboard for ambient Claude Code usage and session awareness.

**Architecture:** A bubbletea TUI that polls bridge files (written by the statusline binary and hooks) for usage/session data, maintains a ring buffer for burn rate calculation, and renders everything with lipgloss using 24-bit true color following the "Editorial Terminalism" design system.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, go-colorful. Existing infra: statusline-src/main.go, claude-tmux-status.sh.

**Spec:** `docs/superpowers/specs/2026-04-01-claude-monitor-design.md`

---

### Task 1: Go Module & Project Skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/monitor/main.go`
- Create: `internal/config/config.go`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /Users/aaltwesthuis/Sources/playground/claude-monitor
go mod init github.com/aaltwesthuis/claude-monitor
```

Expected: `go.mod` created with module path.

- [ ] **Step 2: Create config package with constants**

Create `internal/config/config.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"time"
)

const (
	Version    = "1.0.0"
	VersionTag = "V1.0-STABLE"

	MinTermWidth  = 120
	MinTermHeight = 30

	PollBridge   = 2 * time.Second
	PollState    = 1 * time.Second
	PollRegistry = 5 * time.Second
	PollPID      = 10 * time.Second
	TickAnimation = 200 * time.Millisecond

	RingBufferSize    = 360
	BurnRateWindowMin = 15

	StaleThreshold     = 5 * time.Minute
	ZombieGracePeriod  = 30 * time.Second
	ZombieCleanupDelay = 60 * time.Second
)

var (
	BridgeDir   = os.TempDir()
	SessionsDir = filepath.Join(homeDir(), ".claude", "sessions")
)

func BridgePattern() string {
	return filepath.Join(BridgeDir, "claude-monitor-*.json")
}

func StatePattern() string {
	return filepath.Join(BridgeDir, "claude-session-state-*.json")
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return h
}
```

- [ ] **Step 3: Create minimal main.go entry point**

Create `cmd/monitor/main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/aaltwesthuis/claude-monitor/internal/config"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
)

func main() {
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
```

- [ ] **Step 4: Create stub TUI model (enough to compile)**

Create `internal/tui/model.go`:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	width  int
	height int
	ready  bool
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return "claude-monitor stub"
}
```

- [ ] **Step 5: Install dependencies and verify compilation**

Run:
```bash
cd /Users/aaltwesthuis/Sources/playground/claude-monitor
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/lucasb-eyer/go-colorful
go build ./cmd/monitor/
```

Expected: Binary compiles with no errors.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: initialize Go module with project skeleton and stub TUI"
```

---

### Task 2: Data Types

**Files:**
- Create: `internal/data/types.go`

- [ ] **Step 1: Create shared data types**

Create `internal/data/types.go`:

```go
package data

import "time"

// BridgeData represents usage data from /tmp/claude-monitor-{sessionId}.json
type BridgeData struct {
	SessionID  string     `json:"session_id"`
	Timestamp  int64      `json:"timestamp"`
	RateLimits RateLimits `json:"rate_limits"`
	Tokens     Tokens     `json:"tokens"`
	Model      string     `json:"model"`
	Cwd        string     `json:"cwd"`
}

type RateLimits struct {
	FiveHour WindowLimit `json:"five_hour"`
	SevenDay WindowLimit `json:"seven_day"`
}

type WindowLimit struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       float64 `json:"resets_at"`
}

// HasRateLimits returns true if rate limit data is present (non-zero).
func (b *BridgeData) HasRateLimits() bool {
	return b.RateLimits.FiveHour.ResetsAt > 0 || b.RateLimits.SevenDay.ResetsAt > 0
}

type Tokens struct {
	Input         int `json:"input"`
	Output        int `json:"output"`
	CacheRead     int `json:"cache_read"`
	CacheCreation int `json:"cache_creation"`
	TotalInput    int `json:"total_input"`
	TotalOutput   int `json:"total_output"`
}

func (t *Tokens) Total() int {
	return t.TotalInput + t.TotalOutput
}

// SessionState represents state from /tmp/claude-session-state-{pid}.json
type SessionState struct {
	PID       int    `json:"pid"`
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	UpdatedAt int64  `json:"updated_at"`
	Event     string `json:"event"`
}

// SessionRegistry represents data from ~/.claude/sessions/{pid}.json
type SessionRegistry struct {
	PID       int    `json:"pid"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Name      string `json:"name"`
	StartedAt int64  `json:"startedAt"`
	Kind      string `json:"kind"`
}

// SessionInfo is the merged view of a session for the TUI.
type SessionInfo struct {
	PID         int
	SessionID   string
	Name        string // session name or last path component of cwd
	Cwd         string
	Status      SessionStatus
	LastBridge  time.Time // time of last bridge file update
	StartedAt   time.Time
	Alive       bool
	ZombieSince time.Time // when the PID was first detected as dead
}

type SessionStatus int

const (
	StatusIdle SessionStatus = iota
	StatusWorking
	StatusWaiting
	StatusBlocked
	StatusZombie
)

func (s SessionStatus) String() string {
	switch s {
	case StatusWorking:
		return "WORKING"
	case StatusWaiting:
		return "WAITING"
	case StatusBlocked:
		return "BLOCKED"
	case StatusZombie:
		return "ZOMBIE_STATE"
	default:
		return "IDLE"
	}
}

// UsageSnapshot is what the TUI uses for display.
type UsageSnapshot struct {
	FiveHour    WindowUsage
	SevenDay    WindowUsage
	TotalTokens int
	HasData     bool
	IsStale     bool
	LastUpdate  time.Time
}

type WindowUsage struct {
	UsedPercentage float64
	ResetsAt       time.Time
	Severity       string // NOMINAL, ELEVATED, HIGH, CRITICAL
}

func SeverityFor(pct float64) string {
	switch {
	case pct >= 90:
		return "CRITICAL"
	case pct >= 70:
		return "HIGH"
	case pct >= 50:
		return "ELEVATED"
	default:
		return "NOMINAL"
	}
}

// BurnRate holds calculated burn rate data.
type BurnRate struct {
	PercentPerHour float64
	TokensPerHour  float64
	TimeToExhaust  time.Duration // 0 means SAFE
	HasData        bool
}

// SortOrder for session table.
type SortOrder int

const (
	SortByName SortOrder = iota
	SortByStatus
	SortByLatency
)
```

- [ ] **Step 2: Verify compilation**

Run:
```bash
go build ./internal/data/
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add shared data types for bridge, session, and usage data"
```

---

### Task 3: Ring Buffer for Burn Rate Calculation

**Files:**
- Create: `internal/data/ringbuffer.go`
- Create: `internal/data/ringbuffer_test.go`

- [ ] **Step 1: Write the failing test for ring buffer**

Create `internal/data/ringbuffer_test.go`:

```go
package data

import (
	"testing"
	"time"
)

func TestRingBuffer_AddAndLen(t *testing.T) {
	rb := NewRingBuffer(5)
	if rb.Len() != 0 {
		t.Fatalf("expected len 0, got %d", rb.Len())
	}

	now := time.Now()
	rb.Add(Observation{Timestamp: now, FiveHourPct: 10.0, SevenDayPct: 20.0, TotalTokens: 1000})
	if rb.Len() != 1 {
		t.Fatalf("expected len 1, got %d", rb.Len())
	}

	// Fill past capacity
	for i := 0; i < 6; i++ {
		rb.Add(Observation{Timestamp: now.Add(time.Duration(i) * time.Second), FiveHourPct: float64(i), TotalTokens: i * 100})
	}
	if rb.Len() != 5 {
		t.Fatalf("expected len 5 (capped at capacity), got %d", rb.Len())
	}
}

func TestRingBuffer_BurnRate_NoData(t *testing.T) {
	rb := NewRingBuffer(100)
	rate := rb.CalcBurnRate(15 * time.Minute)
	if rate.HasData {
		t.Fatal("expected HasData=false with no observations")
	}
}

func TestRingBuffer_BurnRate_SinglePoint(t *testing.T) {
	rb := NewRingBuffer(100)
	rb.Add(Observation{Timestamp: time.Now(), FiveHourPct: 50.0, TotalTokens: 5000})
	rate := rb.CalcBurnRate(15 * time.Minute)
	if rate.HasData {
		t.Fatal("expected HasData=false with only one observation")
	}
}

func TestRingBuffer_BurnRate_LinearGrowth(t *testing.T) {
	rb := NewRingBuffer(100)
	start := time.Now().Add(-10 * time.Minute)

	// 10% over 10 minutes = 60% per hour
	for i := 0; i <= 10; i++ {
		rb.Add(Observation{
			Timestamp:   start.Add(time.Duration(i) * time.Minute),
			FiveHourPct: float64(20 + i), // 20% to 30%
			SevenDayPct: 50.0,
			TotalTokens: 1000 + i*500, // 500 tokens per minute = 30k/hour
		})
	}

	rate := rb.CalcBurnRate(15 * time.Minute)
	if !rate.HasData {
		t.Fatal("expected HasData=true")
	}

	// Should be ~60% per hour (10% in 10 minutes)
	if rate.PercentPerHour < 55 || rate.PercentPerHour > 65 {
		t.Fatalf("expected ~60%%/hour, got %.1f", rate.PercentPerHour)
	}

	// Should be ~30k tokens/hour (5000 tokens in 10 minutes)
	if rate.TokensPerHour < 28000 || rate.TokensPerHour > 32000 {
		t.Fatalf("expected ~30k tokens/hour, got %.0f", rate.TokensPerHour)
	}

	// Time to exhaustion: (100-30)/60 = ~70min
	if rate.TimeToExhaust < 65*time.Minute || rate.TimeToExhaust > 75*time.Minute {
		t.Fatalf("expected ~70min to exhaust, got %v", rate.TimeToExhaust)
	}
}

func TestRingBuffer_BurnRate_ZeroRate(t *testing.T) {
	rb := NewRingBuffer(100)
	start := time.Now().Add(-5 * time.Minute)

	// Flat usage = 0 rate
	for i := 0; i <= 5; i++ {
		rb.Add(Observation{
			Timestamp:   start.Add(time.Duration(i) * time.Minute),
			FiveHourPct: 40.0,
			TotalTokens: 5000,
		})
	}

	rate := rb.CalcBurnRate(15 * time.Minute)
	if !rate.HasData {
		t.Fatal("expected HasData=true")
	}
	if rate.TimeToExhaust != 0 {
		t.Fatalf("expected TimeToExhaust=0 (SAFE) for zero rate, got %v", rate.TimeToExhaust)
	}
}

func TestRingBuffer_BurnRate_WindowFilter(t *testing.T) {
	rb := NewRingBuffer(100)
	now := time.Now()

	// Old data point (30 min ago) - should be excluded from 15min window
	rb.Add(Observation{Timestamp: now.Add(-30 * time.Minute), FiveHourPct: 0.0, TotalTokens: 0})

	// Recent data (last 5 min)
	for i := 0; i <= 5; i++ {
		rb.Add(Observation{
			Timestamp:   now.Add(-5*time.Minute + time.Duration(i)*time.Minute),
			FiveHourPct: float64(50 + i*2), // 50% to 60%
			TotalTokens: 10000 + i*1000,
		})
	}

	rate := rb.CalcBurnRate(15 * time.Minute)
	if !rate.HasData {
		t.Fatal("expected HasData=true")
	}

	// 10% in 5 minutes = 120%/hour (not distorted by old 0% point)
	if rate.PercentPerHour < 110 || rate.PercentPerHour > 130 {
		t.Fatalf("expected ~120%%/hour, got %.1f", rate.PercentPerHour)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/data/ -v -run TestRingBuffer
```

Expected: Compilation error -- `NewRingBuffer`, `Observation` not defined.

- [ ] **Step 3: Implement ring buffer**

Create `internal/data/ringbuffer.go`:

```go
package data

import "time"

// Observation is a single data point in the ring buffer.
type Observation struct {
	Timestamp   time.Time
	FiveHourPct float64
	SevenDayPct float64
	TotalTokens int
}

// RingBuffer is a fixed-size circular buffer of observations.
type RingBuffer struct {
	buf  []Observation
	head int
	len  int
	cap  int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]Observation, capacity),
		cap: capacity,
	}
}

// Add appends an observation, overwriting the oldest if full.
func (rb *RingBuffer) Add(obs Observation) {
	rb.buf[rb.head] = obs
	rb.head = (rb.head + 1) % rb.cap
	if rb.len < rb.cap {
		rb.len++
	}
}

// Len returns the number of observations in the buffer.
func (rb *RingBuffer) Len() int {
	return rb.len
}

// entries returns all observations in chronological order within the given time window.
func (rb *RingBuffer) entries(window time.Duration) []Observation {
	if rb.len == 0 {
		return nil
	}

	cutoff := time.Now().Add(-window)
	var result []Observation

	// Walk the buffer in insertion order (oldest first)
	start := (rb.head - rb.len + rb.cap) % rb.cap
	for i := 0; i < rb.len; i++ {
		idx := (start + i) % rb.cap
		if rb.buf[idx].Timestamp.After(cutoff) {
			result = append(result, rb.buf[idx])
		}
	}
	return result
}

// CalcBurnRate computes burn rate from observations within the given window.
func (rb *RingBuffer) CalcBurnRate(window time.Duration) BurnRate {
	entries := rb.entries(window)
	if len(entries) < 2 {
		return BurnRate{HasData: false}
	}

	first := entries[0]
	last := entries[len(entries)-1]
	elapsed := last.Timestamp.Sub(first.Timestamp).Seconds()

	if elapsed < 1 {
		return BurnRate{HasData: false}
	}

	deltaPct := last.FiveHourPct - first.FiveHourPct
	deltaTokens := float64(last.TotalTokens - first.TotalTokens)

	pctPerHour := (deltaPct / elapsed) * 3600
	tokensPerHour := (deltaTokens / elapsed) * 3600

	var tte time.Duration
	if pctPerHour > 0 {
		hoursLeft := (100.0 - last.FiveHourPct) / pctPerHour
		tte = time.Duration(hoursLeft * float64(time.Hour))
	}

	// Clamp negative rates to zero
	if pctPerHour < 0 {
		pctPerHour = 0
	}
	if tokensPerHour < 0 {
		tokensPerHour = 0
	}

	return BurnRate{
		PercentPerHour: pctPerHour,
		TokensPerHour:  tokensPerHour,
		TimeToExhaust:  tte,
		HasData:        true,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/data/ -v -run TestRingBuffer
```

Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: implement ring buffer with burn rate calculation and tests"
```

---

### Task 4: Bridge File Reader

**Files:**
- Create: `internal/data/bridge.go`
- Create: `internal/data/bridge_test.go`

- [ ] **Step 1: Write failing tests for bridge reader**

Create `internal/data/bridge_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/data/ -v -run "TestReadBridge|TestLatestUsage"
```

Expected: Compilation error -- `ReadBridgeFiles`, `LatestUsage` not defined.

- [ ] **Step 3: Implement bridge reader**

Create `internal/data/bridge.go`:

```go
package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
)

// ReadBridgeFiles reads all /tmp/claude-monitor-*.json files from the given directory.
func ReadBridgeFiles(dir string) ([]BridgeData, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var bridges []BridgeData
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "claude-monitor-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		var bd BridgeData
		if err := json.Unmarshal(data, &bd); err != nil {
			continue // skip malformed
		}
		bridges = append(bridges, bd)
	}
	return bridges, nil
}

// ReadStateFiles reads all /tmp/claude-session-state-*.json files from the given directory.
func ReadStateFiles(dir string) (map[int]SessionState, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	states := make(map[int]SessionState)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "claude-session-state-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		var ss SessionState
		if err := json.Unmarshal(data, &ss); err != nil {
			continue
		}
		states[ss.PID] = ss
	}
	return states, nil
}

// LatestUsage returns the usage snapshot from the most recently updated bridge
// that has rate limit data.
func LatestUsage(bridges []BridgeData) UsageSnapshot {
	var best *BridgeData
	for i := range bridges {
		b := &bridges[i]
		if !b.HasRateLimits() {
			continue
		}
		if best == nil || b.Timestamp > best.Timestamp {
			best = b
		}
	}

	if best == nil {
		return UsageSnapshot{HasData: false}
	}

	lastUpdate := time.Unix(best.Timestamp, 0)
	isStale := time.Since(lastUpdate) > config.StaleThreshold

	return UsageSnapshot{
		FiveHour: WindowUsage{
			UsedPercentage: best.RateLimits.FiveHour.UsedPercentage,
			ResetsAt:       time.Unix(int64(best.RateLimits.FiveHour.ResetsAt), 0),
			Severity:       SeverityFor(best.RateLimits.FiveHour.UsedPercentage),
		},
		SevenDay: WindowUsage{
			UsedPercentage: best.RateLimits.SevenDay.UsedPercentage,
			ResetsAt:       time.Unix(int64(best.RateLimits.SevenDay.ResetsAt), 0),
			Severity:       SeverityFor(best.RateLimits.SevenDay.UsedPercentage),
		},
		TotalTokens: best.Tokens.Total(),
		HasData:     true,
		IsStale:     isStale,
		LastUpdate:  lastUpdate,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/data/ -v -run "TestReadBridge|TestLatestUsage"
```

Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: implement bridge file reader and usage snapshot extraction"
```

---

### Task 5: Session Data Reader

**Files:**
- Create: `internal/data/sessions.go`
- Create: `internal/data/sessions_test.go`

- [ ] **Step 1: Write failing tests for session reader**

Create `internal/data/sessions_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/data/ -v -run "TestReadSession|TestMerge|TestSessionHex|TestFormatLatency"
```

Expected: Compilation error -- functions not defined.

- [ ] **Step 3: Implement session reader and merger**

Create `internal/data/sessions.go`:

```go
package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// ReadSessionRegistry reads all ~/.claude/sessions/*.json files.
func ReadSessionRegistry(dir string) ([]SessionRegistry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var registries []SessionRegistry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var reg SessionRegistry
		if err := json.Unmarshal(data, &reg); err != nil {
			continue
		}
		registries = append(registries, reg)
	}
	return registries, nil
}

// IsPIDAlive checks if a process with the given PID exists.
func IsPIDAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

// MergeSessions combines registry, state, and bridge data into a unified view.
func MergeSessions(registries []SessionRegistry, states map[int]SessionState, bridges []BridgeData) []SessionInfo {
	// Index bridges by session ID
	bridgeBySession := make(map[string]BridgeData)
	for _, b := range bridges {
		existing, ok := bridgeBySession[b.SessionID]
		if !ok || b.Timestamp > existing.Timestamp {
			bridgeBySession[b.SessionID] = b
		}
	}

	var sessions []SessionInfo
	for _, reg := range registries {
		info := SessionInfo{
			PID:       reg.PID,
			SessionID: reg.SessionID,
			Cwd:       reg.Cwd,
			StartedAt: time.UnixMilli(reg.StartedAt),
		}

		// Name: use session name, fall back to last path component of cwd
		if reg.Name != "" {
			info.Name = reg.Name
		} else {
			info.Name = filepath.Base(reg.Cwd)
		}

		// PID liveness
		info.Alive = IsPIDAlive(reg.PID)

		// Status from state file
		if !info.Alive {
			info.Status = StatusZombie
			info.ZombieSince = time.Now()
		} else if state, ok := states[reg.PID]; ok {
			switch state.State {
			case "working":
				info.Status = StatusWorking
			case "waiting":
				info.Status = StatusWaiting
			case "blocked":
				info.Status = StatusBlocked
			default:
				info.Status = StatusIdle
			}
		} else {
			info.Status = StatusIdle
		}

		// Latency from bridge file
		if b, ok := bridgeBySession[reg.SessionID]; ok {
			info.LastBridge = time.Unix(b.Timestamp, 0)
		}

		sessions = append(sessions, info)
	}

	return sessions
}

// SessionHexID returns a short hex string from a PID, e.g. 44946 -> "0xAF92".
func SessionHexID(pid int) string {
	return fmt.Sprintf("0x%04X", pid&0xFFFF)
}

// FormatLatency returns a human-readable latency string.
func FormatLatency(lastBridge time.Time) string {
	if lastBridge.IsZero() {
		return "INF"
	}
	d := time.Since(lastBridge)
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/data/ -v -run "TestReadSession|TestMerge|TestSessionHex|TestFormatLatency"
```

Expected: All 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: implement session registry reader, PID check, and session merger"
```

---

### Task 6: Lipgloss Styles (Design System)

**Files:**
- Create: `internal/tui/styles.go`

- [ ] **Step 1: Create complete style definitions**

Create `internal/tui/styles.go`:

```go
package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// ── Surface colors ──────────────────────────────────────────────────────────

var (
	colSurface          = lipgloss.Color("#0d0d1c")
	colSurfaceContainer = lipgloss.Color("#18182a")
	colSurfaceBright    = lipgloss.Color("#2a2a42")
)

// ── Content colors ──────────────────────────────────────────────────────────

var (
	colOnSurface        = lipgloss.Color("#e6e3fa")
	colOnSurfaceVariant = lipgloss.Color("#c9c5dd")
	colOutlineVariant   = lipgloss.Color("#474658")
)

// ── Accent colors ───────────────────────────────────────────────────────────

var (
	colPrimary   = lipgloss.Color("#d1abfd")
	colSecondary = lipgloss.Color("#8cb7fe")
	colGreen     = lipgloss.Color("#a6e3a1")
	colYellow    = lipgloss.Color("#f9e2af")
	colPeach     = lipgloss.Color("#fab387")
	colRed       = lipgloss.Color("#f38ba8")
)

// ── Dim accent colors (for pulsing) ────────────────────────────────────────

var (
	colPeachDim = lipgloss.Color("#7d5944")
	colRedDim   = lipgloss.Color("#7a4654")
)

// ── Gradient stops for progress bars ────────────────────────────────────────

var (
	gradGreen  = mustParseHex("#a6e3a1")
	gradYellow = mustParseHex("#f9e2af")
	gradPeach  = mustParseHex("#fab387")
	gradRed    = mustParseHex("#f38ba8")
)

func mustParseHex(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		panic("invalid color: " + hex)
	}
	return c
}

// GradientColor returns the interpolated color for a percentage value (0-100).
func GradientColor(pct float64) lipgloss.Color {
	var c colorful.Color

	switch {
	case pct <= 50:
		t := pct / 50.0
		c = gradGreen.BlendRgb(gradYellow, t)
	case pct <= 70:
		t := (pct - 50) / 20.0
		c = gradYellow.BlendRgb(gradPeach, t)
	case pct <= 90:
		t := (pct - 70) / 20.0
		c = gradPeach.BlendRgb(gradRed, t)
	default:
		c = gradRed
	}

	return lipgloss.Color(c.Hex())
}

// ── Label style helpers ─────────────────────────────────────────────────────

// LetterSpace inserts spaces between characters for the "editorial" label look.
func LetterSpace(s string) string {
	runes := []rune(s)
	result := make([]rune, 0, len(runes)*2)
	for i, r := range runes {
		result = append(result, r)
		if i < len(runes)-1 {
			result = append(result, ' ')
		}
	}
	return string(result)
}

// ── Base styles ─────────────────────────────────────────────────────────────

var (
	// SurfaceBase is the root background.
	SurfaceBase = lipgloss.NewStyle().Background(colSurface)

	// SurfaceContainer is for major section backgrounds.
	SurfaceContainer = lipgloss.NewStyle().Background(colSurfaceContainer)

	// SurfaceBright is for elevated panels (burn rate).
	SurfaceBright = lipgloss.NewStyle().Background(colSurfaceBright)
)

// ── Typography styles ───────────────────────────────────────────────────────

var (
	// LabelMD is the small uppercase label style.
	LabelMD = lipgloss.NewStyle().
		Foreground(colOnSurfaceVariant)

	// HeadlineMD is the section headline style.
	HeadlineMD = lipgloss.NewStyle().
		Foreground(colOnSurface).
		Bold(true)

	// BodyLG is the primary text style.
	BodyLG = lipgloss.NewStyle().
		Foreground(colOnSurface)

	// ValueBold is for large data values.
	ValueBold = lipgloss.NewStyle().
		Foreground(colOnSurface).
		Bold(true)

	// ValueGreen is for token velocity.
	ValueGreen = lipgloss.NewStyle().
		Foreground(colGreen).
		Bold(true)

	// PrimaryText is for accented headers.
	PrimaryText = lipgloss.NewStyle().
		Foreground(colPrimary)

	// DimText is for metadata and low-importance text.
	DimText = lipgloss.NewStyle().
		Foreground(colOutlineVariant)
)

// ── Status chip styles ──────────────────────────────────────────────────────

var (
	StatusWorkingStyle = lipgloss.NewStyle().Foreground(colSecondary)
	StatusWaitingStyle = lipgloss.NewStyle().Foreground(colPeach)
	StatusBlockedStyle = lipgloss.NewStyle().Foreground(colRed)
	StatusZombieStyle  = lipgloss.NewStyle().Foreground(colOutlineVariant)
	StatusIdleStyle    = lipgloss.NewStyle().Foreground(colOnSurfaceVariant)

	StatusWaitingDimStyle = lipgloss.NewStyle().Foreground(colPeachDim)
	StatusBlockedDimStyle = lipgloss.NewStyle().Foreground(colRedDim)
)

// ── Spinner frames ──────────────────────────────────────────────────────────

var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ── Size warning style ──────────────────────────────────────────────────────

var SizeWarning = lipgloss.NewStyle().
	Foreground(colOnSurfaceVariant).
	Bold(true).
	Align(lipgloss.Center)
```

- [ ] **Step 2: Verify compilation**

Run:
```bash
go build ./internal/tui/
```

Expected: No errors. (Will fail until model.go imports are updated -- that's fine, we fix in next task.)

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: define lipgloss styles and color palette for Editorial Terminalism"
```

---

### Task 7: TUI Components -- Usage Bars

**Files:**
- Create: `internal/tui/components/usage.go`

- [ ] **Step 1: Implement usage bars component**

Create `internal/tui/components/usage.go`:

```go
package components

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/data"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
)

// RenderUsagePanel renders the left 2/3 usage statistics panel.
func RenderUsagePanel(usage data.UsageSnapshot, width int, isStale bool, animTick int) string {
	bg := tui.SurfaceContainer.Width(width)

	if !usage.HasData {
		noData := tui.LabelMD.Render("N O   D A T A")
		content := lipgloss.Place(width-4, 8, lipgloss.Center, lipgloss.Center, noData)
		return bg.Padding(1, 2).Render(content)
	}

	var b strings.Builder

	// Header line
	header := tui.LabelMD.Render(tui.LetterSpace("COMPUTE METRICS"))
	staleIndicator := ""
	if isStale {
		if animTick%10 < 5 {
			staleIndicator = tui.StatusWaitingStyle.Render("  [STALE]")
		} else {
			staleIndicator = tui.StatusWaitingDimStyle.Render("  [STALE]")
		}
	}
	hostname, _ := hostnameShort()
	nodeLabel := tui.LabelMD.Render(fmt.Sprintf("NODE: %s", hostname))

	headerLine := lipgloss.JoinHorizontal(lipgloss.Top,
		header+staleIndicator,
		strings.Repeat(" ", max(1, width-4-lipgloss.Width(header+staleIndicator)-lipgloss.Width(nodeLabel))),
		nodeLabel,
	)
	b.WriteString(headerLine + "\n")

	// Subheader
	b.WriteString(tui.HeadlineMD.Render("USAGE_STATISTICS") + "\n\n")

	// 5H bar
	b.WriteString(renderBar("5H WINDOW USAGE", usage.FiveHour, width-4))
	b.WriteString("\n")

	// 7D bar
	b.WriteString(renderBar("7D WINDOW USAGE", usage.SevenDay, width-4))

	return bg.Padding(1, 2).Render(b.String())
}

func renderBar(label string, w data.WindowUsage, barWidth int) string {
	var b strings.Builder

	// Colored square indicator
	gradColor := tui.GradientColor(w.UsedPercentage)
	square := lipgloss.NewStyle().Foreground(gradColor).Render("■")

	// Label line
	labelText := tui.LabelMD.Render(fmt.Sprintf("%s %s", square, tui.LetterSpace(label)))
	resetTime := tui.LabelMD.Render(fmt.Sprintf("RESETS_AT: %s", w.ResetsAt.Format("15:04")))

	labelLine := lipgloss.JoinHorizontal(lipgloss.Top,
		labelText,
		strings.Repeat(" ", max(1, barWidth-lipgloss.Width(labelText)-lipgloss.Width(resetTime))),
		resetTime,
	)
	b.WriteString(labelLine + "\n")

	// Progress bar
	fillWidth := barWidth - 2 // padding
	if fillWidth < 10 {
		fillWidth = 10
	}
	filled := int(w.UsedPercentage / 100.0 * float64(fillWidth))
	if filled > fillWidth {
		filled = fillWidth
	}
	if filled < 0 {
		filled = 0
	}

	var bar strings.Builder
	for i := 0; i < filled; i++ {
		pctAtPos := float64(i) / float64(fillWidth) * 100.0
		c := tui.GradientColor(pctAtPos)
		bar.WriteString(lipgloss.NewStyle().Foreground(c).Render("█"))
	}
	emptyColor := lipgloss.Color("#1a1a2e")
	for i := filled; i < fillWidth; i++ {
		bar.WriteString(lipgloss.NewStyle().Foreground(emptyColor).Render("░"))
	}

	// Overlay text centered on bar
	overlayText := fmt.Sprintf("%d%% %s", int(w.UsedPercentage), w.Severity)
	barStr := bar.String()

	// Build the bar line with overlay
	barLine := lipgloss.NewStyle().Width(barWidth).Render(barStr)

	// Place overlay text
	overlayStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#e6e3fa")).
		Bold(true)
	overlay := overlayStyle.Render(overlayText)

	// Center the overlay on the bar
	overlayPos := (barWidth - lipgloss.Width(overlay)) / 2
	if overlayPos < 0 {
		overlayPos = 0
	}

	// Rebuild with overlay -- for terminal, we just show bar + label below or inline
	// Since true overlay isn't possible, show percentage at the end
	pctLabel := lipgloss.NewStyle().
		Foreground(gradColor).
		Bold(true).
		Render(fmt.Sprintf(" %d%% %s", int(w.UsedPercentage), w.Severity))

	_ = barLine
	_ = overlay

	b.WriteString(bar.String() + pctLabel + "\n")

	return b.String()
}

func hostnameShort() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "LOCAL", nil
	}
	parts := strings.SplitN(h, ".", 2)
	return strings.ToUpper(parts[0]), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 2: Verify compilation**

Run:
```bash
go build ./internal/tui/components/
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: implement usage bars component with gradient progress bars"
```

---

### Task 8: TUI Components -- Burn Rate Panel

**Files:**
- Create: `internal/tui/components/burnrate.go`

- [ ] **Step 1: Implement burn rate panel component**

Create `internal/tui/components/burnrate.go`:

```go
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/data"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
)

// RenderBurnRatePanel renders the right 1/3 burn rate analysis panel.
func RenderBurnRatePanel(rate data.BurnRate, width, height int, animTick int) string {
	bg := tui.SurfaceBright.Width(width).Height(height)

	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Foreground(tui.ColPrimary()).
		Render("🔥 " + tui.LetterSpace("BURN_RATE_ANALYSIS"))
	b.WriteString(header + "\n\n")

	if !rate.HasData {
		b.WriteString(tui.LabelMD.Render(tui.LetterSpace("RATE_P_HOUR")) + "\n")
		b.WriteString(tui.ValueBold.Render("--") + "\n\n")
		b.WriteString(tui.LabelMD.Render(tui.LetterSpace("TOKEN_VELOCITY")) + "\n")
		b.WriteString(tui.ValueBold.Render("--") + "\n\n")
		b.WriteString(tui.LabelMD.Render(tui.LetterSpace("TIME_TO_EXHAUSTION")) + "\n")
		b.WriteString(tui.ValueBold.Render("--") + "\n")

		return bg.Padding(1, 2).Render(b.String())
	}

	// Rate per hour
	b.WriteString(tui.LabelMD.Render(tui.LetterSpace("RATE_P_HOUR")) + "\n")
	rateStr := fmt.Sprintf("%.1f%%", rate.PercentPerHour)
	rateColor := rateColor(rate.PercentPerHour)
	b.WriteString(lipgloss.NewStyle().Foreground(rateColor).Bold(true).Render(rateStr) + "\n\n")

	// Token velocity
	b.WriteString(tui.LabelMD.Render(tui.LetterSpace("TOKEN_VELOCITY")) + "\n")
	tokenStr := formatTokenVelocity(rate.TokensPerHour) + "  t/h"
	b.WriteString(tui.ValueGreen.Render(tokenStr) + "\n\n")

	// Time to exhaustion
	tteLabel := tui.LabelMD.Render(tui.LetterSpace("TIME_TO_EXHAUSTION"))
	b.WriteString(tteLabel + "\n")

	tteBlock := renderTTEBlock(rate, width-6, animTick)
	b.WriteString(tteBlock + "\n")

	return bg.Padding(1, 2).Render(b.String())
}

func renderTTEBlock(rate data.BurnRate, width, animTick int) string {
	innerBg := lipgloss.NewStyle().
		Background(tui.ColSurfaceContainer()).
		Padding(0, 1)

	var text string
	var style lipgloss.Style

	if rate.TimeToExhaust <= 0 {
		text = "SAFE"
		style = lipgloss.NewStyle().Foreground(tui.ColGreen()).Bold(true)
	} else {
		text = "LIMIT IN " + formatDuration(rate.TimeToExhaust)
		// Pulse when under 30 minutes
		if rate.TimeToExhaust < 30*time.Minute {
			if animTick%10 < 5 {
				style = lipgloss.NewStyle().Foreground(tui.ColRed()).Bold(true)
			} else {
				style = lipgloss.NewStyle().Foreground(tui.ColPeach()).Bold(true)
			}
		} else {
			style = lipgloss.NewStyle().Foreground(tui.ColOnSurface()).Bold(true)
		}
	}

	return innerBg.Render(style.Render(text))
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "~0M"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("~%dH %02dM", hours, minutes)
	}
	return fmt.Sprintf("~%dM", minutes)
}

func formatTokenVelocity(tokPerHour float64) string {
	if tokPerHour >= 1_000_000 {
		return fmt.Sprintf("%.1fM", tokPerHour/1_000_000)
	}
	if tokPerHour >= 1_000 {
		return fmt.Sprintf("%.0fk", tokPerHour/1_000)
	}
	return fmt.Sprintf("%.0f", tokPerHour)
}

func rateColor(pctPerHour float64) lipgloss.Color {
	switch {
	case pctPerHour >= 20:
		return tui.ColRed()
	case pctPerHour >= 10:
		return tui.ColPeach()
	case pctPerHour >= 5:
		return tui.ColYellow()
	default:
		return tui.ColOnSurface()
	}
}
```

- [ ] **Step 2: Add color accessor functions to styles.go**

The burn rate component needs color accessors (since lipgloss.Color is a string type, we need functions for use in the component). Add to `internal/tui/styles.go`:

```go
// Color accessors for use in components that need raw lipgloss.Color values.
func ColPrimary() lipgloss.Color         { return colPrimary }
func ColSecondary() lipgloss.Color        { return colSecondary }
func ColGreen() lipgloss.Color            { return colGreen }
func ColYellow() lipgloss.Color           { return colYellow }
func ColPeach() lipgloss.Color            { return colPeach }
func ColRed() lipgloss.Color              { return colRed }
func ColOnSurface() lipgloss.Color        { return colOnSurface }
func ColOnSurfaceVariant() lipgloss.Color { return colOnSurfaceVariant }
func ColSurfaceContainer() lipgloss.Color { return colSurfaceContainer }
func ColSurfaceBright() lipgloss.Color    { return colSurfaceBright }
func ColOutlineVariant() lipgloss.Color   { return colOutlineVariant }
```

- [ ] **Step 3: Verify compilation**

Run:
```bash
go build ./internal/tui/components/
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: implement burn rate panel with time-to-exhaustion and pulsing"
```

---

### Task 9: TUI Components -- Session Table

**Files:**
- Create: `internal/tui/components/sessions.go`

- [ ] **Step 1: Implement session table component**

Create `internal/tui/components/sessions.go`:

```go
package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/data"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
)

// RenderSessionTable renders the full-width active process matrix.
func RenderSessionTable(sessions []data.SessionInfo, width int, sortOrder data.SortOrder, animTick int) string {
	bg := tui.SurfaceContainer.Width(width)

	var b strings.Builder

	// Sort sessions
	sortSessions(sessions, sortOrder)

	// Compute load status
	loadStatus, loadColor := computeLoad(sessions)

	// Header
	icon := tui.PrimaryText.Render("≡")
	title := lipgloss.NewStyle().Foreground(tui.ColPrimary()).Bold(true).Render("  " + tui.LetterSpace("ACTIVE_PROCESS_MATRIX"))
	total := tui.LabelMD.Render(fmt.Sprintf("● TOTAL: %02d", len(sessions)))
	load := lipgloss.NewStyle().Foreground(loadColor).Render(fmt.Sprintf("● LOAD: %s", loadStatus))

	rightHeader := total + "    " + load
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top,
		icon+title,
		strings.Repeat(" ", max(1, width-4-lipgloss.Width(icon+title)-lipgloss.Width(rightHeader))),
		rightHeader,
	)
	b.WriteString(headerLine + "\n\n")

	// Column headers
	colWidths := columnWidths(width - 4)
	headers := []string{"SESSION_ID", "TASK_KERNEL", "LATENCY", "STATUS_BIT"}
	var headerCells []string
	for i, h := range headers {
		cell := tui.LabelMD.Width(colWidths[i]).Render(tui.LetterSpace(h))
		headerCells = append(headerCells, cell)
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...) + "\n")

	// Empty state
	if len(sessions) == 0 {
		emptyMsg := tui.LabelMD.Render("N O   A C T I V E   S E S S I O N S")
		b.WriteString("\n" + lipgloss.Place(width-4, 3, lipgloss.Center, lipgloss.Center, emptyMsg) + "\n")
		return bg.Padding(1, 2).Render(b.String())
	}

	// Rows
	for _, sess := range sessions {
		b.WriteString("\n")
		b.WriteString(renderSessionRow(sess, colWidths, animTick))
	}

	return bg.Padding(1, 2).Render(b.String())
}

func renderSessionRow(sess data.SessionInfo, colWidths [4]int, animTick int) string {
	// SESSION_ID
	hexID := data.SessionHexID(sess.PID)
	idStyle := lipgloss.NewStyle().Foreground(tui.ColPrimary()).Width(colWidths[0])
	if sess.Status == data.StatusZombie {
		idStyle = idStyle.Foreground(tui.ColOutlineVariant())
	}
	idCell := idStyle.Render(hexID)

	// TASK_KERNEL
	nameStyle := lipgloss.NewStyle().Foreground(tui.ColOnSurface()).Width(colWidths[1])
	if sess.Status == data.StatusZombie {
		nameStyle = nameStyle.Foreground(tui.ColOutlineVariant()).Italic(true)
	}
	nameCell := nameStyle.Render(sess.Name)

	// LATENCY
	latency := data.FormatLatency(sess.LastBridge)
	latencyStyle := lipgloss.NewStyle().Foreground(tui.ColOnSurfaceVariant()).Width(colWidths[2])
	if sess.Status == data.StatusZombie {
		latencyStyle = latencyStyle.Foreground(tui.ColOutlineVariant())
		latency = "--"
	}
	latencyCell := latencyStyle.Render(latency)

	// STATUS_BIT
	statusCell := renderStatusChip(sess.Status, colWidths[3], animTick)

	return lipgloss.JoinHorizontal(lipgloss.Top, idCell, nameCell, latencyCell, statusCell)
}

func renderStatusChip(status data.SessionStatus, width, animTick int) string {
	style := lipgloss.NewStyle().Width(width)

	switch status {
	case data.StatusWorking:
		frame := tui.SpinnerFrames[animTick%len(tui.SpinnerFrames)]
		return style.Render(tui.StatusWorkingStyle.Render(frame + " WORKING"))

	case data.StatusWaiting:
		if animTick%10 < 5 {
			return style.Render(tui.StatusWaitingStyle.Render("● WAITING"))
		}
		return style.Render(tui.StatusWaitingDimStyle.Render("● WAITING"))

	case data.StatusBlocked:
		// Faster pulse: 3 ticks on, 3 off (600ms cycle at 200ms tick)
		if animTick%6 < 3 {
			return style.Render(tui.StatusBlockedStyle.Render("⚠ BLOCKED"))
		}
		return style.Render(tui.StatusBlockedDimStyle.Render("⚠ BLOCKED"))

	case data.StatusZombie:
		return style.Render(tui.StatusZombieStyle.Render("ZOMBIE_STATE"))

	default: // Idle
		return style.Render(tui.StatusIdleStyle.Render("○ IDLE"))
	}
}

func columnWidths(totalWidth int) [4]int {
	// SESSION_ID: 15, TASK_KERNEL: flexible, LATENCY: 12, STATUS_BIT: 18
	idW := 15
	latencyW := 12
	statusW := 18
	nameW := totalWidth - idW - latencyW - statusW
	if nameW < 20 {
		nameW = 20
	}
	return [4]int{idW, nameW, latencyW, statusW}
}

func computeLoad(sessions []data.SessionInfo) (string, lipgloss.Color) {
	hasBlocked := false
	hasWaiting := false
	for _, s := range sessions {
		if s.Status == data.StatusBlocked {
			hasBlocked = true
		}
		if s.Status == data.StatusWaiting {
			hasWaiting = true
		}
	}
	if hasBlocked {
		return "CRITICAL", tui.ColRed()
	}
	if hasWaiting {
		return "ATTENTION", tui.ColPeach()
	}
	return "NOMINAL", tui.ColGreen()
}

func sortSessions(sessions []data.SessionInfo, order data.SortOrder) {
	sort.Slice(sessions, func(i, j int) bool {
		switch order {
		case data.SortByStatus:
			if sessions[i].Status != sessions[j].Status {
				return sessions[i].Status > sessions[j].Status // blocked first
			}
			return sessions[i].Name < sessions[j].Name
		case data.SortByLatency:
			if sessions[i].LastBridge.IsZero() && !sessions[j].LastBridge.IsZero() {
				return false
			}
			if !sessions[i].LastBridge.IsZero() && sessions[j].LastBridge.IsZero() {
				return true
			}
			return sessions[i].LastBridge.After(sessions[j].LastBridge)
		default: // SortByName
			return sessions[i].Name < sessions[j].Name
		}
	})
}
```

- [ ] **Step 2: Verify compilation**

Run:
```bash
go build ./internal/tui/components/
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: implement session table with status chips, sorting, and animations"
```

---

### Task 10: TUI Components -- Footer

**Files:**
- Create: `internal/tui/components/footer.go`

- [ ] **Step 1: Implement footer component**

Create `internal/tui/components/footer.go`:

```go
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
	"github.com/aaltwesthuis/claude-monitor/internal/tui"
)

// RenderFooter renders the bottom status bar.
func RenderFooter(startTime time.Time, width int) string {
	now := time.Now()

	// Left: version + timestamp
	left := tui.LabelMD.Render(
		fmt.Sprintf("CLAUDE_MONITOR_V%s // %s", config.Version, now.Format("2006-01-02T15:04:05Z")),
	)

	// Center: version tag
	center := tui.LabelMD.Render(config.VersionTag)

	// Right: uptime
	uptime := formatUptime(time.Since(startTime))
	right := tui.LabelMD.Render(fmt.Sprintf("UPTIME: %s", uptime))

	// Layout: left --- center --- right
	leftW := lipgloss.Width(left)
	centerW := lipgloss.Width(center)
	rightW := lipgloss.Width(right)

	gapTotal := width - leftW - centerW - rightW
	if gapTotal < 2 {
		gapTotal = 2
	}
	gapLeft := gapTotal / 2
	gapRight := gapTotal - gapLeft

	return lipgloss.NewStyle().
		Background(tui.ColSurfaceContainer()).
		Width(width).
		Render(
			left +
				strings.Repeat(" ", gapLeft) +
				center +
				strings.Repeat(" ", gapRight) +
				right,
		)
}

func formatUptime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
```

- [ ] **Step 2: Verify compilation**

Run:
```bash
go build ./internal/tui/components/
```

Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: implement footer bar with version, timestamp, and uptime"
```

---

### Task 11: Bubbletea Model -- Full Wiring

**Files:**
- Modify: `internal/tui/model.go`
- Create: `internal/tui/keymap.go`

- [ ] **Step 1: Create keymap**

Create `internal/tui/keymap.go`:

```go
package tui

import "github.com/charmbracelet/bubbletea"

func handleKey(msg tea.KeyMsg, m *Model) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return *m, tea.Quit
	case "r":
		return *m, m.forceRefresh()
	case "s":
		m.sortOrder = (m.sortOrder + 1) % 3
		return *m, nil
	}
	return *m, nil
}
```

- [ ] **Step 2: Rewrite model.go with full wiring**

Rewrite `internal/tui/model.go`:

```go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aaltwesthuis/claude-monitor/internal/config"
	"github.com/aaltwesthuis/claude-monitor/internal/data"
	"github.com/aaltwesthuis/claude-monitor/internal/tui/components"
)

// ── Messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time
type bridgeTickMsg time.Time
type stateTickMsg time.Time
type registryTickMsg time.Time
type pidTickMsg time.Time

// ── Model ───────────────────────────────────────────────────────────────────

type Model struct {
	width     int
	height    int
	ready     bool
	startTime time.Time

	// Data
	usage      data.UsageSnapshot
	burnRate   data.BurnRate
	sessions   []data.SessionInfo
	ringBuffer *data.RingBuffer

	// UI state
	animTick  int
	sortOrder data.SortOrder

	// Zombie tracking: PID -> time first detected dead
	zombies map[int]time.Time
}

func NewModel() Model {
	return Model{
		startTime:  time.Now(),
		ringBuffer: data.NewRingBuffer(config.RingBufferSize),
		zombies:    make(map[int]time.Time),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickAnimation(),
		tickBridge(),
		tickState(),
		tickRegistry(),
		tickPID(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		result, cmd := handleKey(msg, &m)
		return result, cmd

	case tickMsg:
		m.animTick++
		return m, tickAnimation()

	case bridgeTickMsg:
		m.refreshBridge()
		return m, tickBridge()

	case stateTickMsg:
		m.refreshState()
		return m, tickState()

	case registryTickMsg:
		m.refreshRegistry()
		return m, tickRegistry()

	case pidTickMsg:
		m.refreshPIDs()
		return m, tickPID()
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}

	if m.width < config.MinTermWidth || m.height < config.MinTermHeight {
		msg := SizeWarning.Width(m.width).Height(m.height).Render(
			"TERMINAL TOO SMALL -- REQUIRES 120+ COLUMNS, 30+ ROWS",
		)
		return msg
	}

	// Zone 1: Usage (2/3) + Burn Rate (1/3)
	usageWidth := m.width * 2 / 3
	burnWidth := m.width - usageWidth

	usagePanel := components.RenderUsagePanel(m.usage, usageWidth, m.usage.IsStale, m.animTick)
	burnPanel := components.RenderBurnRatePanel(m.burnRate, burnWidth, 0, m.animTick)

	topSection := lipgloss.JoinHorizontal(lipgloss.Top, usagePanel, burnPanel)

	// Zone 2: Session table
	sessionTable := components.RenderSessionTable(m.sessions, m.width, m.sortOrder, m.animTick)

	// Zone 3: Footer
	footer := components.RenderFooter(m.startTime, m.width)

	// Compose
	content := lipgloss.JoinVertical(lipgloss.Left,
		topSection,
		sessionTable,
		footer,
	)

	// Fill remaining space with base background
	return lipgloss.NewStyle().
		Background(colSurface).
		Width(m.width).
		Height(m.height).
		Render(content)
}

// ── Data refresh methods ────────────────────────────────────────────────────

func (m *Model) refreshBridge() {
	bridges, err := data.ReadBridgeFiles(config.BridgeDir)
	if err != nil {
		return
	}
	m.usage = data.LatestUsage(bridges)

	// Feed ring buffer
	if m.usage.HasData {
		m.ringBuffer.Add(data.Observation{
			Timestamp:   time.Now(),
			FiveHourPct: m.usage.FiveHour.UsedPercentage,
			SevenDayPct: m.usage.SevenDay.UsedPercentage,
			TotalTokens: m.usage.TotalTokens,
		})
		m.burnRate = m.ringBuffer.CalcBurnRate(time.Duration(config.BurnRateWindowMin) * time.Minute)
	}
}

func (m *Model) refreshState() {
	states, err := data.ReadStateFiles(config.BridgeDir)
	if err != nil {
		return
	}

	// Update session statuses from state files
	for i, sess := range m.sessions {
		if sess.Status == data.StatusZombie {
			continue
		}
		if state, ok := states[sess.PID]; ok {
			switch state.State {
			case "working":
				m.sessions[i].Status = data.StatusWorking
			case "waiting":
				m.sessions[i].Status = data.StatusWaiting
			case "blocked":
				m.sessions[i].Status = data.StatusBlocked
			default:
				m.sessions[i].Status = data.StatusIdle
			}
		}
	}
}

func (m *Model) refreshRegistry() {
	registries, err := data.ReadSessionRegistry(config.SessionsDir)
	if err != nil {
		return
	}

	states, _ := data.ReadStateFiles(config.BridgeDir)
	bridges, _ := data.ReadBridgeFiles(config.BridgeDir)

	newSessions := data.MergeSessions(registries, states, bridges)

	// Apply zombie tracking
	for i, sess := range newSessions {
		if sess.Status == data.StatusZombie {
			if since, ok := m.zombies[sess.PID]; ok {
				newSessions[i].ZombieSince = since
			} else {
				m.zombies[sess.PID] = time.Now()
				newSessions[i].ZombieSince = time.Now()
			}
		} else {
			delete(m.zombies, sess.PID)
		}
	}

	// Filter out zombies past cleanup delay
	var filtered []data.SessionInfo
	for _, sess := range newSessions {
		if sess.Status == data.StatusZombie && !sess.ZombieSince.IsZero() {
			if time.Since(sess.ZombieSince) > config.ZombieCleanupDelay {
				delete(m.zombies, sess.PID)
				continue
			}
		}
		filtered = append(filtered, sess)
	}

	m.sessions = filtered
}

func (m *Model) refreshPIDs() {
	for i, sess := range m.sessions {
		if sess.Status == data.StatusZombie {
			continue
		}
		if !data.IsPIDAlive(sess.PID) {
			m.sessions[i].Status = data.StatusZombie
			m.sessions[i].Alive = false
			if _, ok := m.zombies[sess.PID]; !ok {
				m.zombies[sess.PID] = time.Now()
				m.sessions[i].ZombieSince = time.Now()
			}
		}
	}
}

func (m *Model) forceRefresh() tea.Cmd {
	m.refreshBridge()
	m.refreshState()
	m.refreshRegistry()
	m.refreshPIDs()
	return nil
}

// ── Tick commands ───────────────────────────────────────────────────────────

func tickAnimation() tea.Cmd {
	return tea.Tick(config.TickAnimation, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func tickBridge() tea.Cmd {
	return tea.Tick(config.PollBridge, func(t time.Time) tea.Msg {
		return bridgeTickMsg(t)
	})
}

func tickState() tea.Cmd {
	return tea.Tick(config.PollState, func(t time.Time) tea.Msg {
		return stateTickMsg(t)
	})
}

func tickRegistry() tea.Cmd {
	return tea.Tick(config.PollRegistry, func(t time.Time) tea.Msg {
		return registryTickMsg(t)
	})
}

func tickPID() tea.Cmd {
	return tea.Tick(config.PollPID, func(t time.Time) tea.Msg {
		return pidTickMsg(t)
	})
}
```

- [ ] **Step 3: Verify full compilation**

Run:
```bash
go build ./cmd/monitor/
```

Expected: Binary compiles successfully.

- [ ] **Step 4: Run all tests**

Run:
```bash
go test ./... -v
```

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: wire up bubbletea model with all components, data polling, and animations"
```

---

### Task 12: Extend Statusline Binary (Bridge Writer)

**Files:**
- Modify: `~/.claude/statusline-src/main.go`

- [ ] **Step 1: Add RateLimits to StatusInput struct**

In `~/.claude/statusline-src/main.go`, add the `RateLimits` field to the `StatusInput` struct (after line 24, before the closing brace on line 25):

```go
type StatusInput struct {
	Model         Model         `json:"model"`
	ContextWindow ContextWindow `json:"context_window"`
	Cost          Cost          `json:"cost"`
	Exceeds200k   bool          `json:"exceeds_200k_tokens"`
	Cwd           string        `json:"cwd"`
	SessionID     string        `json:"session_id"`
	RateLimits    struct {
		FiveHour struct {
			UsedPercentage float64 `json:"used_percentage"`
			ResetsAt       float64 `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay struct {
			UsedPercentage float64 `json:"used_percentage"`
			ResetsAt       float64 `json:"resets_at"`
		} `json:"seven_day"`
	} `json:"rate_limits"`
}
```

- [ ] **Step 2: Add writeMonitorBridge function**

Add this function before `main()` (e.g., after `writeGitCache` around line 239):

```go
func writeMonitorBridge(input StatusInput) {
	if input.SessionID == "" {
		return
	}
	bridgePath := filepath.Join(os.TempDir(), "claude-monitor-"+input.SessionID+".json")
	cacheT := input.ContextWindow.CurrentUsage.CacheReadInputTokens + input.ContextWindow.CurrentUsage.CacheCreationInputTokens
	bridgeData, _ := json.Marshal(map[string]any{
		"session_id": input.SessionID,
		"timestamp":  time.Now().Unix(),
		"rate_limits": map[string]any{
			"five_hour": map[string]any{
				"used_percentage": input.RateLimits.FiveHour.UsedPercentage,
				"resets_at":       input.RateLimits.FiveHour.ResetsAt,
			},
			"seven_day": map[string]any{
				"used_percentage": input.RateLimits.SevenDay.UsedPercentage,
				"resets_at":       input.RateLimits.SevenDay.ResetsAt,
			},
		},
		"tokens": map[string]any{
			"input":          input.ContextWindow.CurrentUsage.InputTokens,
			"output":         input.ContextWindow.CurrentUsage.OutputTokens,
			"cache_read":     input.ContextWindow.CurrentUsage.CacheReadInputTokens,
			"cache_creation": input.ContextWindow.CurrentUsage.CacheCreationInputTokens,
			"total_input":    input.ContextWindow.TotalInputTokens,
			"total_output":   input.ContextWindow.TotalOutputTokens,
		},
		"model": input.Model.DisplayName,
		"cwd":   input.Cwd,
	})
	_ = os.WriteFile(bridgePath, bridgeData, 0644)
}
```

- [ ] **Step 3: Call writeMonitorBridge from main**

In `main()`, after the existing bridge write (after line 396), add:

```go
	writeMonitorBridge(input)
```

- [ ] **Step 4: Rebuild the statusline binary**

Run:
```bash
cd ~/.claude/statusline-src && go build -o ~/.claude/statusline-bin .
```

Expected: Binary compiles, replaces the existing one.

- [ ] **Step 5: Verify the bridge write works**

Test by piping sample JSON:

```bash
echo '{"model":{"display_name":"Claude Sonnet 4"},"context_window":{"used_percentage":42,"remaining_percentage":58,"context_window_size":200000,"current_usage":{"input_tokens":1000,"output_tokens":500,"cache_read_input_tokens":200,"cache_creation_input_tokens":100},"total_input_tokens":5000,"total_output_tokens":3000},"cost":{"total_lines_added":10,"total_lines_removed":5},"exceeds_200k_tokens":false,"cwd":"/tmp/test","session_id":"test-verify","rate_limits":{"five_hour":{"used_percentage":25.5,"resets_at":1775000000},"seven_day":{"used_percentage":60.0,"resets_at":1775500000}}}' | ~/.claude/statusline-bin > /dev/null 2>&1; cat /tmp/claude-monitor-test-verify.json | jq .
```

Expected: JSON output with `rate_limits`, `tokens`, `model`, `cwd` fields.

- [ ] **Step 6: Clean up test file and commit (no commit -- this is outside our repo)**

```bash
rm -f /tmp/claude-monitor-test-verify.json
```

Note: The statusline binary changes are outside the `claude-monitor` repo. No git commit here -- these are infrastructure changes to `~/.claude/`.

---

### Task 13: Extend Tmux Hook (Session State Writer)

**Files:**
- Modify: `~/.claude/hooks/claude-tmux-status.sh`

- [ ] **Step 1: Add session state writer function**

Add this function before the `# Main` section (before line 112) in `~/.claude/hooks/claude-tmux-status.sh`:

```bash
# Write session state for claude-monitor TUI
write_monitor_state() {
  local event="$1" state="$2"
  local pid="$$"
  local session_id=""

  # Try to get session_id from stdin JSON (if available)
  # For notification events, stdin has JSON with session_id
  if [[ -n "${CLAUDE_SESSION_ID:-}" ]]; then
    session_id="$CLAUDE_SESSION_ID"
  fi

  local state_file="/tmp/claude-session-state-${pid}.json"
  printf '{"pid":%d,"session_id":"%s","state":"%s","updated_at":%d,"event":"%s"}\n' \
    "$pid" "$session_id" "$state" "$(date +%s)" "$event" > "$state_file"
}
```

- [ ] **Step 2: Update the case statement to call write_monitor_state**

Replace the case statement (lines 116-122):

```bash
case "${1:-}" in
  stop)
    set_icon "$pane_id" "○"
    write_monitor_state "stop" "idle"
    ;;
  notification)
    set_icon "$pane_id" "◐"
    # Check if this is a permission prompt
    local input=""
    input="$(cat)" 2>/dev/null || true
    if echo "$input" | grep -qiE 'permission|allow|approve|deny|trust'; then
      write_monitor_state "notification" "blocked"
    else
      write_monitor_state "notification" "waiting"
    fi
    ;;
  pretooluse)
    set_icon "$pane_id" "◐"
    write_monitor_state "pretooluse" "working"
    ;;
  restore)      restore "$pane_id" ;;
  cleanup)      cleanup_stale ;;
  *)            echo "Usage: $0 [stop|notification|pretooluse|restore|cleanup]" >&2; exit 1 ;;
esac
```

- [ ] **Step 3: Verify the hook still works**

Run:
```bash
bash -n ~/.claude/hooks/claude-tmux-status.sh
```

Expected: No syntax errors.

- [ ] **Step 4: Test the state writer**

```bash
CLAUDE_SESSION_ID=test-session ~/.claude/hooks/claude-tmux-status.sh pretooluse 2>/dev/null; cat /tmp/claude-session-state-$$.json 2>/dev/null | jq . || echo "State file written (tmux not available for icon, but state file should exist)"
```

Note: This may fail on the tmux icon part if not in tmux, but the state file should be written.

- [ ] **Step 5: Clean up test files**

```bash
rm -f /tmp/claude-session-state-$$.json
```

Note: Hook changes are outside the `claude-monitor` repo. No git commit here.

---

### Task 14: Build, Run, and Verify

**Files:**
- No new files

- [ ] **Step 1: Run all tests**

Run:
```bash
cd /Users/aaltwesthuis/Sources/playground/claude-monitor
go test ./... -v
```

Expected: All tests pass.

- [ ] **Step 2: Build the binary**

Run:
```bash
go build -o claude-monitor ./cmd/monitor/
```

Expected: Binary compiles successfully.

- [ ] **Step 3: Run a quick smoke test**

Run:
```bash
# Write a fake bridge file for testing
cat > /tmp/claude-monitor-smoke-test.json << 'EOF'
{
  "session_id": "smoke-test",
  "timestamp": TIMESTAMP,
  "rate_limits": {
    "five_hour": {"used_percentage": 42.0, "resets_at": RESET5},
    "seven_day": {"used_percentage": 73.0, "resets_at": RESET7}
  },
  "tokens": {"input": 1000, "output": 500, "cache_read": 200, "cache_creation": 50, "total_input": 15000, "total_output": 8000},
  "model": "Claude Sonnet 4",
  "cwd": "/tmp/test"
}
EOF
# Replace timestamps with actual values
sed -i '' "s/TIMESTAMP/$(date +%s)/g" /tmp/claude-monitor-smoke-test.json
sed -i '' "s/RESET5/$(date -v+3H +%s)/g" /tmp/claude-monitor-smoke-test.json
sed -i '' "s/RESET7/$(date -v+5d +%s)/g" /tmp/claude-monitor-smoke-test.json
```

Then run the monitor briefly:

```bash
timeout 3 ./claude-monitor || true
```

Expected: TUI renders for 3 seconds showing the usage bars with test data, then exits. No panics.

- [ ] **Step 4: Clean up test data**

```bash
rm -f /tmp/claude-monitor-smoke-test.json
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: verify build and tests pass"
```

---

### Task 15: Polish and Compilation Fixes

This is a catch-all task for any compilation errors, import mismatches, or minor issues discovered during Tasks 1-14. It is expected that wiring everything together will surface a few issues.

**Files:**
- Potentially any file created in Tasks 1-14

- [ ] **Step 1: Run `go vet ./...`**

Run:
```bash
go vet ./...
```

Fix any issues reported.

- [ ] **Step 2: Run `go build ./...`**

Run:
```bash
go build ./...
```

Fix any compilation errors.

- [ ] **Step 3: Run all tests one final time**

Run:
```bash
go test ./... -v -count=1
```

Expected: All tests pass.

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve compilation and vet issues from integration"
```
