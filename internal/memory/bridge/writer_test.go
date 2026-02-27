package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillpm/internal/fsutil"
	"skillpm/internal/memory/scoring"
)

func TestGenerateRankingsContent_NilBoard(t *testing.T) {
	content := generateRankingsContent(nil)
	if !strings.Contains(content, fsutil.ManagedMarkerSimple) {
		t.Error("missing managed marker")
	}
	if !strings.Contains(content, "No skills scored yet") {
		t.Error("missing 'no skills' message")
	}
}

func TestGenerateRankingsContent_EmptyBoard(t *testing.T) {
	board := &scoring.ScoreBoard{
		Version:          1,
		WorkingMemoryMax: 12,
		Scores:           nil,
	}
	content := generateRankingsContent(board)
	if !strings.Contains(content, "No skills scored yet") {
		t.Error("missing 'no skills' message for empty board")
	}
}

func TestGenerateRankingsContent_WithActiveSkills(t *testing.T) {
	board := &scoring.ScoreBoard{
		Version:          1,
		WorkingMemoryMax: 12,
		Threshold:        0.3,
		Scores: []scoring.SkillScore{
			{
				SkillRef:        "clawhub/go-test-helper",
				ActivationLevel: 0.82,
				Recency:         0.9,
				Frequency:       0.6,
				ContextMatch:    0.8,
				FeedbackBoost:   0.8,
				InWorkingMemory: true,
			},
			{
				SkillRef:        "clawhub/code-review",
				ActivationLevel: 0.65,
				Recency:         0.3,
				Frequency:       0.4,
				ContextMatch:    0.9,
				FeedbackBoost:   0.5,
				InWorkingMemory: true,
			},
			{
				SkillRef:        "clawhub/deploy-helper",
				ActivationLevel: 0.20,
				Recency:         0.1,
				Frequency:       0.1,
				ContextMatch:    0.3,
				FeedbackBoost:   0.5,
				InWorkingMemory: false,
			},
		},
		ComputedAt: time.Now().UTC(),
	}

	content := generateRankingsContent(board)

	if !strings.Contains(content, "## Active Skills (Working Memory)") {
		t.Error("missing active skills section")
	}
	if !strings.Contains(content, "**clawhub/go-test-helper**") {
		t.Error("missing go-test-helper in active")
	}
	if !strings.Contains(content, "recently used") {
		t.Error("missing 'recently used' reason for high recency")
	}
	if !strings.Contains(content, "## Inactive Skills") {
		t.Error("missing inactive skills section")
	}
	if !strings.Contains(content, "clawhub/deploy-helper") {
		t.Error("missing deploy-helper in inactive")
	}
	if !strings.Contains(content, "Working memory: 2/12 slots") {
		t.Error("missing footer stats")
	}
}

func TestWriteRankings_CreatesFile(t *testing.T) {
	home := t.TempDir()
	projectPath := "/test/project"

	board := &scoring.ScoreBoard{
		Version:          1,
		WorkingMemoryMax: 12,
		Scores: []scoring.SkillScore{
			{SkillRef: "test/skill", ActivationLevel: 0.5, InWorkingMemory: true},
		},
	}

	err := WriteRankings(home, projectPath, board)
	if err != nil {
		t.Fatalf("WriteRankings error: %v", err)
	}

	path := RankingsPath(home, projectPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rankings: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, fsutil.ManagedMarkerSimple) {
		t.Error("missing managed marker")
	}
	if !strings.Contains(content, "test/skill") {
		t.Error("missing skill ref")
	}
}

