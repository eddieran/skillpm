package observation

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillpm/internal/memory/eventlog"
)

// makeSkillDir creates a subdirectory inside skillsDir named skillName and
// writes a SKILL.md file into it. It returns the path to SKILL.md.
func makeSkillDir(t *testing.T, skillsDir, skillName string) string {
	t.Helper()
	dir := filepath.Join(skillsDir, skillName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("makeSkillDir MkdirAll: %v", err)
	}
	mdPath := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(mdPath, []byte("# "+skillName), 0o644); err != nil {
		t.Fatalf("makeSkillDir WriteFile SKILL.md: %v", err)
	}
	return mdPath
}

// countEventsInLog reads the JSONL eventlog file and returns all decoded events.
func countEventsInLog(t *testing.T, logPath string) []eventlog.UsageEvent {
	t.Helper()
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("countEventsInLog Open: %v", err)
	}
	defer f.Close()
	var events []eventlog.UsageEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev eventlog.UsageEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("countEventsInLog Unmarshal: %v", err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("countEventsInLog scanner: %v", err)
	}
	return events
}

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

// TestScanAllNilObserver verifies that calling ScanAll on a nil *Observer
// returns (nil, nil) without panicking.
func TestScanAllNilObserver(t *testing.T) {
	var o *Observer
	events, err := o.ScanAll()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events, got: %v", events)
	}
}

// TestScanAgentEmptyDir verifies that scanning an empty skills directory
// produces no events.
func TestScanAgentEmptyDir(t *testing.T) {
	skillsDir := t.TempDir()
	el := eventlog.New(filepath.Join(t.TempDir(), "events.jsonl"))
	o := New(el, map[string]string{"agent1": skillsDir}, "")

	events := o.ScanAgent("agent1", skillsDir, time.Time{})
	if len(events) != 0 {
		t.Fatalf("expected 0 events for empty dir, got %d", len(events))
	}
}

// TestScanAgentDetectsNewSkills verifies that two skill subdirectories each
// containing a SKILL.md file are detected when lastScan is the zero time.
func TestScanAgentDetectsNewSkills(t *testing.T) {
	skillsDir := t.TempDir()
	makeSkillDir(t, skillsDir, "skillA")
	makeSkillDir(t, skillsDir, "skillB")

	el := eventlog.New(filepath.Join(t.TempDir(), "events.jsonl"))
	o := New(el, map[string]string{"agentX": skillsDir}, "")

	events := o.ScanAgent("agentX", skillsDir, time.Time{})
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify structural correctness of each event.
	seen := map[string]bool{}
	for _, ev := range events {
		if ev.Agent != "agentX" {
			t.Errorf("event Agent = %q, want %q", ev.Agent, "agentX")
		}
		if ev.Kind != eventlog.EventAccess {
			t.Errorf("event Kind = %q, want %q", ev.Kind, eventlog.EventAccess)
		}
		if ev.Scope != "global" {
			t.Errorf("event Scope = %q, want global", ev.Scope)
		}
		if ev.Fields["method"] != "mtime" {
			t.Errorf("event Fields[method] = %q, want mtime", ev.Fields["method"])
		}
		if ev.ID == "" {
			t.Error("event ID must not be empty")
		}
		seen[ev.SkillRef] = true
	}
	if !seen["skillA"] {
		t.Error("expected event for skillA, not found")
	}
	if !seen["skillB"] {
		t.Error("expected event for skillB, not found")
	}
}

// TestScanAgentSkipsUnmodified verifies that when lastScan is set to a time
// in the future (after SKILL.md was written), no events are produced.
func TestScanAgentSkipsUnmodified(t *testing.T) {
	skillsDir := t.TempDir()
	makeSkillDir(t, skillsDir, "oldSkill")

	// Set lastScan to 1 minute in the future so mtime is never after lastScan.
	futureLastScan := time.Now().UTC().Add(time.Minute)

	el := eventlog.New(filepath.Join(t.TempDir(), "events.jsonl"))
	o := New(el, map[string]string{"agent1": skillsDir}, "")

	events := o.ScanAgent("agent1", skillsDir, futureLastScan)
	if len(events) != 0 {
		t.Fatalf("expected 0 events when SKILL.md older than lastScan, got %d", len(events))
	}
}

