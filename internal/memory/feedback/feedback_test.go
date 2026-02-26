package feedback

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillpm/internal/memory/eventlog"
)

// ---- helpers ----------------------------------------------------------------

// writeSignals serialises a slice of Signal values as JSONL into path,
// creating any required parent directories.
func writeSignals(t *testing.T, path string, sigs []Signal) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("writeSignals: MkdirAll: %v", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("writeSignals: open: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, s := range sigs {
		if err := enc.Encode(s); err != nil {
			t.Fatalf("writeSignals: encode: %v", err)
		}
	}
}

// makeAccessEvents returns count EventAccess events for skillRef, all
// timestamped at ts.
func makeAccessEvents(skillRef string, ts time.Time, count int) []eventlog.UsageEvent {
	evs := make([]eventlog.UsageEvent, count)
	for i := range evs {
		evs[i] = eventlog.UsageEvent{
			ID:        "ev-" + skillRef,
			Timestamp: ts,
			SkillRef:  skillRef,
			Kind:      eventlog.EventAccess,
		}
	}
	return evs
}

// almostEqual returns true when |a-b| < epsilon.
func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

// ---- TestRateNilCollector ---------------------------------------------------

func TestRateNilCollector(t *testing.T) {
	var c *Collector
	if err := c.Rate("skill/x", "agent", 5, "great"); err != nil {
		t.Fatalf("nil receiver should be noop, got error: %v", err)
	}
}

// ---- TestRateOutOfRange ----------------------------------------------------

func TestRateOutOfRange(t *testing.T) {
	dir := t.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))

	cases := []struct {
		name   string
		rating int
	}{
		{"zero", 0},
		{"six", 6},
		{"negative", -1},
		{"large positive", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := c.Rate("skill/x", "agent", tc.rating, "")
			if err == nil {
				t.Fatalf("expected error for rating %d, got nil", tc.rating)
			}
		})
	}
}

// ---- TestRateAndQueryRoundTrip ---------------------------------------------

func TestRateAndQueryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))

	before := time.Now().UTC().Add(-time.Second)
	if err := c.Rate("skill/alpha", "bot", 5, "excellent"); err != nil {
		t.Fatalf("Rate: %v", err)
	}

	sigs, err := c.QuerySignals(before)
	if err != nil {
		t.Fatalf("QuerySignals: %v", err)
	}
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	sig := sigs[0]
	if sig.SkillRef != "skill/alpha" {
		t.Errorf("SkillRef: want skill/alpha, got %s", sig.SkillRef)
	}
	if sig.Agent != "bot" {
		t.Errorf("Agent: want bot, got %s", sig.Agent)
	}
	if sig.Kind != FeedbackExplicit {
		t.Errorf("Kind: want explicit, got %s", sig.Kind)
	}
	if sig.Reason != "excellent" {
		t.Errorf("Reason: want excellent, got %s", sig.Reason)
	}
	// rating 5 → (5-3)/2 = 1.0
	if !almostEqual(sig.Rating, 1.0, 1e-9) {
		t.Errorf("Rating: want 1.0, got %f", sig.Rating)
	}
}

// ---- TestRateMapping -------------------------------------------------------

func TestRateMapping(t *testing.T) {
	// Map 1-5 → expected normalized values
	cases := []struct {
		rating   int
		wantNorm float64
	}{
		{1, -1.0},
		{2, -0.5},
		{3, 0.0},
		{4, 0.5},
		{5, 1.0},
	}

	dir := t.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))

	for _, tc := range cases {
		if err := c.Rate("skill/beta", "bot", tc.rating, ""); err != nil {
			t.Fatalf("Rate(%d): %v", tc.rating, err)
		}
	}

	// Average of -1, -0.5, 0, 0.5, 1.0 = 0
	avg, err := c.AggregateRating("skill/beta", time.Time{})
	if err != nil {
		t.Fatalf("AggregateRating: %v", err)
	}
	if !almostEqual(avg, 0.0, 1e-9) {
		t.Errorf("AggregateRating: want 0.0, got %f", avg)
	}

	// Verify individual signals carry the right normalised value.
	sigs, err := c.QuerySignals(time.Time{})
	if err != nil {
		t.Fatalf("QuerySignals: %v", err)
	}
	if len(sigs) != len(cases) {
		t.Fatalf("expected %d signals, got %d", len(cases), len(sigs))
	}
	for i, tc := range cases {
		if !almostEqual(sigs[i].Rating, tc.wantNorm, 1e-9) {
			t.Errorf("signal[%d] rating: want %f, got %f", i, tc.wantNorm, sigs[i].Rating)
		}
	}
}

