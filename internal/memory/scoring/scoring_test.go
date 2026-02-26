package scoring

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillpm/internal/memory/context"
	"skillpm/internal/memory/eventlog"
	"skillpm/internal/memory/feedback"
)

const floatTol = 0.01

func withinTol(a, b float64) bool {
	return math.Abs(a-b) <= floatTol
}

// ---- ComputeRecency ----

func TestComputeRecencyZeroTime(t *testing.T) {
	got := ComputeRecency(time.Time{}, 7.0)
	if got != 0 {
		t.Errorf("expected 0 for zero time, got %v", got)
	}
}

func TestComputeRecencyNow(t *testing.T) {
	got := ComputeRecency(time.Now(), 7.0)
	// exp(-lambda*0) = 1.0; with tiny elapsed time it should be very close to 1.
	if !withinTol(got, 1.0) {
		t.Errorf("expected ~1.0 for just-accessed, got %v", got)
	}
}

func TestComputeRecencyOneHalfLife(t *testing.T) {
	// 7 days ago with halfLife=7 → exp(-ln2) = 0.5
	lastAccess := time.Now().Add(-7 * 24 * time.Hour)
	got := ComputeRecency(lastAccess, 7.0)
	if !withinTol(got, 0.5) {
		t.Errorf("expected ~0.5 for one half-life, got %v", got)
	}
}

func TestComputeRecencyTwoHalfLives(t *testing.T) {
	// 14 days ago with halfLife=7 → exp(-2*ln2) = 0.25
	lastAccess := time.Now().Add(-14 * 24 * time.Hour)
	got := ComputeRecency(lastAccess, 7.0)
	if !withinTol(got, 0.25) {
		t.Errorf("expected ~0.25 for two half-lives, got %v", got)
	}
}

// ---- ComputeFrequency ----

func TestComputeFrequencyZero(t *testing.T) {
	got := ComputeFrequency(0)
	if got != 0 {
		t.Errorf("expected 0 for zero events, got %v", got)
	}
}

func TestComputeFrequencyOne(t *testing.T) {
	// log(2)/log(101) ≈ 0.150
	got := ComputeFrequency(1)
	want := math.Log(2) / math.Log(101)
	if !withinTol(got, want) {
		t.Errorf("expected ~%.4f for 1 event, got %.4f", want, got)
	}
	if got <= 0 {
		t.Errorf("expected positive value for 1 event, got %v", got)
	}
}

func TestComputeFrequencyHundred(t *testing.T) {
	// log(101)/log(101) = 1.0
	got := ComputeFrequency(100)
	if !withinTol(got, 1.0) {
		t.Errorf("expected ~1.0 for 100 events, got %v", got)
	}
}

func TestComputeFrequencyClamped(t *testing.T) {
	// 1000 events exceeds the log scale ceiling; must be clamped to 1.0
	got := ComputeFrequency(1000)
	if got != 1.0 {
		t.Errorf("expected 1.0 (clamped) for 1000 events, got %v", got)
	}
}

// ---- ComputeContextMatch ----

func TestComputeContextMatchNoAffinity(t *testing.T) {
	profile := context.Profile{ProjectType: "go", Frameworks: []string{"cobra"}}
	affinity := context.SkillContextAffinity{} // all empty slices
	got := ComputeContextMatch(profile, affinity)
	if got != 0.5 {
		t.Errorf("expected 0.5 neutral for empty affinity, got %v", got)
	}
}

func TestComputeContextMatchPerfect(t *testing.T) {
	profile := context.Profile{ProjectType: "go"}
	affinity := context.SkillContextAffinity{
		ProjectTypes: []string{"go"},
	}
	got := ComputeContextMatch(profile, affinity)
	if !withinTol(got, 1.0) {
		t.Errorf("expected ~1.0 for perfect project type match, got %v", got)
	}
}

