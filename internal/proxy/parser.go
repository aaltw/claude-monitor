package proxy

import "encoding/json"

// ParseResult holds the raw byte counts per category from parsing an API request.
type ParseResult struct {
	Model               string
	HasExtendedThinking bool
	CategoryBytes       map[string]int
	TotalBytes          int
	TurnNumber          int // count of user messages
}

// partialRequest decodes only the top-level fields we need.
type partialRequest struct {
	Model    string           `json:"model"`
	System   json.RawMessage  `json:"system"`
	Tools    json.RawMessage  `json:"tools"`
	Messages []partialMessage `json:"messages"`
	Thinking json.RawMessage  `json:"thinking"`
}

type partialMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// contentBlockType peeks at a content block's "type" field.
type contentBlockType struct {
	Type string `json:"type"`
}

// ParseRequestBody parses an Anthropic Messages API request body and returns
// byte counts per category.
func ParseRequestBody(body []byte) (ParseResult, error) {
	var req partialRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return ParseResult{}, err
	}

	cats := map[string]int{
		CatSystem:   0,
		CatTools:    0,
		CatHistory:  0,
		CatResults:  0,
		CatThinking: 0,
	}

	// System prompt
	if len(req.System) > 0 {
		cats[CatSystem] = len(req.System)
	}

	// Tool definitions
	if len(req.Tools) > 0 {
		cats[CatTools] = len(req.Tools)
	}

	// Check for extended thinking config
	hasExtendedThinking := false
	if len(req.Thinking) > 0 {
		var tc map[string]any
		if json.Unmarshal(req.Thinking, &tc) == nil {
			if _, ok := tc["budget_tokens"]; ok {
				hasExtendedThinking = true
			}
		}
	}

	// Messages: classify content blocks
	turnNumber := 0
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			turnNumber++
		}
		classifyMessage(msg, cats)
	}

	total := 0
	for _, b := range cats {
		total += b
	}

	return ParseResult{
		Model:               req.Model,
		HasExtendedThinking: hasExtendedThinking,
		CategoryBytes:       cats,
		TotalBytes:          total,
		TurnNumber:          turnNumber,
	}, nil
}

// classifyMessage adds the byte count of a message's content to the appropriate categories.
func classifyMessage(msg partialMessage, cats map[string]int) {
	if len(msg.Content) == 0 {
		return
	}

	// Content can be a string (user shorthand) or an array of content blocks.
	// Try string first.
	var str string
	if json.Unmarshal(msg.Content, &str) == nil {
		// Plain string content is always conversation history
		cats[CatHistory] += len(msg.Content)
		return
	}

	// Array of content blocks
	var blocks []json.RawMessage
	if json.Unmarshal(msg.Content, &blocks) != nil {
		// Can't parse — dump all bytes into history as fallback
		cats[CatHistory] += len(msg.Content)
		return
	}

	for _, block := range blocks {
		var bt contentBlockType
		if json.Unmarshal(block, &bt) != nil {
			cats[CatHistory] += len(block)
			continue
		}

		switch bt.Type {
		case "thinking":
			cats[CatThinking] += len(block)
		case "tool_result":
			cats[CatResults] += len(block)
		case "tool_use", "text":
			cats[CatHistory] += len(block)
		default:
			cats[CatHistory] += len(block)
		}
	}
}
