package observation

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
	"skillpm/internal/memory/eventlog"
)

// LastScanState tracks per-agent last scan timestamps.
type LastScanState struct {
	Agents map[string]time.Time `toml:"agents"`
}

// Observer scans agent skill directories for usage via mtime.
type Observer struct {
	agentDirs    map[string]string
	eventLog     *eventlog.EventLog
	lastScanPath string
}

// New creates an Observer.
func New(el *eventlog.EventLog, agentDirs map[string]string, lastScanPath string) *Observer {
	return &Observer{agentDirs: agentDirs, eventLog: el, lastScanPath: lastScanPath}
}

// ScanAll scans all known agent skill directories and records events.
func (o *Observer) ScanAll() ([]eventlog.UsageEvent, error) {
	if o == nil {
		return nil, nil
	}
	state := o.loadLastScan()
	var allEvents []eventlog.UsageEvent

	for agent, dir := range o.agentDirs {
		lastScan := state.Agents[agent]
		events := o.ScanAgent(agent, dir, lastScan)
		allEvents = append(allEvents, events...)
		state.Agents[agent] = time.Now().UTC()
	}

	if len(allEvents) > 0 {
		if err := o.eventLog.Append(allEvents...); err != nil {
			return allEvents, fmt.Errorf("MEM_OBSERVE_SCAN: %w", err)
		}
	}
	o.saveLastScan(state)
	return allEvents, nil
}

// ScanAgent scans a single agent's skills directory.
func (o *Observer) ScanAgent(agent, skillsDir string, lastScan time.Time) []eventlog.UsageEvent {
	var events []eventlog.UsageEvent
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	now := time.Now().UTC()
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

func (o *Observer) loadLastScan() LastScanState {
	state := LastScanState{Agents: map[string]time.Time{}}
	if o.lastScanPath == "" {
		return state
	}
	blob, err := os.ReadFile(o.lastScanPath)
	if err != nil {
		return state
	}
	_ = toml.Unmarshal(blob, &state)
	if state.Agents == nil {
		state.Agents = map[string]time.Time{}
	}
	return state
}

func (o *Observer) saveLastScan(state LastScanState) {
	if o.lastScanPath == "" {
		return
	}
	blob, err := toml.Marshal(state)
	if err != nil {
		return
	}
	tmp := o.lastScanPath + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, o.lastScanPath)
}