// ---- TestAggregateRatingEmpty ----------------------------------------------

func TestAggregateRatingEmpty(t *testing.T) {
	dir := t.TempDir()
	// Create an empty log file — no signals written.
	path := filepath.Join(dir, "feedback.jsonl")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("create empty file: %v", err)
	}
	c := New(path)
	avg, err := c.AggregateRating("skill/any", time.Time{})
	if err != nil {
		t.Fatalf("AggregateRating on empty file: %v", err)
	}
	if avg != 0 {
		t.Errorf("expected 0 for empty log, got %f", avg)
	}
}

// ---- TestAggregateRatingFiltersSince ---------------------------------------

func TestAggregateRatingFiltersSince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feedback.jsonl")

	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour)

	old1 := Signal{ID: "o1", Timestamp: now.Add(-48 * time.Hour), SkillRef: "skill/g", Kind: FeedbackExplicit, Rating: -1.0}
	old2 := Signal{ID: "o2", Timestamp: now.Add(-36 * time.Hour), SkillRef: "skill/g", Kind: FeedbackExplicit, Rating: -0.5}
	new1 := Signal{ID: "n1", Timestamp: now.Add(-12 * time.Hour), SkillRef: "skill/g", Kind: FeedbackExplicit, Rating: 0.5}
	new2 := Signal{ID: "n2", Timestamp: now.Add(-6 * time.Hour), SkillRef: "skill/g", Kind: FeedbackExplicit, Rating: 1.0}

	writeSignals(t, path, []Signal{old1, old2, new1, new2})

	c := New(path)
	avg, err := c.AggregateRating("skill/g", cutoff)
	if err != nil {
		t.Fatalf("AggregateRating: %v", err)
	}
	// Only new1 (0.5) and new2 (1.0) pass the filter → average = 0.75
	want := 0.75
	if !almostEqual(avg, want, 1e-9) {
		t.Errorf("AggregateRating with since filter: want %f, got %f", want, avg)
	}
}

// ---- TestAggregateRatingNonExistentFile ------------------------------------

func TestAggregateRatingNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	c := New(filepath.Join(dir, "does-not-exist.jsonl"))

	avg, err := c.AggregateRating("skill/x", time.Time{})
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if avg != 0 {
		t.Errorf("expected 0 for missing file, got %f", avg)
	}
}

// ---- TestInferFrequentUsePositive ------------------------------------------

func TestInferFrequentUsePositive(t *testing.T) {
	dir := t.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))

	// 5 access events within the last 7 days.
	recent := time.Now().UTC().Add(-24 * time.Hour)
	events := makeAccessEvents("skill/freq", recent, 5)

	sigs := c.InferFromEvents(events, nil)

	var found bool
	for _, s := range sigs {
		if s.SkillRef == "skill/freq" && s.Reason == "frequent-use-positive" {
			found = true
			if !almostEqual(s.Rating, 0.5, 1e-9) {
				t.Errorf("frequent-use-positive rating: want 0.5, got %f", s.Rating)
			}
			if s.Kind != FeedbackImplicit {
				t.Errorf("frequent-use-positive kind: want implicit, got %s", s.Kind)
			}
		}
	}
	if !found {
		t.Fatalf("expected a frequent-use-positive signal, got signals: %+v", sigs)
	}
}

// ---- TestInferNeverAccessedNegative ----------------------------------------

func TestInferNeverAccessedNegative(t *testing.T) {
	dir := t.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))

	injectedAt := map[string]time.Time{
		"skill/stale": time.Now().UTC().Add(-31 * 24 * time.Hour),
	}
	// No access events at all.
	sigs := c.InferFromEvents(nil, injectedAt)

	var found bool
	for _, s := range sigs {
		if s.SkillRef == "skill/stale" && s.Reason == "never-accessed-negative" {
			found = true
			if !almostEqual(s.Rating, -0.3, 1e-9) {
				t.Errorf("never-accessed-negative rating: want -0.3, got %f", s.Rating)
			}
			if s.Kind != FeedbackImplicit {
				t.Errorf("never-accessed-negative kind: want implicit, got %s", s.Kind)
			}
		}
	}
	if !found {
		t.Fatalf("expected a never-accessed-negative signal, got signals: %+v", sigs)
	}
}

// ---- TestInferSessionRetentionPositive -------------------------------------

