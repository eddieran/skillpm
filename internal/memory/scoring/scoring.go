package scoring

import (
	"math"
	"sort"
	"time"

	"skillpm/internal/memory/context"
	"skillpm/internal/memory/eventlog"
	"skillpm/internal/memory/feedback"
)

// Config holds scoring weights and parameters.
type Config struct {
	WeightRecency  float64
	WeightFreq     float64
	WeightContext  float64
	WeightFeedback float64
	HalfLifeDays   float64
}

// DefaultConfig returns the default scoring configuration.
func DefaultConfig() Config {
	return Config{
		WeightRecency:  0.35,
		WeightFreq:     0.25,
		WeightContext:  0.25,
		WeightFeedback: 0.15,
		HalfLifeDays:   7.0,
	}
}

// SkillScore holds the activation score breakdown for a single skill.
type SkillScore struct {
	SkillRef        string    `toml:"skill_ref" json:"skill_ref"`
	ActivationLevel float64   `toml:"activation_level" json:"activation_level"`
	Recency         float64   `toml:"recency" json:"recency"`
	Frequency       float64   `toml:"frequency" json:"frequency"`
	ContextMatch    float64   `toml:"context_match" json:"context_match"`
	FeedbackBoost   float64   `toml:"feedback_boost" json:"feedback_boost"`
	InWorkingMemory bool      `toml:"in_working_memory" json:"in_working_memory"`
	LastComputed    time.Time `toml:"last_computed" json:"last_computed"`
}

// ScoreBoard is the persisted set of all skill scores.
type ScoreBoard struct {
	Version          int          `toml:"version" json:"version"`
	WorkingMemoryMax int          `toml:"working_memory_max" json:"working_memory_max"`
	Threshold        float64      `toml:"threshold" json:"threshold"`
	Scores           []SkillScore `toml:"scores" json:"scores"`
	ComputedAt       time.Time    `toml:"computed_at" json:"computed_at"`
}

// SkillInput provides the data needed to score a single skill.
type SkillInput struct {
	SkillRef string
	Affinity context.SkillContextAffinity
}

// Engine computes activation scores.
type Engine struct {
	config   Config
	eventLog *eventlog.EventLog
	feedback *feedback.Collector
}

// NewEngine creates a scoring engine.
func NewEngine(cfg Config, el *eventlog.EventLog, fb *feedback.Collector) *Engine {
	return &Engine{config: cfg, eventLog: el, feedback: fb}
}

// Compute calculates activation scores for all given skills.
func (e *Engine) Compute(skills []SkillInput, profile context.Profile, maxSlots int, threshold float64) (*ScoreBoard, error) {
	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour)

	// Get event stats
	stats, err := e.eventLog.Stats(since)
	if err != nil {
		return nil, err
	}
	statMap := map[string]eventlog.SkillStats{}
	for _, s := range stats {
		statMap[s.SkillRef] = s
	}

	board := &ScoreBoard{
		Version:          1,
		WorkingMemoryMax: maxSlots,
		Threshold:        threshold,
		ComputedAt:       now,
	}

	for _, skill := range skills {
		st := statMap[skill.SkillRef]

		r := ComputeRecency(st.LastAccess, e.config.HalfLifeDays)
		f := ComputeFrequency(st.EventCount)
		c := ComputeContextMatch(profile, skill.Affinity)

		avgRating, _ := e.feedback.AggregateRating(skill.SkillRef, since)
		fb := ComputeFeedbackBoost(avgRating)

		score := e.config.WeightRecency*r +
			e.config.WeightFreq*f +
			e.config.WeightContext*c +
			e.config.WeightFeedback*fb

		board.Scores = append(board.Scores, SkillScore{
			SkillRef:        skill.SkillRef,
			ActivationLevel: math.Round(score*1000) / 1000,
			Recency:         math.Round(r*1000) / 1000,
			Frequency:       math.Round(f*1000) / 1000,
			ContextMatch:    math.Round(c*1000) / 1000,
			FeedbackBoost:   math.Round(fb*1000) / 1000,
			LastComputed:    now,
		})
	}

	// Sort by activation level descending
	sort.Slice(board.Scores, func(i, j int) bool {
		return board.Scores[i].ActivationLevel > board.Scores[j].ActivationLevel
	})

	// Mark working memory membership
	for i := range board.Scores {
		board.Scores[i].InWorkingMemory = i < maxSlots && board.Scores[i].ActivationLevel >= threshold
	}

	return board, nil
}

// WorkingSet returns the skill refs that are in working memory.
func WorkingSet(board *ScoreBoard) []string {
	if board == nil {
		return nil
	}
	var refs []string
	for _, s := range board.Scores {
		if s.InWorkingMemory {
			refs = append(refs, s.SkillRef)
		}
	}
	return refs
}

// ComputeRecency calculates the recency component using exponential decay.
func ComputeRecency(lastAccess time.Time, halfLifeDays float64) float64 {
	if lastAccess.IsZero() {
		return 0
	}
	daysSince := time.Since(lastAccess).Hours() / 24
	if daysSince < 0 {
		daysSince = 0
	}
	lambda := math.Log(2) / halfLifeDays
	return math.Exp(-lambda * daysSince)
}

// ComputeFrequency calculates the frequency component using logarithmic scaling.
func ComputeFrequency(eventCount int) float64 {
	if eventCount <= 0 {
		return 0
	}
	v := math.Log(1+float64(eventCount)) / math.Log(101)
	if v > 1.0 {
		return 1.0
	}
	return v
}

// ComputeContextMatch calculates the context match between a profile and skill affinity.
func ComputeContextMatch(profile context.Profile, affinity context.SkillContextAffinity) float64 {
	if len(affinity.ProjectTypes) == 0 && len(affinity.Frameworks) == 0 && len(affinity.TaskSignals) == 0 {
		return 0.5 // neutral when no affinity declared
	}

	var scores []float64

	if len(affinity.ProjectTypes) > 0 {
		match := 0.0
		for _, pt := range affinity.ProjectTypes {
			if pt == profile.ProjectType {
				match = 1.0
				break
			}
		}
		scores = append(scores, match)
	}

	if len(affinity.Frameworks) > 0 {
		overlap := 0
		profileFW := toSet(profile.Frameworks)
		for _, fw := range affinity.Frameworks {
			if _, ok := profileFW[fw]; ok {
				overlap++
			}
		}
		scores = append(scores, float64(overlap)/float64(len(affinity.Frameworks)))
	}

	if len(affinity.TaskSignals) > 0 {
		overlap := 0
		profileTS := toSet(profile.TaskSignals)
		for _, ts := range affinity.TaskSignals {
			if _, ok := profileTS[ts]; ok {
				overlap++
			}
		}
		scores = append(scores, float64(overlap)/float64(len(affinity.TaskSignals)))
	}

	if len(scores) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}

// ComputeFeedbackBoost maps average rating [-1,+1] to [0,1].
func ComputeFeedbackBoost(avgRating float64) float64 {
	return (avgRating + 1.0) / 2.0
}

func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}
