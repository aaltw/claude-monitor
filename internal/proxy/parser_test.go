package proxy

import "testing"

func TestParseRequestBody_AllCategories(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"system": "You are Claude Code, a helpful assistant.",
		"tools": [
			{"name": "Read", "description": "Reads a file", "input_schema": {"type": "object"}},
			{"name": "Edit", "description": "Edits a file", "input_schema": {"type": "object"}}
		],
		"messages": [
			{"role": "user", "content": "Fix the bug in main.go"},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Let me analyze the code and find the bug."},
				{"type": "text", "text": "I'll read the file first."},
				{"type": "tool_use", "id": "tu_1", "name": "Read", "input": {"file_path": "main.go"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "tu_1", "content": "package main\nfunc main() {}"}
			]}
		]
	}`)

	result, err := ParseRequestBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %s", result.Model)
	}
	if result.HasExtendedThinking {
		t.Error("expected HasExtendedThinking=false")
	}

	// All 5 categories should have non-zero bytes
	for _, cat := range []string{CatSystem, CatTools, CatHistory, CatResults, CatThinking} {
		if result.CategoryBytes[cat] == 0 {
			t.Errorf("expected non-zero bytes for category %s", cat)
		}
	}

	// System should be smaller than tools (2 tool definitions with schemas)
	if result.CategoryBytes[CatSystem] >= result.CategoryBytes[CatTools] {
		t.Errorf("expected system (%d) < tools (%d)",
			result.CategoryBytes[CatSystem], result.CategoryBytes[CatTools])
	}
}

func TestParseRequestBody_SystemAsArray(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"system": [
			{"type": "text", "text": "You are Claude Code."},
			{"type": "text", "text": "Follow these instructions."}
		],
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`)

	result, err := ParseRequestBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CategoryBytes[CatSystem] == 0 {
		t.Error("expected non-zero system bytes for array-format system")
	}
}

func TestParseRequestBody_ExtendedThinking(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"system": "You are helpful.",
		"thinking": {"type": "enabled", "budget_tokens": 10000},
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`)

	result, err := ParseRequestBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasExtendedThinking {
		t.Error("expected HasExtendedThinking=true")
	}
}

func TestParseRequestBody_EmptyMessages(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"system": "You are helpful.",
		"messages": []
	}`)

	result, err := ParseRequestBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CategoryBytes[CatSystem] == 0 {
		t.Error("expected non-zero system bytes")
	}
	if result.TotalBytes == 0 {
		t.Error("expected non-zero total bytes")
	}
}

func TestParseRequestBody_UserContentAsString(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4-6",
		"messages": [
			{"role": "user", "content": "Just a plain string message"}
		]
	}`)

	result, err := ParseRequestBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CategoryBytes[CatHistory] == 0 {
		t.Error("expected non-zero history bytes for string content")
	}
}
