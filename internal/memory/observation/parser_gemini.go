package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillpm/internal/memory/eventlog"
)

// GeminiParser parses Gemini CLI JSON session files.
type GeminiParser struct{}

func (p *GeminiParser) Agent() string { return "gemini" }

func (p *GeminiParser) SessionGlobs(home string) []string {
	return []string{
		filepath.Join(home, ".gemini", "tmp", "*", "chats", "*.json"),
	}
}

type geminiSession struct {
	Messages []geminiMessage `json:"messages"`
}

type geminiMessage struct {
	Role    string             `json:"role"`
	Content json.RawMessage    `json:"content"`
	Parts   []geminiPart       `json:"parts"`
	Calls   []geminiFuncCall   `json:"functionCalls"`
}

type geminiPart struct {
	FunctionCall *geminiFuncCall `json:"functionCall,omitempty"`
}

type geminiFuncCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// ParseFile parses a Gemini JSON file. Since these are full JSON (not JSONL),
// we re-parse the entire file when mtime changes (offset is not used for seeking).
func (p *GeminiParser) ParseFile(path string, _ int64, knownSkills map[string]bool) ([]SessionHit, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}

	sessionID := filepath.Base(path)
	var hits []SessionHit

	// Try parsing as session object
	var session geminiSession
	if json.Unmarshal(data, &session) == nil && len(session.Messages) > 0 {
		for _, msg := range session.Messages {
			hits = append(hits, extractGeminiHits(msg, sessionID, knownSkills)...)
		}
	}

	// Try parsing as array of messages
	var messages []geminiMessage
	if json.Unmarshal(data, &messages) == nil {
		for _, msg := range messages {
			hits = append(hits, extractGeminiHits(msg, sessionID, knownSkills)...)
		}
	}

	return hits, info.Size(), nil
}

func extractGeminiHits(msg geminiMessage, sessionID string, knownSkills map[string]bool) []SessionHit {
	var hits []SessionHit

	// Check function calls in parts
	for _, part := range msg.Parts {
		if part.FunctionCall != nil {
			if hit := matchGeminiFuncCall(*part.FunctionCall, sessionID, knownSkills); hit != nil {
				hits = append(hits, *hit)
			}
		}
	}

	// Check top-level function calls
	for _, fc := range msg.Calls {
		if hit := matchGeminiFuncCall(fc, sessionID, knownSkills); hit != nil {
			hits = append(hits, *hit)
		}
	}

	return hits
}

func matchGeminiFuncCall(fc geminiFuncCall, sessionID string, knownSkills map[string]bool) *SessionHit {
	argsStr := string(fc.Args)
	for skillDir := range knownSkills {
		if strings.Contains(fc.Name, skillDir) || strings.Contains(argsStr, skillDir) {
			return &SessionHit{
				SkillDirName: skillDir,
				Agent:        "gemini",
				Kind:         eventlog.EventInvoke,
				Timestamp:    time.Now().UTC(),
				SessionID:    sessionID,
				Fields:       map[string]string{"method": "session_parse"},
			}
		}
	}
	return nil
}