func TestWriteRankings_OverwritesExisting(t *testing.T) {
	home := t.TempDir()
	projectPath := "/test/project"

	board1 := &scoring.ScoreBoard{
		Version:          1,
		WorkingMemoryMax: 12,
		Scores: []scoring.SkillScore{
			{SkillRef: "test/skill-v1", ActivationLevel: 0.5, InWorkingMemory: true},
		},
	}
	board2 := &scoring.ScoreBoard{
		Version:          1,
		WorkingMemoryMax: 12,
		Scores: []scoring.SkillScore{
			{SkillRef: "test/skill-v2", ActivationLevel: 0.8, InWorkingMemory: true},
		},
	}

	WriteRankings(home, projectPath, board1)
	WriteRankings(home, projectPath, board2)

	data, _ := os.ReadFile(RankingsPath(home, projectPath))
	content := string(data)
	if strings.Contains(content, "skill-v1") {
		t.Error("old content should be replaced")
	}
	if !strings.Contains(content, "skill-v2") {
		t.Error("new content should be present")
	}
}

func TestCleanupRankings_RemovesManaged(t *testing.T) {
	home := t.TempDir()
	projectPath := "/test/project"

	board := &scoring.ScoreBoard{
		Version:          1,
		WorkingMemoryMax: 12,
		Scores: []scoring.SkillScore{
			{SkillRef: "test/skill", ActivationLevel: 0.5, InWorkingMemory: true},
		},
	}
	WriteRankings(home, projectPath, board)

	err := CleanupRankings(home, projectPath)
	if err != nil {
		t.Fatalf("CleanupRankings error: %v", err)
	}

	path := RankingsPath(home, projectPath)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("rankings file should be removed after cleanup")
	}
}

func TestCleanupRankings_SkipsUserFile(t *testing.T) {
	home := t.TempDir()
	projectPath := "/test/project"

	// Write a file without managed marker (simulating user-created file)
	dir := claudeProjectDir(home, projectPath)
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "skillpm-rankings.md")
	os.WriteFile(path, []byte("# My custom rankings\nHand written."), 0o644)

	err := CleanupRankings(home, projectPath)
	if err != nil {
		t.Fatalf("CleanupRankings error: %v", err)
	}

	// File should still exist
	if _, err := os.Stat(path); err != nil {
		t.Error("user file should NOT be removed")
	}
}

func TestCleanupRankings_NoFile(t *testing.T) {
	err := CleanupRankings("/nonexistent", "/nonexistent/project")
	if err != nil {
		t.Fatalf("CleanupRankings should not error on missing file: %v", err)
	}
}

func TestScoreReason(t *testing.T) {
	tests := []struct {
		name  string
		score scoring.SkillScore
		want  string
	}{
		{
			"recently used",
			scoring.SkillScore{Recency: 0.9, Frequency: 0.1, ContextMatch: 0.1, FeedbackBoost: 0.1},
			"recently used",
		},
		{
			"frequently used",
			scoring.SkillScore{Recency: 0.1, Frequency: 0.8, ContextMatch: 0.1, FeedbackBoost: 0.1},
			"frequently used",
		},
		{
			"strong context",
			scoring.SkillScore{Recency: 0.1, Frequency: 0.1, ContextMatch: 0.9, FeedbackBoost: 0.1},
			"strong context match",
		},
		{
			"positive feedback",
			scoring.SkillScore{Recency: 0.1, Frequency: 0.1, ContextMatch: 0.1, FeedbackBoost: 0.9},
			"positive feedback",
		},
		{
			"moderate",
			scoring.SkillScore{Recency: 0.3, Frequency: 0.3, ContextMatch: 0.3, FeedbackBoost: 0.3},
			"moderate relevance",
		},
		{
			"multiple reasons",
			scoring.SkillScore{Recency: 0.9, Frequency: 0.8, ContextMatch: 0.9, FeedbackBoost: 0.9},
			"recently used, frequently used, strong context match, positive feedback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreReason(tt.score)
			if got != tt.want {
				t.Errorf("scoreReason = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRankingsPath(t *testing.T) {
	home := "/Users/test"
	path := RankingsPath(home, "/test/project")
	if !strings.HasSuffix(path, "skillpm-rankings.md") {
		t.Errorf("RankingsPath = %q, should end with skillpm-rankings.md", path)
	}
	if !strings.Contains(path, ".claude/projects/") {
		t.Errorf("RankingsPath = %q, should contain .claude/projects/", path)
	}
}
