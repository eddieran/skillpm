package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/memory/eventlog"
)

func TestGeminiParser_SessionFormat(t *testing.T) {
	dir := t.TempDir()
	session := geminiSession{
		Messages: []geminiMessage{
			{
				Role: "model",
				Parts: []geminiPart{
					{FunctionCall: &geminiFuncCall{
						Name: "run_skill",
						Args: json.RawMessage(`{"skill":"code-review"}`),
					}},
				},
			},
		},
	}
	blob, _ := json.Marshal(session)
	path := filepath.Join(dir, "chat.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"code-review": true}
	parser := &GeminiParser{}
	hits, offset, err := parser.ParseFile(path, 0, known)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if offset == 0 {
		t.Error("offset should be > 0")
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].SkillDirName != "code-review" {
		t.Errorf("SkillDirName = %q, want code-review", hits[0].SkillDirName)
	}
	if hits[0].Kind != eventlog.EventInvoke {
		t.Errorf("Kind = %q, want invoke", hits[0].Kind)
	}
}

func TestGeminiParser_ArrayFormat(t *testing.T) {
	dir := t.TempDir()
	messages := []geminiMessage{
		{
			Role: "model",
			Calls: []geminiFuncCall{
				{Name: "code-review", Args: json.RawMessage(`{}`)},
			},
		},
	}
	blob, _ := json.Marshal(messages)
	path := filepath.Join(dir, "chat.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"code-review": true}
	parser := &GeminiParser{}
	hits, _, err := parser.ParseFile(path, 0, known)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
}

func TestGeminiParser_MutualExclusion(t *testing.T) {
	// Session format with messages should NOT also parse as array
	dir := t.TempDir()
	session := geminiSession{
		Messages: []geminiMessage{
			{
				Role: "model",
				Parts: []geminiPart{
					{FunctionCall: &geminiFuncCall{
						Name: "skill-a",
						Args: json.RawMessage(`{}`),
					}},
				},
			},
		},
	}
	blob, _ := json.Marshal(session)
	path := filepath.Join(dir, "chat.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"skill-a": true}
	parser := &GeminiParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	// Should get exactly 1 hit, not 2 (which would happen with dual-parse)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit (mutual exclusion), got %d", len(hits))
	}
}

func TestGeminiParser_NoMatch(t *testing.T) {
	dir := t.TempDir()
	session := geminiSession{
		Messages: []geminiMessage{
			{
				Role: "model",
				Parts: []geminiPart{
					{FunctionCall: &geminiFuncCall{
						Name: "unrelated_func",
						Args: json.RawMessage(`{"foo":"bar"}`),
					}},
				},
			},
		},
	}
	blob, _ := json.Marshal(session)
	path := filepath.Join(dir, "chat.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"code-review": true}
	parser := &GeminiParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits, got %d", len(hits))
	}
}

func TestGeminiParser_Agent(t *testing.T) {
	parser := &GeminiParser{}
	if parser.Agent() != "gemini" {
		t.Errorf("Agent() = %q, want gemini", parser.Agent())
	}
}

func TestGeminiParser_SessionGlobs(t *testing.T) {
	parser := &GeminiParser{}
	globs := parser.SessionGlobs("/home/user")
	if len(globs) != 1 {
		t.Fatalf("expected 1 glob, got %d", len(globs))
	}
}
