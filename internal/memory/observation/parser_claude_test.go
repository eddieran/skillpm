package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/memory/eventlog"
)

func writeJSONL(t *testing.T, dir, name string, lines []interface{}) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create JSONL: %v", err)
	}
	defer f.Close()
	for _, line := range lines {
		blob, _ := json.Marshal(line)
		f.Write(blob)
		f.Write([]byte("\n"))
	}
	return path
}

func TestClaudeParser_SkillInvoke(t *testing.T) {
	dir := t.TempDir()
	lines := []interface{}{
		map[string]interface{}{
			"type":      "assistant",
			"sessionId": "sess-001",
			"timestamp": "2026-02-26T10:00:00Z",
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []map[string]interface{}{
					{
						"type":  "tool_use",
						"name":  "Skill",
						"id":    "tu-1",
						"input": map[string]string{"skill": "code-review"},
					},
				},
			},
		},
	}
	path := writeJSONL(t, dir, "session.jsonl", lines)
	known := map[string]bool{"code-review": true}

	parser := &ClaudeParser{}
	hits, newOffset, err := parser.ParseFile(path, 0, known)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if newOffset == 0 {
		t.Error("newOffset should be > 0")
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
	if hits[0].Fields["tool"] != "Skill" {
		t.Errorf("Fields[tool] = %q, want Skill", hits[0].Fields["tool"])
	}
}

func TestClaudeParser_ReadAccess(t *testing.T) {
	dir := t.TempDir()
	lines := []interface{}{
		map[string]interface{}{
			"type":      "assistant",
			"sessionId": "sess-002",
			"timestamp": "2026-02-26T10:01:00Z",
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []map[string]interface{}{
					{
						"type":  "tool_use",
						"name":  "Read",
						"id":    "tu-2",
						"input": map[string]string{"file_path": "/home/user/.claude/skills/go-test-helper/SKILL.md"},
					},
				},
			},
		},
	}
	path := writeJSONL(t, dir, "session.jsonl", lines)
	known := map[string]bool{"go-test-helper": true}

	parser := &ClaudeParser{}
	hits, _, err := parser.ParseFile(path, 0, known)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].SkillDirName != "go-test-helper" {
		t.Errorf("SkillDirName = %q, want go-test-helper", hits[0].SkillDirName)
	}
	if hits[0].Kind != eventlog.EventAccess {
		t.Errorf("Kind = %q, want access", hits[0].Kind)
	}
}

func TestClaudeParser_SkipsUserMessages(t *testing.T) {
	dir := t.TempDir()
	lines := []interface{}{
		map[string]interface{}{
			"type":      "user",
			"sessionId": "sess-003",
			"timestamp": "2026-02-26T10:00:00Z",
			"message": map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": "please review the code"},
				},
			},
		},
	}
	path := writeJSONL(t, dir, "session.jsonl", lines)
	known := map[string]bool{"code-review": true}

	parser := &ClaudeParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits for user messages, got %d", len(hits))
	}
}

func TestClaudeParser_UnknownSkillSkipped(t *testing.T) {
	dir := t.TempDir()
	lines := []interface{}{
		map[string]interface{}{
			"type":      "assistant",
			"sessionId": "sess-004",
			"timestamp": "2026-02-26T10:00:00Z",
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []map[string]interface{}{
					{
						"type":  "tool_use",
						"name":  "Skill",
						"id":    "tu-3",
						"input": map[string]string{"skill": "unknown-skill"},
					},
				},
			},
		},
	}
	path := writeJSONL(t, dir, "session.jsonl", lines)
	known := map[string]bool{"code-review": true}

	parser := &ClaudeParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits for unknown skill, got %d", len(hits))
	}
}

func TestClaudeParser_IncrementalOffset(t *testing.T) {
	dir := t.TempDir()
	line1 := map[string]interface{}{
		"type": "assistant", "sessionId": "s1", "timestamp": "2026-02-26T10:00:00Z",
		"message": map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "tool_use", "name": "Skill", "id": "t1", "input": map[string]string{"skill": "skill-a"}},
			},
		},
	}
	path := writeJSONL(t, dir, "session.jsonl", []interface{}{line1})
	known := map[string]bool{"skill-a": true, "skill-b": true}

	parser := &ClaudeParser{}
	hits1, offset1, _ := parser.ParseFile(path, 0, known)
	if len(hits1) != 1 {
		t.Fatalf("first parse: expected 1 hit, got %d", len(hits1))
	}

	// Append second line
	line2 := map[string]interface{}{
		"type": "assistant", "sessionId": "s1", "timestamp": "2026-02-26T10:01:00Z",
		"message": map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "tool_use", "name": "Skill", "id": "t2", "input": map[string]string{"skill": "skill-b"}},
			},
		},
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	blob, _ := json.Marshal(line2)
	f.Write(blob)
	f.Write([]byte("\n"))
	f.Close()

	// Parse from offset — should only get skill-b
	hits2, _, _ := parser.ParseFile(path, offset1, known)
	if len(hits2) != 1 {
		t.Fatalf("incremental parse: expected 1 hit, got %d", len(hits2))
	}
	if hits2[0].SkillDirName != "skill-b" {
		t.Errorf("incremental hit = %q, want skill-b", hits2[0].SkillDirName)
	}
}

func TestClaudeParser_MultipleToolUses(t *testing.T) {
	dir := t.TempDir()
	lines := []interface{}{
		map[string]interface{}{
			"type": "assistant", "sessionId": "s1", "timestamp": "2026-02-26T10:00:00Z",
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []map[string]interface{}{
					{"type": "tool_use", "name": "Skill", "id": "t1", "input": map[string]string{"skill": "a"}},
					{"type": "text", "text": "doing stuff"},
					{"type": "tool_use", "name": "Read", "id": "t2", "input": map[string]string{"file_path": "/x/skills/b/SKILL.md"}},
				},
			},
		},
	}
	path := writeJSONL(t, dir, "session.jsonl", lines)
	known := map[string]bool{"a": true, "b": true}

	parser := &ClaudeParser{}
	hits, _, _ := parser.ParseFile(path, 0, known)
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
}

func TestMatchSkillPath(t *testing.T) {
	known := map[string]bool{"code-review": true, "test-helper": true}
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/.claude/skills/code-review/SKILL.md", "code-review"},
		{"/home/user/.claude/skills/test-helper/scripts/run.sh", "test-helper"},
		{"/home/user/.claude/skills/unknown/SKILL.md", ""},
		{"", ""},
		{"/some/other/path", ""},
	}
	for _, tt := range tests {
		got := matchSkillPath(tt.path, known)
		if got != tt.want {
			t.Errorf("matchSkillPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	ts := parseTimestamp("2026-02-26T10:00:00Z")
	if ts.Year() != 2026 || ts.Month() != 2 || ts.Day() != 26 {
		t.Errorf("parseTimestamp gave %v", ts)
	}

	// Empty string should return now-ish time
	ts2 := parseTimestamp("")
	if ts2.IsZero() {
		t.Error("empty timestamp should not be zero")
	}
}

func TestClaudeParser_SessionGlobs(t *testing.T) {
	parser := &ClaudeParser{}
	globs := parser.SessionGlobs("/home/user")
	if len(globs) != 2 {
		t.Fatalf("expected 2 globs, got %d", len(globs))
	}
}
