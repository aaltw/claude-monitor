package proxy

import "time"

// ContextSnapshot is a single composition measurement from one API call.
type ContextSnapshot struct {
	SessionID         string                    `json:"session_id"`
	Timestamp         time.Time                 `json:"timestamp"`
	Model             string                    `json:"model"`
	TotalInputTokens  int                       `json:"total_input_tokens"`
	ContextWindowSize int                       `json:"context_window_size"`
	UsedPct           float64                   `json:"used_pct"`
	Categories        map[string]CategoryTokens `json:"categories"`
	TurnNumber        int                       `json:"turn_number"`
}

// CategoryTokens holds the token estimate for one category.
type CategoryTokens struct {
	Tokens int     `json:"tokens"`
	Pct    float64 `json:"pct"`
	Bytes  int     `json:"bytes"`
}

// Category keys.
const (
	CatSystem   = "system"
	CatTools    = "tools"
	CatHistory  = "history"
	CatResults  = "results"
	CatThinking = "thinking"
)

// ContextWindowSizes maps model ID prefixes to their context window size.
// If extended thinking is enabled, use 1,000,000 instead.
var ContextWindowSizes = map[string]int{
	"claude-opus-4":   200000,
	"claude-sonnet-4": 200000,
	"claude-haiku-4":  200000,
}

// LookupContextWindowSize returns the context window size for a model ID.
// Falls back to 200000 if unknown.
func LookupContextWindowSize(modelID string, hasExtendedThinking bool) int {
	if hasExtendedThinking {
		return 1000000
	}
	for prefix, size := range ContextWindowSizes {
		if len(modelID) >= len(prefix) && modelID[:len(prefix)] == prefix {
			return size
		}
	}
	return 200000
}
