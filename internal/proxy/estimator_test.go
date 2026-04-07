package proxy

import "testing"

func TestEstimate_ProportionalScaling(t *testing.T) {
	parsed := ParseResult{
		Model: "claude-sonnet-4-6",
		CategoryBytes: map[string]int{
			CatSystem:   1000,
			CatTools:    4000,
			CatHistory:  2000,
			CatResults:  2500,
			CatThinking: 500,
		},
		TotalBytes: 10000,
		TurnNumber: 5,
	}

	snap := Estimate(parsed, 3000, 200000)

	if snap.TotalInputTokens != 3000 {
		t.Errorf("expected 3000 total input tokens, got %d", snap.TotalInputTokens)
	}

	// Check proportional scaling: system = 1000/10000 * 3000 = 300
	if snap.Categories[CatSystem].Tokens != 300 {
		t.Errorf("expected system=300, got %d", snap.Categories[CatSystem].Tokens)
	}
	if snap.Categories[CatTools].Tokens != 1200 {
		t.Errorf("expected tools=1200, got %d", snap.Categories[CatTools].Tokens)
	}

	// Check percentages sum to 100
	totalPct := 0.0
	for _, cat := range snap.Categories {
		totalPct += cat.Pct
	}
	if totalPct < 99.9 || totalPct > 100.1 {
		t.Errorf("expected percentages to sum to ~100, got %.1f", totalPct)
	}

	// Check used_pct
	expectedUsedPct := float64(3000) / float64(200000) * 100
	if snap.UsedPct < expectedUsedPct-0.1 || snap.UsedPct > expectedUsedPct+0.1 {
		t.Errorf("expected used_pct ~%.1f, got %.1f", expectedUsedPct, snap.UsedPct)
	}
}

func TestEstimate_ZeroTotalBytes(t *testing.T) {
	parsed := ParseResult{
		Model: "claude-sonnet-4-6",
		CategoryBytes: map[string]int{
			CatSystem:   0,
			CatTools:    0,
			CatHistory:  0,
			CatResults:  0,
			CatThinking: 0,
		},
		TotalBytes: 0,
		TurnNumber: 0,
	}

	snap := Estimate(parsed, 0, 200000)
	if snap.TotalInputTokens != 0 {
		t.Errorf("expected 0 total input tokens, got %d", snap.TotalInputTokens)
	}
}
