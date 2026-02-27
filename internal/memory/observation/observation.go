package observation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillpm/internal/memory/eventlog"
)

// mtimeEntry tracks an agent using mtime fallback observation.
type mtimeEntry struct {
	agent     string
	skillsDir string
	scanner   SkillsDirScanner
}

// Observer orchestrates skill usage observation from session transcripts
// and mtime fallback for unobservable agents.
type Observer struct {
	eventLog      *eventlog.EventLog
	scanStatePath string
	parsers       []SessionParser
	mtimeFallback []mtimeEntry
	skillIndex    *SkillIndex
}

// New creates an Observer with session transcript parsing support.
// skillRefs are the currently installed skill refs (e.g. "source/skill-name").
// agentDirs maps agent names to their skills directories (for mtime fallback).
func New(el *eventlog.EventLog, agentDirs map[string]string, scanStatePath string, skillRefs ...string) *Observer {
	idx := NewSkillIndex(skillRefs)

	o := &Observer{
		eventLog:      el,
		scanStatePath: scanStatePath,
		skillIndex:    idx,
	}

	// Register session parsers for agents with observable transcripts
	o.parsers = []SessionParser{
		&ClaudeParser{},
		&CodexParser{},
		&GeminiParser{},
		&OpenCodeParser{},
	}

	// Register mtime fallback for ALL agents.
	// Session parsers provide high-confidence events; mtime provides
	// low-confidence fallback when session files don't exist.
	// Dedup handles any overlaps.
	mtimeScanner := &MtimeScanner{}
	for agent, dir := range agentDirs {
		o.mtimeFallback = append(o.mtimeFallback, mtimeEntry{
			agent:     agent,
			skillsDir: dir,
			scanner:   mtimeScanner,
		})
	}

	return o
}

// ScanAll runs both session transcript parsing and mtime fallback,
// writing discovered events to the eventlog.
func (o *Observer) ScanAll() ([]eventlog.UsageEvent, error) {
	if o == nil {
		return nil, nil
	}

	state := loadScanState(o.scanStatePath)
	home, _ := os.UserHomeDir()
	knownSkills := o.skillIndex.KnownDirNames()

	var allHits []SessionHit

	// 1. Session transcript parsing
	for _, parser := range o.parsers {
		hits := o.scanParser(parser, home, knownSkills, &state)
		allHits = append(allHits, hits...)
	}

	// 2. Mtime fallback
	now := time.Now().UTC()
	for _, entry := range o.mtimeFallback {
		lastScan := state.MtimeAgents[entry.agent]
		hits := entry.scanner.ScanSkillsDir(entry.skillsDir, lastScan, knownSkills)
		for i := range hits {
			hits[i].Agent = entry.agent
		}
		allHits = append(allHits, hits...)
		state.MtimeAgents[entry.agent] = now
	}

	// 3. Dedup by SessionID + SkillDirName + Kind
	allHits = dedupHits(allHits)

	// 4. Convert to UsageEvents
	events := make([]eventlog.UsageEvent, 0, len(allHits))
	for _, hit := range allHits {
		ref := o.skillIndex.Resolve(hit.SkillDirName)
		events = append(events, eventlog.UsageEvent{
			ID:        generateEventID(hit),
			Timestamp: hit.Timestamp,
			SkillRef:  ref,
			Agent:     hit.Agent,
			Kind:      hit.Kind,
			Scope:     "global",
			Fields:    hit.Fields,
		})
	}

	// 5. Write to eventlog
	if len(events) > 0 {
		if err := o.eventLog.Append(events...); err != nil {
			return events, fmt.Errorf("MEM_OBSERVE_SCAN: %w", err)
		}
	}

	// 6. GC old entries
	state.GC(60 * 24 * time.Hour) // 60 days

	// 7. Persist state
	saveScanState(o.scanStatePath, state)

	return events, nil
}

// ScanAgent provides backward-compatible single-agent mtime scanning.
// This is used by callers that haven't migrated to the new ScanAll flow.
func (o *Observer) ScanAgent(agent, skillsDir string, lastScan time.Time) []eventlog.UsageEvent {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	now := time.Now().UTC()
	var events []eventlog.UsageEvent
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
		events = append(events, eventlog.UsageEvent{
			ID:        fmt.Sprintf("%d-%s-%s", now.UnixNano(), agent, entry.Name()),
			Timestamp: now,
			SkillRef:  entry.Name(),
			Agent:     agent,
			Kind:      eventlog.EventAccess,
			Scope:     "global",
			Fields:    map[string]string{"method": "mtime"},
		})
	}
	return events
}

// scanParser runs a single session parser across all matching files.
func (o *Observer) scanParser(parser SessionParser, home string, knownSkills map[string]bool, state *ScanState) []SessionHit {
	globs := parser.SessionGlobs(home)
	if len(globs) == 0 {
		return nil
	}

	// Expand globs and collect files
	type sessionFile struct {
		path    string
		size    int64
		modTime time.Time
	}
	var files []sessionFile
	cutoff := time.Now().Add(-30 * 24 * time.Hour)

	for _, pattern := range globs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue
			}
			if info.ModTime().Before(cutoff) {
				continue
			}
			files = append(files, sessionFile{
				path:    match,
				size:    info.Size(),
				modTime: info.ModTime(),
			})
		}
	}

	// Sort by modtime descending, limit to 500
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	if len(files) > 500 {
		files = files[:500]
	}

	var allHits []SessionHit
	now := time.Now().UTC()

	for _, sf := range files {
		progress := state.Files[sf.path]

		// Determine if file needs parsing
		needsParse := false
		if strings.HasSuffix(sf.path, ".jsonl") {
			// JSONL: incremental by byte offset
			needsParse = sf.size > progress.Offset
		} else {
			// JSON: re-parse on mtime change
			needsParse = sf.modTime.After(progress.ModTime)
		}
		if !needsParse {
			continue
		}

		hits, newOffset, err := parser.ParseFile(sf.path, progress.Offset, knownSkills)
		if err != nil {
			continue // graceful skip
		}

		allHits = append(allHits, hits...)
		state.Files[sf.path] = FileProgress{
			Offset:   newOffset,
			ModTime:  sf.modTime,
			ParsedAt: now,
			Agent:    parser.Agent(),
		}
	}

	return allHits
}

// dedupHits removes duplicate hits based on SessionID + SkillDirName + Kind.
func dedupHits(hits []SessionHit) []SessionHit {
	type key struct {
		sessionID string
		skill     string
		kind      eventlog.EventKind
	}
	seen := make(map[key]bool, len(hits))
	result := make([]SessionHit, 0, len(hits))
	for _, h := range hits {
		k := key{h.SessionID, h.SkillDirName, h.Kind}
		if seen[k] {
			continue
		}
		seen[k] = true
		result = append(result, h)
	}
	return result
}

func generateEventID(hit SessionHit) string {
	return fmt.Sprintf("%d-%s-%s", hit.Timestamp.UnixNano(), hit.Agent, hit.SkillDirName)
}
