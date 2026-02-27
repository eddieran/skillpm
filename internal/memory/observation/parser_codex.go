package observation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"skillpm/internal/memory/eventlog"
)

// CodexParser parses Codex CLI JSONL session transcripts.
type CodexParser struct{}

func (p *CodexParser) Agent() string { return "codex" }

func (p *CodexParser) SessionGlobs(home string) []string {
	return []string{
		filepath.Join(home, ".codex", "sessions", "*", "*", "*", "rollout-*.jsonl"),
	}
}

type codexLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (p *CodexParser) ParseFile(path string, offset int64, knownSkills map[string]bool) ([]SessionHit, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()

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
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	sessionID := filepath.Base(path)
	var hits []SessionHit

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if !bytes.Contains(line, []byte(`"function_call"`)) {
			continue
		}

		var entry codexLine
		if json.Unmarshal(line, &entry) != nil {
			continue
		}

		var payload codexPayload
		if json.Unmarshal(entry.Payload, &payload) != nil {
			continue
		}
		if payload.Type != "function_call" {
			continue
		}

		ts := parseTimestamp(entry.Timestamp)

		// Check if arguments reference a known skill
		for skillDir := range knownSkills {
			if strings.Contains(payload.Arguments, skillDir) || strings.Contains(payload.Name, skillDir) {
				hits = append(hits, SessionHit{
					SkillDirName: skillDir,
					Agent:        "codex",
					Kind:         eventlog.EventAccess,
					Timestamp:    ts,
					SessionID:    sessionID,
					Fields:       map[string]string{"method": "session_parse", "tool": "function_call"},
				})
				break
			}
		}
	}

	newOffset, _ := f.Seek(0, io.SeekCurrent)
	return hits, newOffset, scanner.Err()
}
