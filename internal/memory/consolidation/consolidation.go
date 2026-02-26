package consolidation

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
	mctx "skillpm/internal/memory/context"
	"skillpm/internal/memory/eventlog"
	"skillpm/internal/memory/feedback"
	"skillpm/internal/memory/scoring"
)

// ConsolidationState tracks when consolidation last ran.
type ConsolidationState struct {
	Version       int       `toml:"version"`
	LastRun       time.Time `toml:"last_run"`
	NextScheduled time.Time `toml:"next_scheduled"`
	Interval      string    `toml:"interval"`
}

// RunStats summarizes a consolidation run.
type RunStats struct {
	SkillsEvaluated int      `json:"skills_evaluated"`
	Strengthened    []string `json:"strengthened"`
	Decayed         []string `json:"decayed"`
	Promoted        []string `json:"promoted"`
	Demoted         []string `json:"demoted"`
}

// Recommendation suggests an action for a skill.
type Recommendation struct {
	Kind   string  `json:"kind"` // "install", "remove", "promote", "archive"
	Skill  string  `json:"skill"`
	Reason string  `json:"reason"`
	Score  float64 `json:"score"`
}

// Engine performs periodic memory consolidation.
type Engine struct {
	statePath  string
	scoresPath string
	scoring    *scoring.Engine
	feedback   *feedback.Collector
	eventLog   *eventlog.EventLog
}

// New creates a consolidation engine.
func New(statePath, scoresPath string, sc *scoring.Engine, fb *feedback.Collector, el *eventlog.EventLog) *Engine {
	return &Engine{
		statePath:  statePath,
		scoresPath: scoresPath,
		scoring:    sc,
		feedback:   fb,
		eventLog:   el,
	}
}

// Consolidate runs the full consolidation pipeline.
func (e *Engine) Consolidate(_ context.Context, skills []scoring.SkillInput, profile mctx.Profile, maxSlots int, threshold float64) (*RunStats, error) {
	// Load previous scores
	prevBoard := e.loadScores()

	// Recompute scores
	board, err := e.scoring.Compute(skills, profile, maxSlots, threshold)
	if err != nil {
		return nil, fmt.Errorf("MEM_CONSOLIDATE_RUN: %w", err)
	}

	// Compare with previous scores
	stats := &RunStats{SkillsEvaluated: len(board.Scores)}
	prevMap := map[string]scoring.SkillScore{}
	if prevBoard != nil {
		for _, s := range prevBoard.Scores {
			prevMap[s.SkillRef] = s
		}
	}

	for _, s := range board.Scores {
		prev, existed := prevMap[s.SkillRef]
		if !existed {
			if s.InWorkingMemory {
				stats.Promoted = append(stats.Promoted, s.SkillRef)
			}
			continue
		}
		delta := s.ActivationLevel - prev.ActivationLevel
		if delta > 0.05 {
			stats.Strengthened = append(stats.Strengthened, s.SkillRef)
		} else if delta < -0.05 {
			stats.Decayed = append(stats.Decayed, s.SkillRef)
		}
		if s.InWorkingMemory && !prev.InWorkingMemory {
			stats.Promoted = append(stats.Promoted, s.SkillRef)
		} else if !s.InWorkingMemory && prev.InWorkingMemory {
			stats.Demoted = append(stats.Demoted, s.SkillRef)
		}
	}

	// Persist new scores
	e.saveScores(board)

	// Update consolidation state
	now := time.Now().UTC()
	cs := ConsolidationState{
		Version:       1,
		LastRun:       now,
		NextScheduled: now.Add(24 * time.Hour),
		Interval:      "24h",
	}
	e.saveState(cs)

	return stats, nil
}

// ShouldRun checks if consolidation is overdue.
func (e *Engine) ShouldRun() (bool, error) {
	cs := e.loadState()
	if cs.LastRun.IsZero() {
		return true, nil
	}
	interval, err := time.ParseDuration(cs.Interval)
	if err != nil {
		interval = 24 * time.Hour
	}
	return time.Now().UTC().After(cs.LastRun.Add(interval)), nil
}

// Recommend generates action recommendations based on current scores.
func (e *Engine) Recommend() ([]Recommendation, error) {
	board := e.loadScores()
	if board == nil {
		return nil, nil
	}
	var recs []Recommendation
	for _, s := range board.Scores {
		if s.ActivationLevel < 0.1 {
			recs = append(recs, Recommendation{
				Kind:   "archive",
				Skill:  s.SkillRef,
				Reason: fmt.Sprintf("very low activation (%.2f)", s.ActivationLevel),
				Score:  s.ActivationLevel,
			})
		}
	}
	return recs, nil
}

func (e *Engine) loadState() ConsolidationState {
	var cs ConsolidationState
	cs.Interval = "24h"
	blob, err := os.ReadFile(e.statePath)
	if err != nil {
		return cs
	}
	_ = toml.Unmarshal(blob, &cs)
	if cs.Interval == "" {
		cs.Interval = "24h"
	}
	return cs
}

func (e *Engine) saveState(cs ConsolidationState) {
	blob, err := toml.Marshal(cs)
	if err != nil {
		return
	}
	tmp := e.statePath + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, e.statePath)
}

func (e *Engine) loadScores() *scoring.ScoreBoard {
	blob, err := os.ReadFile(e.scoresPath)
	if err != nil {
		return nil
	}
	var board scoring.ScoreBoard
	if err := toml.Unmarshal(blob, &board); err != nil {
		return nil
	}
	return &board
}

func (e *Engine) saveScores(board *scoring.ScoreBoard) {
	blob, err := toml.Marshal(board)
	if err != nil {
		return
	}
	tmp := e.scoresPath + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, e.scoresPath)
}
