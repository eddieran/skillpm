package eventlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newEvent builds a minimal UsageEvent for tests. ts may be zero, in which
// case Append will stamp it with time.Now().
func newEvent(id, skillRef, agent string, kind EventKind, ts time.Time) UsageEvent {
	return UsageEvent{
		ID:        id,
		Timestamp: ts,
		SkillRef:  skillRef,
		Agent:     agent,
		Kind:      kind,
		Scope:     "global",
		Context: EventContext{
			ProjectRoot: "/tmp/proj",
			ProjectType: "go",
			TaskType:    "build",
		},
		Fields: map[string]string{"key": "val"},
	}
}

// logPath returns a path inside a fresh TempDir sub-directory so that the
// parent directory does not yet exist, letting Append create it.
func logPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "eventlog", "events.jsonl")
}

// ---------------------------------------------------------------------------
// TestAppendNilAndEmpty
// ---------------------------------------------------------------------------

func TestAppendNilAndEmpty(t *testing.T) {
	// nil receiver must be a noop.
	var nilLog *EventLog
	if err := nilLog.Append(newEvent("e1", "a/skill", "agent1", EventAccess, time.Now())); err != nil {
		t.Fatalf("nil receiver Append: %v", err)
	}

	// empty path must be a noop even with real events.
	emptyLog := New("")
	if err := emptyLog.Append(newEvent("e2", "a/skill", "agent1", EventAccess, time.Now())); err != nil {
		t.Fatalf("empty-path Append: %v", err)
	}

	// zero events must be a noop.
	el := New(logPath(t))
	if err := el.Append(); err != nil {
		t.Fatalf("zero-event Append: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestAppendCreatesDir
// ---------------------------------------------------------------------------

func TestAppendCreatesDir(t *testing.T) {
	path := logPath(t) // parent dir does not exist yet
	el := New(path)

	ev := newEvent("e1", "org/skill", "agentA", EventInvoke, time.Now())
	if err := el.Append(ev); err != nil {
		t.Fatalf("Append to non-existent dir: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestAppendAndQueryRoundTrip
// ---------------------------------------------------------------------------

func TestAppendAndQueryRoundTrip(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []UsageEvent{
		newEvent("id1", "org/alpha", "bot", EventAccess, base.Add(1*time.Hour)),
		newEvent("id2", "org/beta", "bot", EventInvoke, base.Add(2*time.Hour)),
		newEvent("id3", "org/gamma", "bot", EventComplete, base.Add(3*time.Hour)),
	}

	if err := el.Append(events...); err != nil {
		t.Fatalf("Append 3 events: %v", err)
	}

	got, err := el.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}

	for i, want := range events {
		g := got[i]
		if g.ID != want.ID {
			t.Fatalf("event[%d] ID: want %q, got %q", i, want.ID, g.ID)
		}
		if g.SkillRef != want.SkillRef {
			t.Fatalf("event[%d] SkillRef: want %q, got %q", i, want.SkillRef, g.SkillRef)
		}
		if g.Agent != want.Agent {
			t.Fatalf("event[%d] Agent: want %q, got %q", i, want.Agent, g.Agent)
		}
		if g.Kind != want.Kind {
			t.Fatalf("event[%d] Kind: want %q, got %q", i, want.Kind, g.Kind)
		}
		if g.Scope != want.Scope {
			t.Fatalf("event[%d] Scope: want %q, got %q", i, want.Scope, g.Scope)
		}
		if g.Context != want.Context {
			t.Fatalf("event[%d] Context: want %+v, got %+v", i, want.Context, g.Context)
		}
		if g.Fields["key"] != want.Fields["key"] {
			t.Fatalf("event[%d] Fields[key]: want %q, got %q", i, want.Fields["key"], g.Fields["key"])
		}
	}
}

// ---------------------------------------------------------------------------
// TestQueryFilterBySince
// ---------------------------------------------------------------------------

func TestQueryFilterBySince(t *testing.T) {
	path := logPath(t)
	el := New(path)

	pivot := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	old1 := newEvent("old1", "s/skill", "a", EventAccess, pivot.Add(-24*time.Hour))
	old2 := newEvent("old2", "s/skill", "a", EventAccess, pivot.Add(-1*time.Hour))
	new1 := newEvent("new1", "s/skill", "a", EventAccess, pivot.Add(1*time.Hour))
	new2 := newEvent("new2", "s/skill", "a", EventAccess, pivot.Add(24*time.Hour))

	if err := el.Append(old1, old2, new1, new2); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := el.Query(QueryFilter{Since: pivot})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events since pivot, got %d", len(got))
	}
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids["new1"] || !ids["new2"] {
		t.Fatalf("expected new1 and new2, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// TestQueryFilterBySkillRef
// ---------------------------------------------------------------------------

func TestQueryFilterBySkillRef(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := el.Append(
		newEvent("e1", "org/target", "a", EventAccess, base.Add(1*time.Minute)),
		newEvent("e2", "org/other", "a", EventAccess, base.Add(2*time.Minute)),
		newEvent("e3", "org/target", "a", EventAccess, base.Add(3*time.Minute)),
	); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := el.Query(QueryFilter{SkillRef: "org/target"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events for org/target, got %d", len(got))
	}
	for _, ev := range got {
		if ev.SkillRef != "org/target" {
			t.Fatalf("unexpected skill_ref %q in result", ev.SkillRef)
		}
	}
}

// ---------------------------------------------------------------------------
// TestQueryFilterByAgent
// ---------------------------------------------------------------------------

func TestQueryFilterByAgent(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := el.Append(
		newEvent("e1", "s/skill", "claude", EventAccess, base.Add(1*time.Minute)),
		newEvent("e2", "s/skill", "cursor", EventAccess, base.Add(2*time.Minute)),
		newEvent("e3", "s/skill", "claude", EventAccess, base.Add(3*time.Minute)),
	); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := el.Query(QueryFilter{Agent: "claude"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events for agent claude, got %d", len(got))
	}
	for _, ev := range got {
		if ev.Agent != "claude" {
			t.Fatalf("unexpected agent %q in result", ev.Agent)
		}
	}
}

// ---------------------------------------------------------------------------
// TestQueryFilterByKind
// ---------------------------------------------------------------------------

func TestQueryFilterByKind(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := el.Append(
		newEvent("e1", "s/skill", "a", EventAccess, base.Add(1*time.Minute)),
		newEvent("e2", "s/skill", "a", EventError, base.Add(2*time.Minute)),
		newEvent("e3", "s/skill", "a", EventAccess, base.Add(3*time.Minute)),
		newEvent("e4", "s/skill", "a", EventFeedback, base.Add(4*time.Minute)),
	); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := el.Query(QueryFilter{Kind: EventAccess})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 access events, got %d", len(got))
	}
	for _, ev := range got {
		if ev.Kind != EventAccess {
			t.Fatalf("unexpected kind %q in result", ev.Kind)
		}
	}
}

// ---------------------------------------------------------------------------
// TestQueryFilterLimit
// ---------------------------------------------------------------------------

func TestQueryFilterLimit(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	events := make([]UsageEvent, 5)
	for i := range events {
		events[i] = newEvent(fmt.Sprintf("e%d", i), "s/skill", "a", EventAccess, base.Add(time.Duration(i)*time.Minute))
	}
	if err := el.Append(events...); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := el.Query(QueryFilter{Limit: 1})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event with limit=1, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// TestQueryNonExistentFile
// ---------------------------------------------------------------------------

func TestQueryNonExistentFile(t *testing.T) {
	el := New(filepath.Join(t.TempDir(), "no-such-dir", "events.jsonl"))

	got, err := el.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query on missing file returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil slice for missing file, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// TestQuerySkipsMalformedLines
// ---------------------------------------------------------------------------

func TestQuerySkipsMalformedLines(t *testing.T) {
	path := logPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	el := New(path)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	good := newEvent("good1", "s/skill", "a", EventAccess, base.Add(1*time.Minute))
	if err := el.Append(good); err != nil {
		t.Fatalf("Append good event: %v", err)
	}

	// inject a malformed line directly.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open log for injection: %v", err)
	}
	if _, err := f.WriteString("this is {not: valid json}\n"); err != nil {
		f.Close()
		t.Fatalf("inject bad line: %v", err)
	}
	f.Close()

	good2 := newEvent("good2", "s/skill", "a", EventAccess, base.Add(2*time.Minute))
	if err := el.Append(good2); err != nil {
		t.Fatalf("Append second good event: %v", err)
	}

	got, err := el.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 valid events (malformed skipped), got %d", len(got))
	}
	for _, ev := range got {
		if ev.ID != "good1" && ev.ID != "good2" {
			t.Fatalf("unexpected event ID %q in results", ev.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// TestStatsAggregation
// ---------------------------------------------------------------------------

func TestStatsAggregation(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	// skill-A: 3 events from 2 agents; skill-B: 2 events from 1 agent.
	if err := el.Append(
		newEvent("a1", "org/skill-a", "agent1", EventAccess, base.Add(1*time.Hour)),
		newEvent("a2", "org/skill-a", "agent2", EventInvoke, base.Add(2*time.Hour)),
		newEvent("a3", "org/skill-a", "agent1", EventComplete, base.Add(3*time.Hour)),
		newEvent("b1", "org/skill-b", "agent3", EventAccess, base.Add(4*time.Hour)),
		newEvent("b2", "org/skill-b", "agent3", EventError, base.Add(5*time.Hour)),
	); err != nil {
		t.Fatalf("Append: %v", err)
	}

	stats, err := el.Stats(time.Time{})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected stats for 2 skills, got %d", len(stats))
	}

	// index by skill ref for assertion.
	m := make(map[string]SkillStats, len(stats))
	for _, s := range stats {
		m[s.SkillRef] = s
	}

	sa, ok := m["org/skill-a"]
	if !ok {
		t.Fatalf("missing stats for org/skill-a")
	}
	if sa.EventCount != 3 {
		t.Fatalf("org/skill-a: expected EventCount=3, got %d", sa.EventCount)
	}
	wantLastA := base.Add(3 * time.Hour)
	if !sa.LastAccess.Equal(wantLastA) {
		t.Fatalf("org/skill-a: expected LastAccess=%v, got %v", wantLastA, sa.LastAccess)
	}
	if len(sa.Agents) != 2 {
		t.Fatalf("org/skill-a: expected 2 unique agents, got %d: %v", len(sa.Agents), sa.Agents)
	}
	agentSet := map[string]bool{}
	for _, ag := range sa.Agents {
		agentSet[ag] = true
	}
	if !agentSet["agent1"] || !agentSet["agent2"] {
		t.Fatalf("org/skill-a: expected agents {agent1, agent2}, got %v", sa.Agents)
	}

	sb, ok := m["org/skill-b"]
	if !ok {
		t.Fatalf("missing stats for org/skill-b")
	}
	if sb.EventCount != 2 {
		t.Fatalf("org/skill-b: expected EventCount=2, got %d", sb.EventCount)
	}
	wantLastB := base.Add(5 * time.Hour)
	if !sb.LastAccess.Equal(wantLastB) {
		t.Fatalf("org/skill-b: expected LastAccess=%v, got %v", wantLastB, sb.LastAccess)
	}
	if len(sb.Agents) != 1 || sb.Agents[0] != "agent3" {
		t.Fatalf("org/skill-b: expected agents {agent3}, got %v", sb.Agents)
	}
}

// TestStatsFiltersBySince verifies that events before the since cutoff are
// excluded from the aggregation.
func TestStatsFiltersBySince(t *testing.T) {
	path := logPath(t)
	el := New(path)

	pivot := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := el.Append(
		newEvent("old", "s/skill", "a", EventAccess, pivot.Add(-1*time.Hour)),
		newEvent("new", "s/skill", "a", EventAccess, pivot.Add(1*time.Hour)),
	); err != nil {
		t.Fatalf("Append: %v", err)
	}

	stats, err := el.Stats(pivot)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 skill stat after since filter, got %d", len(stats))
	}
	if stats[0].EventCount != 1 {
		t.Fatalf("expected EventCount=1 after since filter, got %d", stats[0].EventCount)
	}
}

// ---------------------------------------------------------------------------
// TestTruncateRemovesOld
// ---------------------------------------------------------------------------

func TestTruncateRemovesOld(t *testing.T) {
	path := logPath(t)
	el := New(path)

	cutoff := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	old1 := newEvent("old1", "s/skill", "a", EventAccess, cutoff.Add(-48*time.Hour))
	old2 := newEvent("old2", "s/skill", "a", EventAccess, cutoff.Add(-1*time.Hour))
	new1 := newEvent("new1", "s/skill", "a", EventAccess, cutoff.Add(1*time.Hour))
	new2 := newEvent("new2", "s/skill", "a", EventAccess, cutoff.Add(48*time.Hour))

	if err := el.Append(old1, old2, new1, new2); err != nil {
		t.Fatalf("Append: %v", err)
	}

	removed, err := el.Truncate(cutoff)
	if err != nil {
		t.Fatalf("Truncate: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	remaining, err := el.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query after Truncate: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining events, got %d", len(remaining))
	}
	ids := map[string]bool{}
	for _, ev := range remaining {
		ids[ev.ID] = true
	}
	if !ids["new1"] || !ids["new2"] {
		t.Fatalf("expected new1 and new2 to remain, got %v", remaining)
	}
}

// ---------------------------------------------------------------------------
// TestTruncateNoop
// ---------------------------------------------------------------------------

func TestTruncateNoop(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := el.Append(
		newEvent("e1", "s/skill", "a", EventAccess, base.Add(1*time.Hour)),
		newEvent("e2", "s/skill", "a", EventAccess, base.Add(2*time.Hour)),
	); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// cutoff is far in the past; nothing should be removed.
	cutoff := base.Add(-24 * time.Hour)
	removed, err := el.Truncate(cutoff)
	if err != nil {
		t.Fatalf("Truncate noop: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}

	// file must still contain both events.
	remaining, err := el.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query after noop Truncate: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 events after noop Truncate, got %d", len(remaining))
	}
}

// TestTruncateNonExistentFile verifies that Truncate on a missing file is
// a noop rather than an error.
func TestTruncateNonExistentFile(t *testing.T) {
	el := New(filepath.Join(t.TempDir(), "no-such-dir", "events.jsonl"))
	removed, err := el.Truncate(time.Now())
	if err != nil {
		t.Fatalf("Truncate on missing file returned error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed for missing file, got %d", removed)
	}
}

// ---------------------------------------------------------------------------
// TestTruncateNilReceiver
// ---------------------------------------------------------------------------

func TestTruncateNilReceiver(t *testing.T) {
	var nilLog *EventLog
	removed, err := nilLog.Truncate(time.Now())
	if err != nil {
		t.Fatalf("nil receiver Truncate: %v", err)
	}
	if removed != 0 {
		t.Fatalf("nil receiver Truncate: expected 0, got %d", removed)
	}
}

// ---------------------------------------------------------------------------
// Additional edge-case tests
// ---------------------------------------------------------------------------

// TestAppendAutoTimestamp verifies that events with a zero Timestamp get
// stamped by Append rather than stored as zero.
func TestAppendAutoTimestamp(t *testing.T) {
	path := logPath(t)
	el := New(path)

	ev := UsageEvent{ID: "ts-test", SkillRef: "s/skill", Agent: "a", Kind: EventAccess}
	// Timestamp is zero by construction.
	if ev.Timestamp != (time.Time{}) {
		t.Fatalf("precondition failed: timestamp should be zero")
	}

	before := time.Now().UTC()
	if err := el.Append(ev); err != nil {
		t.Fatalf("Append: %v", err)
	}
	after := time.Now().UTC()

	got, err := el.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	ts := got[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Fatalf("auto-stamped timestamp %v not in [%v, %v]", ts, before, after)
	}
}

// TestQueryCombinedFilters verifies that multiple non-zero filter fields
// are applied together (AND semantics).
func TestQueryCombinedFilters(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := el.Append(
		newEvent("match", "org/x", "bot", EventInvoke, base.Add(1*time.Hour)),
		newEvent("wrong-skill", "org/y", "bot", EventInvoke, base.Add(2*time.Hour)),
		newEvent("wrong-agent", "org/x", "human", EventInvoke, base.Add(3*time.Hour)),
		newEvent("wrong-kind", "org/x", "bot", EventError, base.Add(4*time.Hour)),
	); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := el.Query(QueryFilter{SkillRef: "org/x", Agent: "bot", Kind: EventInvoke})
	if err != nil {
		t.Fatalf("Query combined: %v", err)
	}
	if len(got) != 1 || got[0].ID != "match" {
		t.Fatalf("expected only 'match' event, got %v", got)
	}
}

// TestQueryMultipleAppendCalls checks that successive Append calls
// accumulate events in a single file.
func TestQueryMultipleAppendCalls(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ev := newEvent(fmt.Sprintf("e%d", i), "s/skill", "a", EventAccess, base.Add(time.Duration(i)*time.Minute))
		if err := el.Append(ev); err != nil {
			t.Fatalf("Append event %d: %v", i, err)
		}
	}

	got, err := el.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 events from 5 separate Append calls, got %d", len(got))
	}
}

// TestStatsSortable verifies that Stats results can be deterministically
// sorted by SkillRef (since map iteration order is not guaranteed).
func TestStatsSortable(t *testing.T) {
	path := logPath(t)
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	refs := []string{"c/skill", "a/skill", "b/skill"}
	for i, ref := range refs {
		if err := el.Append(newEvent(fmt.Sprintf("e%d", i), ref, "a", EventAccess, base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	stats, err := el.Stats(time.Time{})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("expected 3 skill stats, got %d", len(stats))
	}

	sorted := make([]SkillStats, len(stats))
	copy(sorted, stats)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].SkillRef < sorted[j].SkillRef })
	if sorted[0].SkillRef != "a/skill" || sorted[1].SkillRef != "b/skill" || sorted[2].SkillRef != "c/skill" {
		t.Fatalf("unexpected order after sort: %v", sorted)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkAppend measures the cost of writing a single event to disk.
func BenchmarkAppend(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench", "events.jsonl")
	el := New(path)
	ev := newEvent("bench-id", "org/bench-skill", "agent", EventAccess, time.Now().UTC())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := el.Append(ev); err != nil {
			b.Fatalf("Append: %v", err)
		}
	}
}

// BenchmarkQuery100 measures the cost of reading a log with 100 events.
func BenchmarkQuery100(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench", "events.jsonl")
	el := New(path)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	events := make([]UsageEvent, 100)
	for i := range events {
		events[i] = newEvent(fmt.Sprintf("bench-%d", i), "org/skill", "agent", EventAccess, base.Add(time.Duration(i)*time.Second))
	}
	if err := el.Append(events...); err != nil {
		b.Fatalf("Append seed events: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := el.Query(QueryFilter{})
		if err != nil {
			b.Fatalf("Query: %v", err)
		}
		if len(got) != 100 {
			b.Fatalf("expected 100 events, got %d", len(got))
		}
	}
}
