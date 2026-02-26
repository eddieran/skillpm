package eventlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventKind classifies usage events.
type EventKind string

const (
	EventAccess   EventKind = "access"
	EventInvoke   EventKind = "invoke"
	EventComplete EventKind = "complete"
	EventError    EventKind = "error"
	EventFeedback EventKind = "feedback"
)

// EventContext captures the environment when an event occurred.
type EventContext struct {
	ProjectRoot string `json:"project_root,omitempty"`
	ProjectType string `json:"project_type,omitempty"`
	TaskType    string `json:"task_type,omitempty"`
}

// UsageEvent records a single skill usage observation.
type UsageEvent struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	SkillRef  string            `json:"skill_ref"`
	Agent     string            `json:"agent"`
	Kind      EventKind         `json:"kind"`
	Scope     string            `json:"scope"`
	Context   EventContext      `json:"context,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// QueryFilter controls which events are returned by Query.
type QueryFilter struct {
	Since    time.Time
	SkillRef string
	Agent    string
	Kind     EventKind
	Limit    int
}

// SkillStats aggregates per-skill usage data.
type SkillStats struct {
	SkillRef   string
	EventCount int
	LastAccess time.Time
	Agents     []string
}

// EventLog is an append-only JSONL event store with query capabilities.
type EventLog struct {
	path string
	mu   sync.Mutex
}

// New creates an EventLog at the given file path.
func New(path string) *EventLog {
	return &EventLog{path: path}
}

// Append writes one or more events to the log file.
func (l *EventLog) Append(events ...UsageEvent) error {
	if l == nil || l.path == "" || len(events) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("MEM_EVENTLOG_APPEND: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("MEM_EVENTLOG_OPEN: %w", err)
	}
	defer f.Close()
	for i := range events {
		if events[i].Timestamp.IsZero() {
			events[i].Timestamp = time.Now().UTC()
		}
		blob, err := json.Marshal(events[i])
		if err != nil {
			return fmt.Errorf("MEM_EVENTLOG_APPEND: %w", err)
		}
		if _, err := f.Write(append(blob, '\n')); err != nil {
			return fmt.Errorf("MEM_EVENTLOG_APPEND: %w", err)
		}
	}
	return nil
}

// Query reads events matching the filter criteria.
func (l *EventLog) Query(f QueryFilter) ([]UsageEvent, error) {
	if l == nil || l.path == "" {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("MEM_EVENTLOG_QUERY: %w", err)
	}
	defer file.Close()

	var results []UsageEvent
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev UsageEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed lines
		}
		if !f.Since.IsZero() && ev.Timestamp.Before(f.Since) {
			continue
		}
		if f.SkillRef != "" && ev.SkillRef != f.SkillRef {
			continue
		}
		if f.Agent != "" && ev.Agent != f.Agent {
			continue
		}
		if f.Kind != "" && ev.Kind != f.Kind {
			continue
		}
		results = append(results, ev)
		if f.Limit > 0 && len(results) >= f.Limit {
			break
		}
	}
	return results, nil
}

// Stats computes per-skill usage statistics since a given time.
func (l *EventLog) Stats(since time.Time) ([]SkillStats, error) {
	events, err := l.Query(QueryFilter{Since: since})
	if err != nil {
		return nil, err
	}
	type acc struct {
		count  int
		last   time.Time
		agents map[string]struct{}
	}
	m := map[string]*acc{}
	for _, ev := range events {
		a, ok := m[ev.SkillRef]
		if !ok {
			a = &acc{agents: map[string]struct{}{}}
			m[ev.SkillRef] = a
		}
		a.count++
		if ev.Timestamp.After(a.last) {
			a.last = ev.Timestamp
		}
		if ev.Agent != "" {
			a.agents[ev.Agent] = struct{}{}
		}
	}
	stats := make([]SkillStats, 0, len(m))
	for ref, a := range m {
		agents := make([]string, 0, len(a.agents))
		for ag := range a.agents {
			agents = append(agents, ag)
		}
		stats = append(stats, SkillStats{
			SkillRef:   ref,
			EventCount: a.count,
			LastAccess: a.last,
			Agents:     agents,
		})
	}
	return stats, nil
}

// Truncate removes events older than the given time and returns the count removed.
func (l *EventLog) Truncate(before time.Time) (int, error) {
	if l == nil || l.path == "" {
		return 0, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	var kept [][]byte
	removed := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev UsageEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			kept = append(kept, append([]byte{}, line...))
			continue
		}
		if ev.Timestamp.Before(before) {
			removed++
			continue
		}
		kept = append(kept, append([]byte{}, line...))
	}
	if removed == 0 {
		return 0, nil
	}
	tmp := l.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	for _, line := range kept {
		f.Write(append(line, '\n'))
	}
	f.Close()
	return removed, os.Rename(tmp, l.path)
}
