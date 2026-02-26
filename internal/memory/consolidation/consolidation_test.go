package consolidation_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"skillpm/internal/memory/consolidation"
	mctx "skillpm/internal/memory/context"
	"skillpm/internal/memory/eventlog"
	"skillpm/internal/memory/feedback"
	"skillpm/internal/memory/scoring"
)

// newEngine builds a consolidation.Engine with all dependencies wired to tmp.
func newEngine(t *testing.T, tmp string) *consolidation.Engine {
	t.Helper()
	el := eventlog.New(filepath.Join(tmp, "events.jsonl"))
	fb := feedback.New(filepath.Join(tmp, "feedback.jsonl"))
	sc := scoring.NewEngine(scoring.DefaultConfig(), el, fb)
	return consolidation.New(
		filepath.Join(tmp, "state.toml"),
		filepath.Join(tmp, "scores.toml"),
		sc, fb, el,
	)
}

// appendAccessEvent writes a single "access" event for a skill to the event log.
func appendAccessEvent(t *testing.T, el *eventlog.EventLog, skillRef string) {
	t.Helper()
	err := el.Append(eventlog.UsageEvent{
		ID:        "test-" + skillRef,
		Timestamp: time.Now().UTC(),
		SkillRef:  skillRef,
		Agent:     "test-agent",
		Kind:      eventlog.EventAccess,
	})
	if err != nil {
		t.Fatalf("failed to append event for %s: %v", skillRef, err)
	}
}

// defaultProfile returns a minimal context profile for use in tests.
func defaultProfile() mctx.Profile {
	return mctx.Profile{ProjectType: "go"}
}

// defaultSkills returns a small set of SkillInput values for testing.
func defaultSkills() []scoring.SkillInput {
	return []scoring.SkillInput{
		{SkillRef: "skill-a"},
		{SkillRef: "skill-b"},
	}
}

// TestShouldRunNeverRun verifies that a fresh engine (no state file) reports it
// should run immediately.
func TestShouldRunNeverRun(t *testing.T) {
	tmp := t.TempDir()
	eng := newEngine(t, tmp)

	should, err := eng.ShouldRun()
	if err != nil {
		t.Fatalf("ShouldRun returned error: %v", err)
	}
	if !should {
		t.Error("expected ShouldRun=true when no state file exists")
	}
}

// TestShouldRunNotOverdue verifies that an engine whose last_run is set to now
// (i.e., well within the 24 h interval) reports it should NOT run.
func TestShouldRunNotOverdue(t *testing.T) {
	tmp := t.TempDir()

	// Write a state file whose LastRun is right now.
	cs := consolidation.ConsolidationState{
		Version:       1,
		LastRun:       time.Now().UTC(),
		NextScheduled: time.Now().UTC().Add(24 * time.Hour),
		Interval:      "24h",
	}
	blob, err := toml.Marshal(cs)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "state.toml"), blob, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	eng := newEngine(t, tmp)
	should, err := eng.ShouldRun()
	if err != nil {
		t.Fatalf("ShouldRun returned error: %v", err)
	}
	if should {
		t.Error("expected ShouldRun=false when last_run is recent")
	}
}

// TestConsolidateFirstRun runs Consolidate with no prior scores and verifies
// that skills in working memory appear in the Promoted list.
func TestConsolidateFirstRun(t *testing.T) {
	tmp := t.TempDir()

	// Append events so scoring produces non-zero activation levels.
	el := eventlog.New(filepath.Join(tmp, "events.jsonl"))
	for i := 0; i < 10; i++ {
		appendAccessEvent(t, el, "skill-a")
		appendAccessEvent(t, el, "skill-b")
	}

	fb := feedback.New(filepath.Join(tmp, "feedback.jsonl"))
	sc := scoring.NewEngine(scoring.DefaultConfig(), el, fb)
	eng := consolidation.New(
		filepath.Join(tmp, "state.toml"),
		filepath.Join(tmp, "scores.toml"),
		sc, fb, el,
	)

	skills := defaultSkills()
	stats, err := eng.Consolidate(context.Background(), skills, defaultProfile(), 5, 0.0)
	if err != nil {
		t.Fatalf("Consolidate returned error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil RunStats")
	}
	if stats.SkillsEvaluated != len(skills) {
		t.Errorf("SkillsEvaluated=%d, want %d", stats.SkillsEvaluated, len(skills))
	}
	// First run: no previous scores exist, so any InWorkingMemory skills go to Promoted.
	if len(stats.Promoted) == 0 {
		t.Error("expected at least one skill in Promoted on first run")
	}
}

