package observation

import (
	"skillpm/internal/memory/eventlog"
	"time"
)

// SessionHit represents a detected skill usage in a session transcript.
type SessionHit struct {
	SkillDirName string             // directory name, e.g. "code-review"
	Agent        string             // e.g. "claude"
	Kind         eventlog.EventKind // EventInvoke | EventAccess
	Timestamp    time.Time          // from session entry, not wall clock
	SessionID    string             // session identifier for dedup
	Fields       map[string]string  // additional context
}

// SessionParser parses an agent's session files to detect skill usage.
type SessionParser interface {
	// Agent returns the agent name this parser handles.
	Agent() string

	// SessionGlobs returns glob patterns for session files (absolute paths).
	SessionGlobs(home string) []string

	// ParseFile parses a session file from the given offset.
	// Returns hits, new offset, and any error.
	// knownSkills is the set of installed skill directory names.
	ParseFile(path string, offset int64, knownSkills map[string]bool) ([]SessionHit, int64, error)
}

// SkillsDirScanner is used for agents without observable session files.
type SkillsDirScanner interface {
	ScanSkillsDir(skillsDir string, lastScan time.Time, knownSkills map[string]bool) []SessionHit
}
