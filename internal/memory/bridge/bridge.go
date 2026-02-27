package bridge

import (
	"os"

	"skillpm/internal/memory/scoring"
)

// Service is the facade for the memory bridge subsystem.
// It provides bidirectional sync between skillpm and Claude Code's auto memory.
type Service struct {
	home        string
	projectPath string
}

// NewService creates a bridge service. Returns nil if home is empty.
func NewService(home, projectPath string) *Service {
	if home == "" {
		return nil
	}
	return &Service{
		home:        home,
		projectPath: projectPath,
	}
}

// ReadContext reads Claude Code's MEMORY.md and returns extracted signals
// for context enrichment. Returns empty signals if unavailable.
func (s *Service) ReadContext() MemorySignals {
	if s == nil {
		return MemorySignals{}
	}
	return ReadMemorySignals(s.home, s.projectPath)
}

// WriteRankings writes the current skill score board to Claude Code's
// auto memory as a topic file (skillpm-rankings.md).
func (s *Service) WriteRankings(board *scoring.ScoreBoard) error {
	if s == nil {
		return nil
	}
	return WriteRankings(s.home, s.projectPath, board)
}

// Cleanup removes the skillpm-rankings.md file from Claude Code's memory.
func (s *Service) Cleanup() error {
	if s == nil {
		return nil
	}
	return CleanupRankings(s.home, s.projectPath)
}

// RankingsPath returns the path where rankings would be written.
func (s *Service) RankingsPath() string {
	if s == nil {
		return ""
	}
	return RankingsPath(s.home, s.projectPath)
}

// MemoryDir returns the Claude Code project memory directory path.
func (s *Service) MemoryDir() string {
	if s == nil {
		return ""
	}
	return claudeProjectDir(s.home, s.projectPath)
}

// Available checks if Claude Code's memory directory exists for this project.
func (s *Service) Available() bool {
	if s == nil {
		return false
	}
	dir := claudeProjectDir(s.home, s.projectPath)
	if dir == "" {
		return false
	}
	// Check if the memory directory exists (Claude Code creates it)
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
