package observation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillpm/internal/memory/eventlog"
)

// ClaudeParser parses Claude Code JSONL session transcripts.
type ClaudeParser struct{}

func (p *ClaudeParser) Agent() string { return "claude" }

func (p *ClaudeParser) SessionGlobs(home string) []string {
	return []string{
		filepath.Join(home, ".claude", "projects", "*", "*.jsonl"),
		filepath.Join(home, ".claude", "projects", "*", "sessions", "*.jsonl"),
	}
}

// claudeLine represents a single line in Claude Code JSONL.
type claudeLine struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	Timestamp string          `json:"timestamp"`
	Message   json.RawMessage `json:"message"`
}

type claudeMessage struct {
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

type claudeContentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

func (p *ClaudeParser) ParseFile(path string, offset int64, knownSkills map[string]bool) ([]SessionHit, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()

	// Check file size; if smaller than offset, file was truncated
	info, err := f.Stat()
	if err != nil {
		return nil, offset, err
	}
	if info.Size() < offset {
		offset = 0
	}

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, offset, err
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) // 1MB max line

	var hits []SessionHit
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Fast pre-filter: only lines with "tool_use" are worth parsing
		if !bytes.Contains(line, []byte(`"tool_use"`)) {
			continue
		}

		var entry claudeLine
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		if entry.Type != "assistant" {
			continue
		}

		var msg claudeMessage
		if json.Unmarshal(entry.Message, &msg) != nil {
			continue
		}

		ts := parseTimestamp(entry.Timestamp)
		sessionID := entry.SessionID
		if sessionID == "" {
			sessionID = filepath.Base(path)
		}

		for _, rawBlock := range msg.Content {
			var block claudeContentBlock
			if json.Unmarshal(rawBlock, &block) != nil {
				continue
			}
			if block.Type != "tool_use" {
				continue
			}

			switch block.Name {
			case "Skill":
				skillName := extractSkillInput(block.Input)
				if skillName != "" && knownSkills[skillName] {
					hits = append(hits, SessionHit{
						SkillDirName: skillName,
						Agent:        "claude",
						Kind:         eventlog.EventInvoke,
						Timestamp:    ts,
						SessionID:    sessionID,
						Fields:       map[string]string{"method": "session_parse", "tool": "Skill"},
					})
				}
			case "Read":
				filePath := extractReadPath(block.Input)
				skillDir := matchSkillPath(filePath, knownSkills)
				if skillDir != "" {
					hits = append(hits, SessionHit{
						SkillDirName: skillDir,
						Agent:        "claude",
						Kind:         eventlog.EventAccess,
						Timestamp:    ts,
						SessionID:    sessionID,
						Fields:       map[string]string{"method": "session_parse", "tool": "Read"},
					})
				}
			}
		}
	}

	newOffset, _ := f.Seek(0, io.SeekCurrent)
	return hits, newOffset, scanner.Err()
}

// extractSkillInput extracts the skill name from a Skill tool_use input.
func extractSkillInput(raw json.RawMessage) string {
	var input struct {
		Skill string `json:"skill"`
	}
	if json.Unmarshal(raw, &input) != nil {
		return ""
	}
	return input.Skill
}

// extractReadPath extracts the file_path from a Read tool_use input.
func extractReadPath(raw json.RawMessage) string {
	var input struct {
		FilePath string `json:"file_path"`
	}
	if json.Unmarshal(raw, &input) != nil {
		return ""
	}
	return input.FilePath
}

// matchSkillPath checks if a file path references a known skill directory.
// Matches patterns like */skills/<skill-dir>/SKILL.md
func matchSkillPath(filePath string, knownSkills map[string]bool) string {
	if filePath == "" {
		return ""
	}
	// Look for /skills/<skill-name>/ in the path
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for i, part := range parts {
		if part == "skills" && i+1 < len(parts) {
			candidate := parts[i+1]
			if knownSkills[candidate] {
				return candidate
			}
		}
	}
	return ""
}

func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	// Try common formats
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now().UTC()
}