func TestComputeContextMatchPartialFramework(t *testing.T) {
	// Affinity requires cobra and gin; profile only has cobra → 1/2 = 0.5
	profile := context.Profile{
		Frameworks: []string{"cobra"},
	}
	affinity := context.SkillContextAffinity{
		Frameworks: []string{"cobra", "gin"},
	}
	got := ComputeContextMatch(profile, affinity)
	if !withinTol(got, 0.5) {
		t.Errorf("expected ~0.5 for 1-of-2 framework match, got %v", got)
	}
}

func TestComputeContextMatchTaskSignal(t *testing.T) {
	// Single task signal matches exactly → 1.0
	profile := context.Profile{
		TaskSignals: []string{"debugging", "testing"},
	}
	affinity := context.SkillContextAffinity{
		TaskSignals: []string{"debugging"},
	}
	got := ComputeContextMatch(profile, affinity)
	if !withinTol(got, 1.0) {
		t.Errorf("expected ~1.0 for matching task signal, got %v", got)
	}
}

func TestComputeContextMatchNoOverlap(t *testing.T) {
	// Project type mismatch, no framework or task signal overlap → average of zeros = 0.0
	profile := context.Profile{
		ProjectType: "python",
		Frameworks:  []string{"django"},
		TaskSignals: []string{"documentation"},
	}
	affinity := context.SkillContextAffinity{
		ProjectTypes: []string{"go"},
		Frameworks:   []string{"cobra", "gin"},
		TaskSignals:  []string{"debugging"},
	}
	got := ComputeContextMatch(profile, affinity)
	if !withinTol(got, 0.0) {
		t.Errorf("expected ~0.0 for no overlap, got %v", got)
	}
}

// ---- ComputeFeedbackBoost ----

func TestComputeFeedbackBoostNeutral(t *testing.T) {
	// avgRating=0 → (0+1)/2 = 0.5
	got := ComputeFeedbackBoost(0)
	if got != 0.5 {
		t.Errorf("expected 0.5 for neutral rating, got %v", got)
	}
}

func TestComputeFeedbackBoostPositive(t *testing.T) {
	// avgRating=+1.0 → (1+1)/2 = 1.0
	got := ComputeFeedbackBoost(1.0)
	if got != 1.0 {
		t.Errorf("expected 1.0 for max positive rating, got %v", got)
	}
}

func TestComputeFeedbackBoostNegative(t *testing.T) {
	// avgRating=-1.0 → (-1+1)/2 = 0.0
	got := ComputeFeedbackBoost(-1.0)
	if got != 0.0 {
		t.Errorf("expected 0.0 for max negative rating, got %v", got)
	}
}

// ---- DefaultConfig ----

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name string
		got  float64
		want float64
	}{
		{"WeightRecency", cfg.WeightRecency, 0.35},
		{"WeightFreq", cfg.WeightFreq, 0.25},
		{"WeightContext", cfg.WeightContext, 0.25},
		{"WeightFeedback", cfg.WeightFeedback, 0.15},
		{"HalfLifeDays", cfg.HalfLifeDays, 7.0},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("DefaultConfig %s: want %v, got %v", tc.name, tc.want, tc.got)
		}
	}
}

// ---- helpers for integration tests ----

// writeFeedbackSignal appends a single feedback Signal as JSONL to path.
func writeFeedbackSignal(t *testing.T, path string, sig feedback.Signal) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir feedback dir: %v", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open feedback file: %v", err)
	}
	defer f.Close()
	blob, err := json.Marshal(sig)
	if err != nil {
		t.Fatalf("marshal signal: %v", err)
	}
	if _, err := f.Write(append(blob, '\n')); err != nil {
		t.Fatalf("write feedback signal: %v", err)
	}
}

// ---- TestComputeIntegration ----

