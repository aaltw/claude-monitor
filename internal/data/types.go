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