// TestScanAgentSkipsNonDir verifies that plain files sitting directly inside
// skillsDir are ignored (only directories are considered skill slots).
func TestScanAgentSkipsNonDir(t *testing.T) {
	skillsDir := t.TempDir()

	// Place a regular file at the top level of skillsDir.
	regularFile := filepath.Join(skillsDir, "not-a-skill.md")
	if err := os.WriteFile(regularFile, []byte("noise"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Also add one legitimate skill so we can confirm the regular file is the
	// only thing being skipped.
	makeSkillDir(t, skillsDir, "realSkill")

	el := eventlog.New(filepath.Join(t.TempDir(), "events.jsonl"))
	o := New(el, map[string]string{"agent1": skillsDir}, "")

	events := o.ScanAgent("agent1", skillsDir, time.Time{})
	if len(events) != 1 {
		t.Fatalf("expected 1 event (non-dir skipped), got %d", len(events))
	}
	if events[0].SkillRef != "realSkill" {
		t.Errorf("expected SkillRef = realSkill, got %q", events[0].SkillRef)
	}
}

// TestScanAllMultipleAgents verifies that ScanAll across two agents each
// owning one skill returns exactly 2 events and that the last_scan state file
// is written.
func TestScanAllMultipleAgents(t *testing.T) {
	tmp := t.TempDir()
	skillsDirA := filepath.Join(tmp, "agentA-skills")
	skillsDirB := filepath.Join(tmp, "agentB-skills")
	if err := os.MkdirAll(skillsDirA, 0o755); err != nil {
		t.Fatalf("MkdirAll skillsDirA: %v", err)
	}
	if err := os.MkdirAll(skillsDirB, 0o755); err != nil {
		t.Fatalf("MkdirAll skillsDirB: %v", err)
	}

	makeSkillDir(t, skillsDirA, "skill-alpha")
	makeSkillDir(t, skillsDirB, "skill-beta")

	logPath := filepath.Join(tmp, "events.jsonl")
	lastScanPath := filepath.Join(tmp, "last_scan.toml")

	el := eventlog.New(logPath)
	agentDirs := map[string]string{
		"agentA": skillsDirA,
		"agentB": skillsDirB,
	}
	o := New(el, agentDirs, lastScanPath)

	events, err := o.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events from 2 agents, got %d", len(events))
	}

	// Verify last_scan file was created.
	if _, err := os.Stat(lastScanPath); os.IsNotExist(err) {
		t.Fatal("expected last_scan.toml to be written, file not found")
	}
}

// TestScanAllAppendsToEventLog verifies that events returned by ScanAll are
// actually written to the backing eventlog JSONL file.
func TestScanAllAppendsToEventLog(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	makeSkillDir(t, skillsDir, "my-skill")

	logPath := filepath.Join(tmp, "events.jsonl")
	el := eventlog.New(logPath)
	o := New(el, map[string]string{"claudeAgent": skillsDir}, filepath.Join(tmp, "last_scan.toml"))

	returned, err := o.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll: %v", err)
	}
	if len(returned) == 0 {
		t.Fatal("expected at least 1 returned event")
	}

	// Read back from disk and compare count.
	written := countEventsInLog(t, logPath)
	if len(written) != len(returned) {
		t.Fatalf("eventlog has %d events, ScanAll returned %d; they must match", len(written), len(returned))
	}

	// Verify content of the written event.
	if written[0].Agent != "claudeAgent" {
		t.Errorf("written Agent = %q, want claudeAgent", written[0].Agent)
	}
	if written[0].SkillRef != "my-skill" {
		t.Errorf("written SkillRef = %q, want my-skill", written[0].SkillRef)
	}
	if written[0].Kind != eventlog.EventAccess {
		t.Errorf("written Kind = %q, want %q", written[0].Kind, eventlog.EventAccess)
	}
}

// TestScanAllPersistsLastScan verifies that after a first ScanAll the
// persisted last_scan.toml causes a second ScanAll (with unchanged files) to
// produce zero new events.
func TestScanAllPersistsLastScan(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	makeSkillDir(t, skillsDir, "stable-skill")

	logPath := filepath.Join(tmp, "events.jsonl")
	lastScanPath := filepath.Join(tmp, "last_scan.toml")
	el := eventlog.New(logPath)
	o := New(el, map[string]string{"botAgent": skillsDir}, lastScanPath)

	// First scan — should detect the skill.
	first, err := o.ScanAll()
	if err != nil {
		t.Fatalf("first ScanAll: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first scan: expected 1 event, got %d", len(first))
	}

	// Second scan — same files, last_scan.toml now records a timestamp after
	// the SKILL.md mtime, so 0 new events expected.
	second, err := o.ScanAll()
	if err != nil {
		t.Fatalf("second ScanAll: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second scan: expected 0 events (last_scan persisted), got %d", len(second))
	}

	// Confirm the eventlog still contains only the 1 event from the first scan.
	all := countEventsInLog(t, logPath)
	if len(all) != 1 {
		t.Fatalf("eventlog should contain exactly 1 event total, got %d", len(all))
	}
}

// TestScanAgentMissingSkillMD ensures that a subdirectory that does NOT
// contain a SKILL.md file is silently skipped.
func TestScanAgentMissingSkillMD(t *testing.T) {
	skillsDir := t.TempDir()

	// Create a directory without SKILL.md.
	bareDir := filepath.Join(skillsDir, "bare-dir")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create a valid skill alongside it.
	makeSkillDir(t, skillsDir, "valid-skill")

	el := eventlog.New(filepath.Join(t.TempDir(), "events.jsonl"))
	o := New(el, map[string]string{"agent1": skillsDir}, "")

	events := o.ScanAgent("agent1", skillsDir, time.Time{})
	if len(events) != 1 {
		t.Fatalf("expected 1 event (dir without SKILL.md skipped), got %d", len(events))
	}
	if events[0].SkillRef != "valid-skill" {
		t.Errorf("expected SkillRef = valid-skill, got %q", events[0].SkillRef)
	}
}

// TestScanAgentNonExistentDir ensures that passing a non-existent skillsDir
// path returns nil without panicking.
func TestScanAgentNonExistentDir(t *testing.T) {
	el := eventlog.New(filepath.Join(t.TempDir(), "events.jsonl"))
	o := New(el, map[string]string{}, "")

	events := o.ScanAgent("agent1", "/does/not/exist/at/all", time.Time{})
	if events != nil {
		t.Fatalf("expected nil for non-existent dir, got %v", events)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkScanAgent10 measures ScanAgent performance with 10 skill directories.
func BenchmarkScanAgent10(b *testing.B) {
	skillsDir := b.TempDir()
	for i := 0; i < 10; i++ {
		dir := filepath.Join(skillsDir, "skill-"+string(rune('a'+i)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("MkdirAll: %v", err)
		}
		mdPath := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(mdPath, []byte("# bench skill"), 0o644); err != nil {
			b.Fatalf("WriteFile: %v", err)
		}
	}

	el := eventlog.New(filepath.Join(b.TempDir(), "events.jsonl"))
	o := New(el, map[string]string{"benchAgent": skillsDir}, "")
	zero := time.Time{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = o.ScanAgent("benchAgent", skillsDir, zero)
	}
}