func TestInferSessionRetentionPositive(t *testing.T) {
	dir := t.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))

	now := time.Now().UTC()
	// 3 access events on 3 distinct calendar days.
	events := []eventlog.UsageEvent{
		{SkillRef: "skill/retained", Kind: eventlog.EventAccess, Timestamp: now.AddDate(0, 0, -5)},
		{SkillRef: "skill/retained", Kind: eventlog.EventAccess, Timestamp: now.AddDate(0, 0, -3)},
		{SkillRef: "skill/retained", Kind: eventlog.EventAccess, Timestamp: now.AddDate(0, 0, -1)},
	}

	sigs := c.InferFromEvents(events, nil)

	var found bool
	for _, s := range sigs {
		if s.SkillRef == "skill/retained" && s.Reason == "session-retention-positive" {
			found = true
			if !almostEqual(s.Rating, 0.3, 1e-9) {
				t.Errorf("session-retention-positive rating: want 0.3, got %f", s.Rating)
			}
			if s.Kind != FeedbackImplicit {
				t.Errorf("session-retention-positive kind: want implicit, got %s", s.Kind)
			}
		}
	}
	if !found {
		t.Fatalf("expected a session-retention-positive signal, got signals: %+v", sigs)
	}
}

// ---- TestInferNoRules ------------------------------------------------------

func TestInferNoRules(t *testing.T) {
	dir := t.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))

	// Single access event recently — does not satisfy any rule threshold.
	events := []eventlog.UsageEvent{
		{SkillRef: "skill/quiet", Kind: eventlog.EventAccess, Timestamp: time.Now().UTC().Add(-time.Hour)},
	}
	// Skill was injected only 10 days ago — does not satisfy the 30-day rule.
	injectedAt := map[string]time.Time{
		"skill/quiet": time.Now().UTC().Add(-10 * 24 * time.Hour),
	}

	sigs := c.InferFromEvents(events, injectedAt)

	for _, s := range sigs {
		if s.SkillRef == "skill/quiet" {
			t.Errorf("expected no signals for skill/quiet, got: %+v", s)
		}
	}
}

// ---- TestQuerySignalsEmpty -------------------------------------------------

func TestQuerySignalsEmpty(t *testing.T) {
	dir := t.TempDir()
	// Log file does not exist yet.
	c := New(filepath.Join(dir, "feedback.jsonl"))

	sigs, err := c.QuerySignals(time.Time{})
	if err != nil {
		t.Fatalf("QuerySignals on missing file: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected empty slice for missing file, got %d signals", len(sigs))
	}
}

// ---- TestQuerySignalsFiltersSince ------------------------------------------

func TestQuerySignalsFiltersSince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feedback.jsonl")

	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour)

	oldSig := Signal{ID: "old", Timestamp: now.Add(-48 * time.Hour), SkillRef: "skill/x", Kind: FeedbackExplicit, Rating: -1.0}
	newSig := Signal{ID: "new", Timestamp: now.Add(-12 * time.Hour), SkillRef: "skill/x", Kind: FeedbackExplicit, Rating: 1.0}

	writeSignals(t, path, []Signal{oldSig, newSig})

	c := New(path)
	sigs, err := c.QuerySignals(cutoff)
	if err != nil {
		t.Fatalf("QuerySignals: %v", err)
	}
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal after cutoff, got %d", len(sigs))
	}
	if sigs[0].ID != "new" {
		t.Errorf("expected signal with id=new, got %s", sigs[0].ID)
	}
	if !almostEqual(sigs[0].Rating, 1.0, 1e-9) {
		t.Errorf("expected rating 1.0, got %f", sigs[0].Rating)
	}
}

// ---- Benchmarks ------------------------------------------------------------

func BenchmarkRate(b *testing.B) {
	dir := b.TempDir()
	c := New(filepath.Join(dir, "feedback.jsonl"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Rate("skill/bench", "agent", (i%5)+1, "bench")
	}
}

func BenchmarkAggregateRating100(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "feedback.jsonl")

	// Pre-populate 100 signals across two skills.
	now := time.Now().UTC()
	sigs := make([]Signal, 100)
	for i := range sigs {
		rating := float64(i%5-2) / 2.0
		ref := "skill/bench"
		if i%2 == 0 {
			ref = "skill/other"
		}
		sigs[i] = Signal{
			ID:        "bench-sig",
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			SkillRef:  ref,
			Kind:      FeedbackExplicit,
			Rating:    rating,
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	enc := json.NewEncoder(f)
	for _, s := range sigs {
		if err := enc.Encode(s); err != nil {
			b.Fatalf("encode: %v", err)
		}
	}
	f.Close()

	c := New(path)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.AggregateRating("skill/bench", time.Time{})
	}
}
