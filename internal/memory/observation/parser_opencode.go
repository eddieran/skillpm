package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillpm/internal/memory/eventlog"
)

// OpenCodeParser parses OpenCode's individual JSON message files.
type OpenCodeParser struct{}

func (p *OpenCodeParser) Agent() string { return "opencode" }

func (p *OpenCodeParser) SessionGlobs(home string) []string {
	return []string{
		filepath.Join(home, ".local", "share", "opencode", "storage", "message", "*", "msg_*.json"),
	}
}

type openCodeMessage struct {
	ID        string           `json:"id"`
	SessionID string           `json:"sessionId"`
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openCodeTool   `json:"toolCalls"`
}

type openCodeTool struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ParseFile parses a single OpenCode message JSON file.
// offset is not used (each file is independent).
func (p *OpenCodeParser) ParseFile(path string, _ int64, knownSkills map[string]bool) ([]SessionHit, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}

	var msg openCodeMessage
	if json.Unmarshal(data, &msg) != nil {
		return nil, info.Size(), nil
	}

	if msg.Role != "assistant" || len(msg.ToolCalls) == 0 {
		return nil, info.Size(), nil
	}

	sessionID := msg.SessionID
	if sessionID == "" {
		sessionID = filepath.Base(filepath.Dir(path))
	}

	var hits []SessionHit
	for _, tc := range msg.ToolCalls {
		inputStr := string(tc.Input)
		for skillDir := range knownSkills {
			if strings.Contains(tc.Name, skillDir) || strings.Contains(inputStr, skillDir) {
				hits = append(hits, SessionHit{
					SkillDirName: skillDir,
					Agent:        "opencode",
					Kind:         eventlog.EventInvoke,
					Timestamp:    time.Now().UTC(),
					SessionID:    sessionID,
					Fields:       map[string]string{"method": "session_parse"},
				})
				break
			}
		}
	}

	return hits, info.Size(), nil
}