// TestConsolidatePersistsScores verifies that after Consolidate, scores.toml exists.
func TestConsolidatePersistsScores(t *testing.T) {
	tmp := t.TempDir()

	el := eventlog.New(filepath.Join(tmp, "events.jsonl"))
	appendAccessEvent(t, el, "skill-a")
	fb := feedback.New(filepath.Join(tmp, "feedback.jsonl"))
	sc := scoring.NewEngine(scoring.DefaultConfig(), el, fb)
	eng := consolidation.New(
		filepath.Join(tmp, "state.toml"),
		filepath.Join(tmp, "scores.toml"),
		sc, fb, el,
	)

	_, err := eng.Consolidate(context.Background(), defaultSkills(), defaultProfile(), 5, 0.0)
	if err != nil {
		t.Fatalf("Consolidate returned error: %v", err)
	}

	scoresPath := filepath.Join(tmp, "scores.toml")
	if _, err := os.Stat(scoresPath); os.IsNotExist(err) {
		t.Errorf("scores.toml was not created at %s", scoresPath)
	}
}

// TestConsolidatePersistsState verifies that after Consolidate, state.toml
// exists and contains a non-zero LastRun timestamp.
func TestConsolidatePersistsState(t *testing.T) {
	tmp := t.TempDir()

	el := eventlog.New(filepath.Join(tmp, "events.jsonl"))
	appendAccessEvent(t, el, "skill-a")
	fb := feedback.New(filepath.Join(tmp, "feedback.jsonl"))
	sc := scoring.NewEngine(scoring.DefaultConfig(), el, fb)
	eng := consolidation.New(
		filepath.Join(tmp, "state.toml"),
		filepath.Join(tmp, "scores.toml"),
		sc, fb, el,
	)

	before := time.Now().UTC().Add(-time.Second)

	_, err := eng.Consolidate(context.Background(), defaultSkills(), defaultProfile(), 5, 0.0)
	if err != nil {
		t.Fatalf("Consolidate returned error: %v", err)
	}

	statePath := filepath.Join(tmp, "state.toml")
	blob, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state.toml not found: %v", err)
	}

	var cs consolidation.ConsolidationState
	if err := toml.Unmarshal(blob, &cs); err != nil {
		t.Fatalf("failed to parse state.toml: %v", err)
	}
	if cs.LastRun.IsZero() {
		t.Error("expected LastRun to be set in state.toml")
	}
	if cs.LastRun.Before(before) {
		t.Errorf("LastRun=%v is before test start %v", cs.LastRun, before)
	}
}

// TestRecommendEmpty verifies that Recommend returns nil when no scores file exists.
func TestRecommendEmpty(t *testing.T) {
	tmp := t.TempDir()
	eng := newEngine(t, tmp)

	recs, err := eng.Recommend()
	if err != nil {
		t.Fatalf("Recommend returned error: %v", err)
	}
	if recs != nil {
		t.Errorf("expected nil recommendations with no scores, got %v", recs)
	}
}

// TestRecommendArchiveLowScore writes a scores.toml with one skill that has a
// very low activation level and verifies that Recommend returns an "archive"
// recommendation for it.
func TestRecommendArchiveLowScore(t *testing.T) {
	tmp := t.TempDir()

	// Build a ScoreBoard with one skill well below the 0.1 archive threshold.
	board := scoring.ScoreBoard{
		Version:          1,
		WorkingMemoryMax: 5,
		Threshold:        0.2,
		ComputedAt:       time.Now().UTC(),
		Scores: []scoring.SkillScore{
			{
				SkillRef:        "dormant-skill",
				ActivationLevel: 0.02,
				LastComputed:    time.Now().UTC(),
			},
			{
				SkillRef:        "active-skill",
				ActivationLevel: 0.75,
				InWorkingMemory: true,
				LastComputed:    time.Now().UTC(),
			},
		},
	}
	blob, err := toml.Marshal(board)
	if err != nil {
		t.Fatalf("marshal scores: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "scores.toml"), blob, 0o644); err != nil {
		t.Fatalf("write scores: %v", err)
	}

	eng := newEngine(t, tmp)

	recs, err := eng.Recommend()
	if err != nil {
		t.Fatalf("Recommend returned error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation for low-activation skill")
	}

	found := false
	for _, r := range recs {
		if r.Skill == "dormant-skill" && r.Kind == "archive" {
			found = true
			if r.Score >= 0.1 {
				t.Errorf("expected Score < 0.1 for archive recommendation, got %.3f", r.Score)
			}
		}
	}
	if !found {
		t.Errorf("expected archive recommendation for dormant-skill, got %+v", recs)
	}

	// Active skill must not be recommended for archiving.
	for _, r := range recs {
		if r.Skill == "active-skill" && r.Kind == "archive" {
			t.Error("active-skill should not be recommended for archive")
		}
	}
}
