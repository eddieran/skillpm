package observation

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillpm/internal/memory/eventlog"
)

// MtimeScanner provides mtime-based fallback observation for agents
// without observable session transcripts (Cursor, Copilot, TRAE, Kiro).
type MtimeScanner struct{}

func (s *MtimeScanner) ScanSkillsDir(skillsDir string, lastScan time.Time, knownSkills map[string]bool) []SessionHit {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}

	now := time.Now().UTC()
	var hits []SessionHit

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillMD := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		info, err := os.Stat(skillMD)
		if err != nil {
			continue
		}
		if !lastScan.IsZero() && !info.ModTime().After(lastScan) {
			continue
		}
		dirName := entry.Name()
		if len(knownSkills) > 0 && !knownSkills[dirName] {
			continue
		}
		hits = append(hits, SessionHit{
			SkillDirName: dirName,
			Agent:        "",                    // set by caller
			Kind:         eventlog.EventAccess,
			Timestamp:    now,
			SessionID:    fmt.Sprintf("mtime-%d", now.UnixNano()),
			Fields:       map[string]string{"method": "mtime"},
		})
	}
	return hits
}
