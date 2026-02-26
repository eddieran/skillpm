package memory

import (
	"os"
	"path/filepath"

	"skillpm/internal/config"
	"skillpm/internal/memory/consolidation"
	mctx "skillpm/internal/memory/context"
	"skillpm/internal/memory/eventlog"
	"skillpm/internal/memory/feedback"
	"skillpm/internal/memory/observation"
	"skillpm/internal/memory/scoring"
	"skillpm/internal/store"
)

// Service is the top-level facade for the memory subsystem.
type Service struct {
	Observer      *observation.Observer
	EventLog      *eventlog.EventLog
	Context       *mctx.Engine
	Scoring       *scoring.Engine
	Feedback      *feedback.Collector
	Consolidation *consolidation.Engine
	stateRoot     string
	enabled       bool
}

// New creates a memory service. Returns a no-op service if disabled.
func New(stateRoot string, cfg config.MemoryConfig, agentDirs map[string]string) *Service {
	memRoot := store.MemoryRoot(stateRoot)
	if cfg.Enabled {
		_ = os.MkdirAll(memRoot, 0o755)
	}

	el := eventlog.New(store.EventLogPath(stateRoot))
	fb := feedback.New(store.FeedbackLogPath(stateRoot))
	ctx := &mctx.Engine{}
	halfLife := 7.0
	if cfg.RecencyHalfLife == "14d" {
		halfLife = 14.0
	} else if cfg.RecencyHalfLife == "3d" {
		halfLife = 3.0
	}
	sc := scoring.NewEngine(scoring.Config{
		WeightRecency:  0.35,
		WeightFreq:     0.25,
		WeightContext:  0.25,
		WeightFeedback: 0.15,
		HalfLifeDays:   halfLife,
	}, el, fb)

	obs := observation.New(el, agentDirs, store.LastScanPath(stateRoot))
	cons := consolidation.New(
		store.ConsolidationPath(stateRoot),
		store.ScoresPath(stateRoot),
		sc, fb, el,
	)

	return &Service{
		Observer:      obs,
		EventLog:      el,
		Context:       ctx,
		Scoring:       sc,
		Feedback:      fb,
		Consolidation: cons,
		stateRoot:     stateRoot,
		enabled:       cfg.Enabled,
	}
}

// IsEnabled returns whether the memory subsystem is active.
func (s *Service) IsEnabled() bool {
	if s == nil {
		return false
	}
	return s.enabled
}

// ScoresPath returns the path to scores.toml.
func (s *Service) ScoresPath() string {
	return store.ScoresPath(s.stateRoot)
}

// Purge removes all memory data files.
func (s *Service) Purge() error {
	memRoot := store.MemoryRoot(s.stateRoot)
	entries, err := os.ReadDir(memRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		_ = os.RemoveAll(filepath.Join(memRoot, entry.Name()))
	}
	return nil
}
