package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/memory/eventlog"
)

func TestOpenCodeParser_ToolCall(t *testing.T) {
	dir := t.TempDir()
	msg := openCodeMessage{
		ID:        "msg_001",
		SessionID: "sess-oc-1",
		Role:      "assistant",
		ToolCalls: []openCodeTool{
			{Name: "run_skill", Input: json.RawMessage(`{"skill":"code-review"}`)},
		},
	}
	blob, _ := json.Marshal(msg)
	path := filepath.Join(dir, "msg_001.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"code-review": true}
	parser := &OpenCodeParser{}
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

func TestOpenCodeParser_SkipsUserMessages(t *testing.T) {
	dir := t.TempDir()
	msg := openCodeMessage{
		ID:        "msg_002",
		SessionID: "sess-oc-2",
		Role:      "user",
		ToolCalls: []openCodeTool{
			{Name: "run_skill", Input: json.RawMessage(`{"skill":"code-review"}`)},
		},
	}
	blob, _ := json.Marshal(msg)
	path := filepath.Join(dir, "msg_002.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"code-review": true}
	parser := &OpenCodeParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits for user role, got %d", len(hits))
	}
}

func TestOpenCodeParser_UnknownSkillSkipped(t *testing.T) {
	dir := t.TempDir()
	msg := openCodeMessage{
		ID:        "msg_003",
		SessionID: "sess-oc-3",
		Role:      "assistant",
		ToolCalls: []openCodeTool{
			{Name: "run_skill", Input: json.RawMessage(`{"skill":"unknown"}`)},
		},
	}
	blob, _ := json.Marshal(msg)
	path := filepath.Join(dir, "msg_003.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"code-review": true}
	parser := &OpenCodeParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits for unknown skill, got %d", len(hits))
	}
}

func TestOpenCodeParser_FallbackSessionID(t *testing.T) {
	dir := t.TempDir()
	msg := openCodeMessage{
		ID:   "msg_004",
		Role: "assistant",
		// SessionID is empty — should fall back to dir name
		ToolCalls: []openCodeTool{
			{Name: "code-review", Input: json.RawMessage(`{}`)},
		},
	}
	blob, _ := json.Marshal(msg)
	path := filepath.Join(dir, "msg_004.json")
	os.WriteFile(path, blob, 0o644)

	known := map[string]bool{"code-review": true}
	parser := &OpenCodeParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].SessionID == "" {
		t.Error("SessionID should not be empty")
	}
}

func TestOpenCodeParser_Agent(t *testing.T) {
	parser := &OpenCodeParser{}
	if parser.Agent() != "opencode" {
		t.Errorf("Agent() = %q, want opencode", parser.Agent())
	}
}