func TestComputeIntegration(t *testing.T) {
	dir := t.TempDir()
	elPath := filepath.Join(dir, "events.jsonl")
	fbPath := filepath.Join(dir, "feedback.jsonl")

	el := eventlog.New(elPath)
	fb := feedback.New(fbPath)

	now := time.Now().UTC()

	// skill-alpha: accessed 20 times recently → high frequency, near-perfect recency
	for i := 0; i < 20; i++ {
		if err := el.Append(eventlog.UsageEvent{
			ID:        "evt-alpha",
			Timestamp: now.Add(-1 * time.Hour),
			SkillRef:  "skill-alpha",
			Agent:     "agent-test",
			Kind:      eventlog.EventAccess,
		}); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	// skill-beta: accessed 5 times, 3 days ago → moderate recency, moderate frequency
	for i := 0; i < 5; i++ {
		if err := el.Append(eventlog.UsageEvent{
			ID:        "evt-beta",
			Timestamp: now.Add(-3 * 24 * time.Hour),
			SkillRef:  "skill-beta",
			Agent:     "agent-test",
			Kind:      eventlog.EventAccess,
		}); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}

	// skill-gamma: no events → zero recency, zero frequency

	// Add positive explicit feedback for skill-alpha (rating 5 → normalized +1.0)
	writeFeedbackSignal(t, fbPath, feedback.Signal{
		ID:        "fb-1",
		Timestamp: now,
		SkillRef:  "skill-alpha",
		Agent:     "agent-test",
		Kind:      feedback.FeedbackExplicit,
		Rating:    1.0,
	})

	// Add negative explicit feedback for skill-beta (rating 1 → normalized -1.0)
	writeFeedbackSignal(t, fbPath, feedback.Signal{
		ID:        "fb-2",
		Timestamp: now,
		SkillRef:  "skill-beta",
		Agent:     "agent-test",
		Kind:      feedback.FeedbackExplicit,
		Rating:    -1.0,
	})

	cfg := DefaultConfig()
	engine := NewEngine(cfg, el, fb)

	profile := context.Profile{
		ProjectType: "go",
		Frameworks:  []string{"cobra"},
	}

	skills := []SkillInput{
		{
			SkillRef: "skill-alpha",
			Affinity: context.SkillContextAffinity{
				ProjectTypes: []string{"go"},
				Frameworks:   []string{"cobra"},
			},
		},
		{
			SkillRef: "skill-beta",
			Affinity: context.SkillContextAffinity{
				ProjectTypes: []string{"python"},
			},
		},
		{
			SkillRef: "skill-gamma",
			Affinity: context.SkillContextAffinity{},
		},
	}

	board, err := engine.Compute(skills, profile, 2, 0.1)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}
	if board == nil {
		t.Fatal("Compute returned nil board")
	}
	if len(board.Scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(board.Scores))
	}

	// Verify descending sort order
	for i := 1; i < len(board.Scores); i++ {
		if board.Scores[i].ActivationLevel > board.Scores[i-1].ActivationLevel {
			t.Errorf("scores not sorted descending at index %d: %.4f > %.4f",
				i, board.Scores[i].ActivationLevel, board.Scores[i-1].ActivationLevel)
		}
	}

	// skill-alpha should be rank 0 (highest): recent + frequent + perfect context + positive feedback
	if board.Scores[0].SkillRef != "skill-alpha" {
		t.Errorf("expected skill-alpha to be rank 0, got %q", board.Scores[0].SkillRef)
	}

	// Working memory: maxSlots=2, threshold=0.1
	// Top 2 skills with activation >= 0.1 should be in working memory.
	wm := WorkingSet(board)
	if len(wm) != 2 {
		t.Errorf("expected 2 skills in working memory, got %d: %v", len(wm), wm)
	}

	// skill-gamma has zero events and neutral affinity; it should NOT be in working memory
	// (its score should be lower than the threshold or rank > maxSlots).
	gammaInWM := false
	for _, ref := range wm {
		if ref == "skill-gamma" {
			gammaInWM = true
		}
	}
	// skill-gamma can still make it into WM if its score exceeds threshold.
	// With zero recency, zero frequency, 0.5 neutral context, 0.5 neutral feedback:
	// score = 0.35*0 + 0.25*0 + 0.25*0.5 + 0.15*0.5 = 0.125+0.075 = 0.200
	// That IS above threshold 0.1, but it should still be rank 3 behind alpha and beta.
	// So it shouldn't be in WM (only top 2).
	if gammaInWM {
		t.Errorf("skill-gamma should not be in working memory (rank > maxSlots)")
	}

	// Verify ScoreBoard metadata
	if board.Version != 1 {
		t.Errorf("expected Version=1, got %d", board.Version)
	}
	if board.WorkingMemoryMax != 2 {
		t.Errorf("expected WorkingMemoryMax=2, got %d", board.WorkingMemoryMax)
	}
	if board.Threshold != 0.1 {
		t.Errorf("expected Threshold=0.1, got %v", board.Threshold)
	}
	if board.ComputedAt.IsZero() {
		t.Error("expected ComputedAt to be set")
	}
}

