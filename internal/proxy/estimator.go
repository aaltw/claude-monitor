package proxy

import "time"

// Estimate converts a ParseResult into a ContextSnapshot by scaling byte counts
// proportionally to the actual input token count from the API response.
func Estimate(parsed ParseResult, actualInputTokens int, contextWindowSize int) ContextSnapshot {
	snap := ContextSnapshot{
		Timestamp:         time.Now(),
		Model:             parsed.Model,
		TotalInputTokens:  actualInputTokens,
		ContextWindowSize: contextWindowSize,
		TurnNumber:        parsed.TurnNumber,
		Categories:        make(map[string]CategoryTokens, 5),
	}

	if contextWindowSize > 0 {
		snap.UsedPct = float64(actualInputTokens) / float64(contextWindowSize) * 100
	}

	if parsed.TotalBytes == 0 || actualInputTokens == 0 {
		for _, cat := range []string{CatSystem, CatTools, CatHistory, CatResults, CatThinking} {
			snap.Categories[cat] = CategoryTokens{Bytes: parsed.CategoryBytes[cat]}
		}
		return snap
	}

	for _, cat := range []string{CatSystem, CatTools, CatHistory, CatResults, CatThinking} {
		bytes := parsed.CategoryBytes[cat]
		tokens := int(float64(bytes) / float64(parsed.TotalBytes) * float64(actualInputTokens))
		pct := float64(tokens) / float64(actualInputTokens) * 100
		snap.Categories[cat] = CategoryTokens{
			Tokens: tokens,
			Pct:    pct,
			Bytes:  bytes,
		}
	}

	return snap
}
