package web

import "time"

// StateMsg is the full dashboard state pushed to clients.
type StateMsg struct {
	Type     string              `json:"type"`
	Usage    UsageMsg            `json:"usage"`
	BurnRate BurnRateMsg         `json:"burn_rate"`
	Sessions []SessionMsg        `json:"sessions"`
	Models   map[string]ModelMsg `json:"models"`
}

type UsageMsg struct {
	HasData     bool      `json:"has_data"`
	IsStale     bool      `json:"is_stale"`
	FiveHour    WindowMsg `json:"five_hour"`
	SevenDay    WindowMsg `json:"seven_day"`
	TotalTokens int       `json:"total_tokens"`
}

type WindowMsg struct {
	UsedPct  float64 `json:"used_pct"`
	ResetsAt string  `json:"resets_at"`
	Severity string  `json:"severity"`
}

type BurnRateMsg struct {
	HasData       bool    `json:"has_data"`
	PctPerHour    float64 `json:"pct_per_hour"`
	TokensPerHour float64 `json:"tokens_per_hour"`
	TTEMinutes    float64 `json:"tte_minutes"`
}

type SessionMsg struct {
	PID     int    `json:"pid"`
	HexID   string `json:"hex_id"`
	Name    string `json:"name"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Latency string `json:"latency"`
	Cwd     string `json:"cwd"`
}

type ModelMsg struct {
	TotalTokens int     `json:"total_tokens"`
	Pct         float64 `json:"pct"`
}

// HistoryMsg is a chart data point pushed periodically.
type HistoryMsg struct {
	Type          string         `json:"type"`
	Timestamp     time.Time      `json:"timestamp"`
	FiveHourPct   float64        `json:"five_hour_pct"`
	SevenDayPct   float64        `json:"seven_day_pct"`
	BurnRatePct   float64        `json:"burn_rate_pct_per_hour"`
	TotalTokens   int            `json:"total_tokens"`
	TokensByModel map[string]int `json:"tokens_by_model"`
}

// EventMsg is a session state change event.
type EventMsg struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	PID       int       `json:"pid"`
	Session   string    `json:"session"`
	Model     string    `json:"model"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
}

// ContextMsg wraps a composition snapshot for WebSocket broadcast.
type ContextMsg struct {
	Type              string                        `json:"type"`
	SessionID         string                        `json:"session_id"`
	Timestamp         time.Time                     `json:"timestamp"`
	Model             string                        `json:"model"`
	TotalInputTokens  int                           `json:"total_input_tokens"`
	ContextWindowSize int                           `json:"context_window_size"`
	UsedPct           float64                       `json:"used_pct"`
	Categories        map[string]ContextCategoryMsg `json:"categories"`
	TurnNumber        int                           `json:"turn_number"`
}

// ContextCategoryMsg holds per-category token data for WebSocket messages.
type ContextCategoryMsg struct {
	Tokens int     `json:"tokens"`
	Pct    float64 `json:"pct"`
	Bytes  int     `json:"bytes"`
}

// ClauditorMsg is broadcast on the same WS channel as state/history/context.
// It blends three clauditor CLI calls:
//   - `sessions --json` → ranked list (Sessions)
//   - `status   --json` → detailed focus session with context % (Focus)
//   - `impact   --json` → lifetime stats (Lifetime* fields)
type ClauditorMsg struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	HasData   bool      `json:"has_data"`

	Sessions []ClauditorSessionMsg `json:"sessions"`
	Focus    *ClauditorFocusMsg    `json:"focus"`

	// Lifetime impact (from `clauditor impact --json`).
	SessionsMonitored   int            `json:"sessions_monitored"`
	TotalTurnsMonitored int            `json:"total_turns_monitored"`
	HealthySessionPct   int            `json:"healthy_session_pct"`
	AvgCacheRatio       float64        `json:"avg_cache_ratio"`
	Detected            map[string]int `json:"detected"`
}

// ClauditorSessionMsg is one entry in `clauditor sessions --json`.
type ClauditorSessionMsg struct {
	Label       string    `json:"label"`
	Model       string    `json:"model"`
	Turns       int       `json:"turns"`
	LastUpdated time.Time `json:"last_updated"`
	CacheStatus string    `json:"cache_status"`
	CacheRatio  float64   `json:"cache_ratio"`
	Cost        float64   `json:"cost"`
	SpikeTurns  int       `json:"spike_turns"`
}

// JSONLContextMsg is a context-composition snapshot derived from Claude
// Code's own per-session JSONL logs (~/.claude/projects/*/*.jsonl) instead
// of the reverse proxy. Works without any ANTHROPIC_BASE_URL rerouting.
type JSONLContextMsg struct {
	Type        string    `json:"type"`
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	ProjectSlug string    `json:"project_slug"`
	Model       string    `json:"model"`
	TurnNumber  int       `json:"turn_number"`

	// Most recent turn's usage breakdown.
	InputTokens   int `json:"input_tokens"`
	CacheCreation int `json:"cache_creation_tokens"`
	CacheRead     int `json:"cache_read_tokens"`
	OutputTokens  int `json:"output_tokens"`

	// Derived metrics.
	TotalInputTokens  int       `json:"total_input_tokens"`
	ContextWindowSize int       `json:"context_window_size"`
	UsedPct           float64   `json:"used_pct"`
	CacheHitRatio     float64   `json:"cache_hit_ratio"`
	InputDelta        int       `json:"input_delta"`
	LastTurnAt        time.Time `json:"last_turn_at"`
	ActiveSessions    int       `json:"active_sessions"`

	// Up to N recent turns for timeline backfill.
	Turns []JSONLTurnMsg `json:"turns"`
}

// JSONLTurnMsg is one assistant turn's usage, keyed for timeline charts.
type JSONLTurnMsg = turnEntry

// ClauditorFocusMsg is the detailed focus session from `clauditor status --json`.
// Includes context-window fields that the sessions list doesn't carry.
type ClauditorFocusMsg struct {
	Session        string  `json:"session"`
	Model          string  `json:"model"`
	Turns          int     `json:"turns"`
	CacheStatus    string  `json:"cache_status"`
	CacheRatio     float64 `json:"cache_ratio"`
	ContextPercent float64 `json:"context_percent"`
	ContextTokens  int     `json:"context_tokens"`
	LoopDetected   bool    `json:"loop_detected"`
	Cost           float64 `json:"cost"`
	SavedByCache   float64 `json:"saved_by_cache"`
}
