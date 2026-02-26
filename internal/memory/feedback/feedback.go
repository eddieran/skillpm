package feedback

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"skillpm/internal/memory/eventlog"
)

// FeedbackKind distinguishes explicit from implicit feedback.
type FeedbackKind string

const (
	FeedbackExplicit FeedbackKind = "explicit"
	FeedbackImplicit FeedbackKind = "implicit"
)

// Signal represents a single feedback data point.
type Signal struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	SkillRef  string            `json:"skill_ref"`
	Agent     string            `json:"agent"`
	Kind      FeedbackKind      `json:"kind"`
	Rating    float64           `json:"rating"` // [-1.0, +1.0]
	Reason    string            `json:"reason,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// Collector manages feedback signals.
type Collector struct {
	logPath string
	mu      sync.Mutex
}

// New creates a Collector writing to the given path.
func New(logPath string) *Collector {
	return &Collector{logPath: logPath}
}

// Rate records explicit user feedback. rating is 1-5, mapped to [-1, +1].
func (c *Collector) Rate(skillRef, agent string, rating int, reason string) error {
	if c == nil || c.logPath == "" {
		return nil
	}
	if rating < 1 || rating > 5 {
		return fmt.Errorf("MEM_FEEDBACK_RANGE: rating must be 1-5, got %d", rating)
	}
	// Map 1-5 to [-1.0, +1.0]: (rating - 3) / 2
	normalized := float64(rating-3) / 2.0
	sig := Signal{
		ID:        fmt.Sprintf("%d-rate-%s", time.Now().UnixNano(), skillRef),
		Timestamp: time.Now().UTC(),
		SkillRef:  skillRef,
		Agent:     agent,
		Kind:      FeedbackExplicit,
		Rating:    normalized,
		Reason:    reason,
	}
	return c.appendSignal(sig)
}

// InferFromEvents generates implicit feedback signals from usage patterns.
func (c *Collector) InferFromEvents(events []eventlog.UsageEvent, injectedAt map[string]time.Time) []Signal {
	now := time.Now().UTC()
	sevenDaysAgo := now.Add(-7 * 24 * time.Hour)
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)

	// Count accesses per skill in last 7 days
	recentAccess := map[string]int{}
	// Count distinct sessions (days) per skill
	sessionDays := map[string]map[string]struct{}{}
	for _, ev := range events {
		if ev.Kind == eventlog.EventAccess {
			if ev.Timestamp.After(sevenDaysAgo) {
				recentAccess[ev.SkillRef]++
			}
			day := ev.Timestamp.Format("2006-01-02")
			if sessionDays[ev.SkillRef] == nil {
				sessionDays[ev.SkillRef] = map[string]struct{}{}
			}
			sessionDays[ev.SkillRef][day] = struct{}{}
		}
	}

	var signals []Signal

	// Rule 1: frequent-use-positive — 5+ accesses in 7 days → +0.5
	for ref, count := range recentAccess {
		if count >= 5 {
			signals = append(signals, Signal{
				ID:        fmt.Sprintf("%d-implicit-freq-%s", now.UnixNano(), ref),
				Timestamp: now,
				SkillRef:  ref,
				Kind:      FeedbackImplicit,
				Rating:    0.5,
				Reason:    "frequent-use-positive",
			})
		}
	}

	// Rule 2: never-accessed-negative — injected 30+ days, 0 access events → -0.3
	for ref, injTime := range injectedAt {
		if injTime.Before(thirtyDaysAgo) && recentAccess[ref] == 0 {
			hasAnyAccess := false
			for _, ev := range events {
				if ev.SkillRef == ref && ev.Kind == eventlog.EventAccess {
					hasAnyAccess = true
					break
				}
			}
			if !hasAnyAccess {
				signals = append(signals, Signal{
					ID:        fmt.Sprintf("%d-implicit-never-%s", now.UnixNano(), ref),
					Timestamp: now,
					SkillRef:  ref,
					Kind:      FeedbackImplicit,
					Rating:    -0.3,
					Reason:    "never-accessed-negative",
				})
			}
		}
	}

	// Rule 3: session-retention-positive — accessed in 3+ separate days → +0.3
	for ref, days := range sessionDays {
		if len(days) >= 3 {
			signals = append(signals, Signal{
				ID:        fmt.Sprintf("%d-implicit-retain-%s", now.UnixNano(), ref),
				Timestamp: now,
				SkillRef:  ref,
				Kind:      FeedbackImplicit,
				Rating:    0.3,
				Reason:    "session-retention-positive",
			})
		}
	}

	return signals
}

// AggregateRating computes the average rating for a skill since a given time.
func (c *Collector) AggregateRating(skillRef string, since time.Time) (float64, error) {
	if c == nil || c.logPath == "" {
		return 0, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := os.Open(c.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("MEM_FEEDBACK_QUERY: %w", err)
	}
	defer f.Close()

	var sum float64
	var count int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var sig Signal
		if err := json.Unmarshal(line, &sig); err != nil {
			continue
		}
		if sig.SkillRef != skillRef {
			continue
		}
		if !since.IsZero() && sig.Timestamp.Before(since) {
			continue
		}
		sum += sig.Rating
		count++
	}
	if count == 0 {
		return 0, nil
	}
	return sum / float64(count), nil
}

// QuerySignals returns feedback signals matching the given criteria.
func (c *Collector) QuerySignals(since time.Time) ([]Signal, error) {
	if c == nil || c.logPath == "" {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := os.Open(c.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("MEM_FEEDBACK_QUERY: %w", err)
	}
	defer f.Close()
	var results []Signal
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var sig Signal
		if err := json.Unmarshal(scanner.Bytes(), &sig); err != nil {
			continue
		}
		if !since.IsZero() && sig.Timestamp.Before(since) {
			continue
		}
		results = append(results, sig)
	}
	return results, nil
}

func (c *Collector) appendSignal(sig Signal) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.logPath), 0o755); err != nil {
		return fmt.Errorf("MEM_FEEDBACK_RATE: %w", err)
	}
	f, err := os.OpenFile(c.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("MEM_FEEDBACK_RATE: %w", err)
	}
	defer f.Close()
	blob, err := json.Marshal(sig)
	if err != nil {
		return err
	}
	_, err = f.Write(append(blob, '\n'))
	return err
}
