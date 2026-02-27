package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillpm/internal/memory/scoring"
)

const managedMarker = "<!-- skillpm:managed -->"

// WriteRankings writes a skill rankings summary to Claude Code's auto memory
// as a topic file (skillpm-rankings.md). Never touches MEMORY.md.
func WriteRankings(home, projectPath string, board *scoring.ScoreBoard) error {
	dir := claudeProjectDir(home, projectPath)
	if dir == "" {
		return fmt.Errorf("BRIDGE_WRITE: cannot resolve Claude project dir")
	}

	content := generateRankingsContent(board)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("BRIDGE_WRITE: mkdir: %w", err)
	}

	path := filepath.Join(dir, "skillpm-rankings.md")
	return atomicWrite(path, content)
}

// CleanupRankings removes the skillpm-rankings.md file if it has the managed marker.
func CleanupRankings(home, projectPath string) error {
	dir := claudeProjectDir(home, projectPath)
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, "skillpm-rankings.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !strings.Contains(string(data), managedMarker) {
		return nil // not ours
	}
	return os.Remove(path)
}

// RankingsPath returns the path where rankings would be written.
func RankingsPath(home, projectPath string) string {
	dir := claudeProjectDir(home, projectPath)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "skillpm-rankings.md")
}

// generateRankingsContent creates a human-readable markdown summary of skill scores.
func generateRankingsContent(board *scoring.ScoreBoard) string {
	var b strings.Builder

	b.WriteString("# Skill Rankings (managed by skillpm)\n")
	b.WriteString(managedMarker + "\n\n")

	if board == nil || len(board.Scores) == 0 {
		b.WriteString("No skills scored yet.\n")
		writeFooter(&b, board)
		return b.String()
	}

	// Active skills (in working memory)
	var active []scoring.SkillScore
	var inactive []scoring.SkillScore
	for _, s := range board.Scores {
		if s.InWorkingMemory {
			active = append(active, s)
		} else {
			inactive = append(inactive, s)
		}
	}

	if len(active) > 0 {
		b.WriteString("## Active Skills (Working Memory)\n")
		for _, s := range active {
			reason := scoreReason(s)
			b.WriteString(fmt.Sprintf("- **%s** (score: %.2f) -- %s\n", s.SkillRef, s.ActivationLevel, reason))
		}
		b.WriteString("\n")
	}

	if len(inactive) > 0 {
		b.WriteString("## Inactive Skills\n")
		for _, s := range inactive {
			b.WriteString(fmt.Sprintf("- %s (score: %.2f)\n", s.SkillRef, s.ActivationLevel))
		}
		b.WriteString("\n")
	}

	writeFooter(&b, board)
	return b.String()
}

// scoreReason generates a short human-readable reason for a skill's score.
func scoreReason(s scoring.SkillScore) string {
	var parts []string
	if s.Recency > 0.7 {
		parts = append(parts, "recently used")
	}
	if s.Frequency > 0.5 {
		parts = append(parts, "frequently used")
	}
	if s.ContextMatch > 0.7 {
		parts = append(parts, "strong context match")
	}
	if s.FeedbackBoost > 0.7 {
		parts = append(parts, "positive feedback")
	}
	if len(parts) == 0 {
		return "moderate relevance"
	}
	return strings.Join(parts, ", ")
}

func writeFooter(b *strings.Builder, board *scoring.ScoreBoard) {
	now := time.Now().UTC().Format("2006-01-02 15:04 UTC")
	if board != nil {
		active := 0
		for _, s := range board.Scores {
			if s.InWorkingMemory {
				active++
			}
		}
		b.WriteString(fmt.Sprintf("---\n*Last updated: %s | Working memory: %d/%d slots*\n",
			now, active, board.WorkingMemoryMax))
	} else {
		b.WriteString(fmt.Sprintf("---\n*Last updated: %s*\n", now))
	}
}

func atomicWrite(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	return os.Rename(tmp, path)
}