// ---- WorkingSet ----

func TestWorkingSetNilBoard(t *testing.T) {
	got := WorkingSet(nil)
	if got != nil {
		t.Errorf("expected nil for nil board, got %v", got)
	}
}

func TestWorkingSetFilters(t *testing.T) {
	board := &ScoreBoard{
		Scores: []SkillScore{
			{SkillRef: "skill-a", ActivationLevel: 0.9, InWorkingMemory: true},
			{SkillRef: "skill-b", ActivationLevel: 0.7, InWorkingMemory: true},
			{SkillRef: "skill-c", ActivationLevel: 0.2, InWorkingMemory: false},
		},
	}
	refs := WorkingSet(board)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs in working memory, got %d: %v", len(refs), refs)
	}
	want := map[string]bool{"skill-a": true, "skill-b": true}
	for _, ref := range refs {
		if !want[ref] {
			t.Errorf("unexpected ref in working memory: %q", ref)
		}
	}
}

// ---- Benchmarks ----

func BenchmarkComputeRecency(b *testing.B) {
	lastAccess := time.Now().Add(-3 * 24 * time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeRecency(lastAccess, 7.0)
	}
}

func BenchmarkComputeFrequency(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeFrequency(42)
	}
}

func BenchmarkComputeContextMatch(b *testing.B) {
	profile := context.Profile{
		ProjectType: "go",
		Frameworks:  []string{"cobra", "gin"},
		TaskSignals: []string{"feature", "debugging"},
	}
	affinity := context.SkillContextAffinity{
		ProjectTypes: []string{"go", "rust"},
		Frameworks:   []string{"cobra", "gorilla"},
		TaskSignals:  []string{"feature"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeContextMatch(profile, affinity)
	}
}

func BenchmarkCompute10Skills(b *testing.B) {
	dir := b.TempDir()
	elPath := filepath.Join(dir, "events.jsonl")
	fbPath := filepath.Join(dir, "feedback.jsonl")

	el := eventlog.New(elPath)
	fb := feedback.New(fbPath)

	now := time.Now().UTC()
	skills := make([]SkillInput, 10)
	for i := range skills {
		ref := "bench-skill"
		skills[i] = SkillInput{
			SkillRef: ref,
			Affinity: context.SkillContextAffinity{
				ProjectTypes: []string{"go"},
			},
		}
		_ = el.Append(eventlog.UsageEvent{
			ID:        "bench-evt",
			Timestamp: now.Add(-1 * time.Hour),
			SkillRef:  ref,
			Agent:     "bench",
			Kind:      eventlog.EventAccess,
		})
	}

	profile := context.Profile{ProjectType: "go"}
	cfg := DefaultConfig()
	engine := NewEngine(cfg, el, fb)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Compute(skills, profile, 5, 0.1)
	}
}
