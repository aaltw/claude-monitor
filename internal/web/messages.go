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
