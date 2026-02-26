# RFC: Procedural Memory System for skillpm

> **Status**: Draft
> **Author**: @eddieran
> **Date**: 2026-02-26
> **Tracking**: [Docs Index](index.md)

## Abstract

Evolve skillpm from a static "install → inject → forget" skill package manager into an **Agent Self-Adaptive Memory** system. After injection, skillpm will observe how agents use skills, profile the working context, score each skill's relevance, collect feedback, consolidate effective patterns, and adaptively inject only the most relevant subset — forming a living procedural memory that strengthens with use and fades with disuse.

---

## Motivation

### Current State

```
source → install → inject (all skills) → agent reads (all skills) → ???
```

skillpm today has **zero visibility** after injection. Once a skill folder lands in `~/.claude/skills/code-review/`, skillpm has no idea whether the agent ever reads it, finds it useful, or ignores it entirely. Every agent gets every skill, creating noise. There is no learning, no adaptation, no feedback.

### Proposed State

```
source → install → observe → profile → score → inject (relevant subset) → agent reads → feedback → learn
                                                  ↑                                                   │
                                                  └───────────── consolidate ◄──────────────────────────┘
```

The agent's skill set becomes a **procedural memory** — a ranked, context-aware, feedback-driven collection where:
- **Frequently used** skills stay readily available (working memory)
- **Unused** skills decay in priority (long-term storage)
- **Context-relevant** skills surface when the environment matches
- **Well-rated** skills get boosted; poorly-rated ones get demoted

---

## Design Principles

1. **Local-first**: All memory data stays in `~/.skillpm/memory/`. Nothing leaves the machine.
2. **Opt-in**: The entire memory system is disabled by default. Enable with `skillpm memory enable`.
3. **Backward-compatible**: All existing commands work identically. Memory is an additive layer.
4. **Lightweight**: Observation uses file-system stat calls (~132 stat operations). Zero impact on agent startup.
5. **No new native dependencies**: Flat files (JSONL + TOML) only. No SQLite, no cgo. Pure Go.
6. **Privacy-preserving**: Events record skill refs and timestamps only, never conversation content.

---

## Architecture Overview

### New Package Tree

```
internal/memory/
├── memory.go              # Top-level Service facade
├── observation/
│   └── observation.go     # File-system scanner, event emitter
├── eventlog/
│   └── eventlog.go        # Append-only JSONL event store + query
├── context/
│   └── context.go         # Project/task context detection
├── scoring/
│   └── scoring.go         # Activation algorithm + scoreboard
├── feedback/
│   └── feedback.go        # Explicit/implicit feedback collection
└── consolidation/
    └── consolidation.go   # Periodic strengthen/decay/recommend
```

### New Storage Layout

```
~/.skillpm/memory/
  events.jsonl             # Phase 1: usage event log (append-only)
  context.toml             # Phase 2: cached context profile
  scores.toml              # Phase 3: activation scoreboard
  feedback.jsonl           # Phase 4: feedback signals (append-only)
  consolidation.toml       # Phase 5: consolidation state
```

### Top-Level Service Facade

```go
// internal/memory/memory.go
package memory

type Service struct {
    Observer      *observation.Observer
    EventLog      *eventlog.EventLog
    Context       *context.Engine
    Scoring       *scoring.Engine
    Feedback      *feedback.Collector
    Consolidation *consolidation.Engine
    stateRoot     string
    enabled       bool
}

// New creates a memory service. Returns a no-op service if disabled.
func New(stateRoot string, cfg config.MemoryConfig) (*Service, error)
func (s *Service) IsEnabled() bool
```

Wired into the existing `app.Service` as a new `Memory` field:

```go
// internal/app/service.go
type Service struct {
    // ... existing fields ...
    Memory *memory.Service
}
```

### Config Extension

```toml
# ~/.skillpm/config.toml
[memory]
enabled = false               # opt-in
working_memory_max = 12       # max skills in active set
threshold = 0.3               # minimum activation score
recency_half_life = "7d"      # score halves every 7 days of disuse
observe_on_sync = true        # auto-observe during sync
adaptive_inject = false       # Phase 6: smart inject as default
```

```go
// internal/config/types.go — new struct
type MemoryConfig struct {
    Enabled          bool    `toml:"enabled"`
    WorkingMemoryMax int     `toml:"working_memory_max"`
    Threshold        float64 `toml:"threshold"`
    RecencyHalfLife  string  `toml:"recency_half_life"`
    ObserveOnSync    bool    `toml:"observe_on_sync"`
    AdaptiveInject   bool    `toml:"adaptive_inject"`
}
```

---

## Phase 1: Observation Layer

**Goal**: Record usage events without modifying any agent. Foundation for all subsequent phases.

### How to Detect Skill Usage Without Modifying Agents

Since we cannot modify agents themselves (Claude, Codex, etc.), we observe indirectly:

| Strategy | Mechanism | Pros | Cons |
|----------|-----------|------|------|
| **A. File-system mtime** | `os.Stat()` on each `SKILL.md`, compare against last scan | Zero config, passive, works across all agents | macOS disables `atime` by default; `mtime` is a conservative proxy |
| **B. Explicit event drop** | Skills opt-in via `memory.toml` declaring feedback hooks | Precise, agent-agnostic | Requires skill authors to participate |
| **C. Session wrapper** | `skillpm observe -- claude-code "fix bug"` wraps agent invocation | Captures session metadata | Requires user workflow change |

**Recommendation**: Start with **Strategy A** (passive mtime scan) + **Strategy B** (opt-in), add **Strategy C** later.

### Data Model

```go
// internal/memory/observation/observation.go

type EventKind string

const (
    EventAccess   EventKind = "access"    // File atime/mtime detected
    EventInvoke   EventKind = "invoke"    // Explicit invocation reported
    EventComplete EventKind = "complete"  // Task completed using skill
    EventError    EventKind = "error"     // Skill usage led to error
    EventFeedback EventKind = "feedback"  // Explicit user feedback
)

type UsageEvent struct {
    ID        string            `json:"id"`          // ULID for ordering
    Timestamp time.Time         `json:"timestamp"`
    SkillRef  string            `json:"skill_ref"`
    Agent     string            `json:"agent"`
    Kind      EventKind         `json:"kind"`
    Scope     string            `json:"scope"`       // "global" or "project"
    Context   EventContext      `json:"context,omitempty"`
    Fields    map[string]string `json:"fields,omitempty"`
}

type EventContext struct {
    ProjectRoot string `json:"project_root,omitempty"`
    ProjectType string `json:"project_type,omitempty"`  // enriched in Phase 2
    TaskType    string `json:"task_type,omitempty"`      // enriched in Phase 2
    WorkingDir  string `json:"working_dir,omitempty"`
}
```

### Event Log

```go
// internal/memory/eventlog/eventlog.go

type EventLog struct {
    path string
    mu   sync.Mutex
}

func (l *EventLog) Append(events ...observation.UsageEvent) error
func (l *EventLog) Query(f QueryFilter) ([]observation.UsageEvent, error)
func (l *EventLog) Stats(since time.Time) ([]SkillStats, error)

type QueryFilter struct {
    Since    time.Time
    Until    time.Time
    SkillRef string
    Agent    string
    Kind     observation.EventKind
    Limit    int
}

type SkillStats struct {
    SkillRef    string
    TotalEvents int
    LastAccess  time.Time
    FirstAccess time.Time
    AgentCounts map[string]int
}
```

### Storage Format

Each line in `~/.skillpm/memory/events.jsonl`:

```json
{"id":"01HXQ...","timestamp":"2026-02-26T10:00:00Z","skill_ref":"clawhub/code-review","agent":"claude","kind":"access","scope":"global","fields":{"method":"mtime"}}
```

### Scanning Algorithm

```go
func (o *Observer) ScanAgent(agent, skillsDir string, lastScan time.Time) []UsageEvent {
    var events []UsageEvent
    filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
        if d.Name() != "SKILL.md" { return nil }
        info, _ := os.Stat(path)
        accessTime := fileAccessTime(info) // platform-specific helper
        if accessTime.After(lastScan) {
            events = append(events, UsageEvent{
                SkillRef:  refFromPath(skillsDir, path),
                Agent:     agent,
                Kind:      EventAccess,
                Timestamp: accessTime,
                Fields:    map[string]string{"method": "mtime"},
            })
        }
        return nil
    })
    return events
}
```

### CLI Commands

```bash
skillpm memory observe [--agent <name>]      # Run one observation pass
skillpm memory events [--since 7d] [--skill <ref>]  # Query event log
skillpm memory stats [--since 30d]           # Aggregate usage statistics
```

### Integration Points

- `Inject()` in `app/service.go` — after injection, record an `EventInvoke` event
- `Sync.Run()` — after sync, auto-trigger observation pass when `observe_on_sync = true`
- `store.EnsureLayout()` — add `memory/` to directory creation list
- Scheduler plist/unit — extend to run `skillpm memory observe` alongside `skillpm sync`

### Files to Create/Modify

| Action | File |
|--------|------|
| Create | `internal/memory/memory.go` |
| Create | `internal/memory/observation/observation.go` |
| Create | `internal/memory/eventlog/eventlog.go` |
| Modify | `internal/store/paths.go` — add `MemoryRoot()`, `EventLogPath()` |
| Modify | `internal/store/state.go` — add `memory/` to `EnsureLayout()` |
| Modify | `internal/config/types.go` — add `MemoryConfig` |
| Modify | `internal/config/defaults.go` — add `Memory` defaults |
| Modify | `internal/app/service.go` — wire `memory.Service` |
| Modify | `cmd/skillpm/main.go` — add `memory` subcommand tree |

---

## Phase 2: Context Engine

**Goal**: Understand the working environment — project type, tech stack, task signals — to enrich events and enable context-aware scoring.

### Data Model

```go
// internal/memory/context/context.go

type Profile struct {
    ProjectType string             `toml:"project_type"`  // "go", "python", "typescript", etc.
    Frameworks  []string           `toml:"frameworks"`    // ["cobra", "gin", "react"]
    TaskSignals []string           `toml:"task_signals"`  // ["testing", "debugging", "feature"]
    BuildSystem string             `toml:"build_system"`  // "make", "npm", "cargo"
    Languages   map[string]float64 `toml:"languages"`     // {"go": 0.8, "md": 0.15}
    DetectedAt  time.Time          `toml:"detected_at"`
}

type Engine struct{}
func (e *Engine) Detect(dir string) (Profile, error)
```

### Detection Strategies

```go
var projectMarkers = map[string][]string{
    "go":         {"go.mod", "go.sum"},
    "python":     {"pyproject.toml", "setup.py", "requirements.txt"},
    "typescript": {"tsconfig.json", "package.json"},
    "rust":       {"Cargo.toml"},
    "java":       {"pom.xml", "build.gradle"},
    "ruby":       {"Gemfile"},
    "csharp":     {"*.csproj", "*.sln"},
}

// Task signals from git:
// - branch name contains "fix"/"bug" → "debugging"
// - branch name contains "feat"/"feature" → "feature"
// - recent commits mention "test" → "testing"
// - recent commits mention "refactor" → "refactor"
```

### Skill Context Affinity

Skills can optionally declare context affinity in `SKILL.md` front-matter:

```yaml
---
context:
  project_types: [go, python]
  task_signals: [testing, debugging]
  frameworks: [cobra]
---
```

Extracted during install, stored in `metadata.toml`:

```go
type SkillContextAffinity struct {
    ProjectTypes []string `toml:"project_types,omitempty"`
    TaskSignals  []string `toml:"task_signals,omitempty"`
    Frameworks   []string `toml:"frameworks,omitempty"`
}
```

### CLI

```bash
skillpm memory context                      # Show detected context for CWD
skillpm memory context --dir /path/to/project
```

### Files to Create/Modify

| Action | File |
|--------|------|
| Create | `internal/memory/context/context.go` |
| Modify | `internal/installer/installer.go` — extract front-matter during install |

---

## Phase 3: Scoring and Ranking

**Goal**: Assign each installed skill an **activation score** to determine working memory membership.

### Cognitive Model

| Concept | skillpm Equivalent |
|---------|-------------------|
| Working Memory | Skills actively injected into agents (limited capacity, default 12) |
| Long-Term Memory | All installed skills |
| Activation Level | Score `[0.0, 1.0]` determining injection priority |
| Activation Threshold | Score above which a skill enters working memory (default 0.3) |

### Scoring Algorithm

Four weighted components:

```
ActivationScore = w_R × Recency + w_F × Frequency + w_C × ContextMatch + w_FB × FeedbackBoost
```

| Component | Weight | Formula | Range |
|-----------|--------|---------|-------|
| **Recency** | 0.35 | `exp(-λ × daysSinceLastUse)` where `λ = ln(2) / halfLifeDays` | [0, 1] |
| **Frequency** | 0.25 | `log(1 + eventCount) / log(1 + 100)` clamped to 1.0 | [0, 1] |
| **ContextMatch** | 0.25 | Jaccard-like overlap between current context and skill affinity | [0, 1] |
| **FeedbackBoost** | 0.15 | `(avgRating + 1) / 2` mapping [-1, +1] to [0, 1] | [0, 1] |

Default weights sum to 1.0. Configurable in `[memory]`.

### Data Model

```go
// internal/memory/scoring/scoring.go

type SkillScore struct {
    SkillRef        string    `toml:"skill_ref"`
    ActivationLevel float64   `toml:"activation_level"`
    Recency         float64   `toml:"recency"`
    Frequency       float64   `toml:"frequency"`
    ContextMatch    float64   `toml:"context_match"`
    FeedbackBoost   float64   `toml:"feedback_boost"`
    InWorkingMemory bool      `toml:"in_working_memory"`
    LastComputed    time.Time `toml:"last_computed"`
}

type ScoreBoard struct {
    Version          int          `toml:"version"`
    WorkingMemoryMax int          `toml:"working_memory_max"`
    Threshold        float64      `toml:"threshold"`
    Scores           []SkillScore `toml:"scores"`
    ComputedAt       time.Time    `toml:"computed_at"`
}

type Engine struct {
    config   Config
    eventLog *eventlog.EventLog
    context  *context.Engine
}

func (e *Engine) Compute(installed []store.InstalledSkill, ctx context.Profile) (*ScoreBoard, error)
func (e *Engine) WorkingSet(board *ScoreBoard) []string
```

### Context Match Algorithm

```go
func computeContextMatch(ctx context.Profile, affinity SkillContextAffinity) float64 {
    score, checks := 0.0, 0.0
    if len(affinity.ProjectTypes) > 0 {
        checks++
        for _, pt := range affinity.ProjectTypes {
            if pt == ctx.ProjectType { score++; break }
        }
    }
    if len(affinity.Frameworks) > 0 {
        checks++
        score += float64(intersectionCount(affinity.Frameworks, ctx.Frameworks)) /
                 float64(len(affinity.Frameworks))
    }
    if len(affinity.TaskSignals) > 0 {
        checks++
        score += float64(intersectionCount(affinity.TaskSignals, ctx.TaskSignals)) /
                 float64(len(affinity.TaskSignals))
    }
    if checks == 0 { return 0.5 } // no affinity declared = neutral
    return score / checks
}
```

### CLI

```bash
skillpm memory scores [--context]            # Show activation scores
skillpm memory working-set                   # Show current working memory
skillpm memory explain <skill-ref>           # Breakdown why a skill has its score
```

Example output for `skillpm memory scores`:

```
SKILL                      SCORE  R     F     C     FB   STATUS
clawhub/code-review        0.87   0.95  0.72  0.90  0.80  [active]
clawhub/auto-test-gen      0.65   0.80  0.45  0.70  0.50  [active]
community/secret-scanner   0.42   0.30  0.60  0.40  0.50  [active]
clawhub/doc-writer         0.28   0.10  0.30  0.50  0.50  [dormant]
community/perf-profiler    0.15   0.05  0.10  0.30  0.50  [dormant]

Working memory: 3/12 slots used | Threshold: 0.30
```

Example output for `skillpm memory explain clawhub/code-review`:

```
Skill: clawhub/code-review
Activation Score: 0.87

Components:
  Recency    (35%): 0.95 — last used 2h ago
  Frequency  (25%): 0.72 — 18 events in last 30d
  Context    (25%): 0.90 — matches: go project, cobra framework
  Feedback   (15%): 0.80 — 4 positive, 1 negative

Status: ACTIVE (in working memory)
Agents: claude (12 events), codex (6 events)
```

### Storage

`~/.skillpm/memory/scores.toml`:

```toml
version = 1
working_memory_max = 12
threshold = 0.3
computed_at = 2026-02-26T10:00:00Z

[[scores]]
skill_ref = "clawhub/code-review"
activation_level = 0.87
recency = 0.95
frequency = 0.72
context_match = 0.90
feedback_boost = 0.80
in_working_memory = true
```

### Files to Create/Modify

| Action | File |
|--------|------|
| Create | `internal/memory/scoring/scoring.go` |

---

## Phase 4: Feedback Loop

**Goal**: Collect explicit user ratings and implicit behavioral signals to refine scoring.

### Data Model

```go
// internal/memory/feedback/feedback.go

type FeedbackKind string

const (
    FeedbackExplicit FeedbackKind = "explicit"  // User-provided rating
    FeedbackImplicit FeedbackKind = "implicit"  // Inferred from behavior
)

type Signal struct {
    ID        string            `json:"id"`
    Timestamp time.Time         `json:"timestamp"`
    SkillRef  string            `json:"skill_ref"`
    Agent     string            `json:"agent"`
    Kind      FeedbackKind      `json:"kind"`
    Rating    float64           `json:"rating"`     // [-1.0, +1.0]
    Reason    string            `json:"reason,omitempty"`
    Fields    map[string]string `json:"fields,omitempty"`
}

type Collector struct {
    logPath string
}

func (c *Collector) Rate(skillRef, agent string, rating float64, reason string) error
func (c *Collector) InferFromEvents(events []observation.UsageEvent) []Signal
func (c *Collector) AggregateRating(skillRef string, since time.Time) (float64, error)
```

### Implicit Feedback Rules

| Rule | Condition | Rating |
|------|-----------|--------|
| `frequent-use-positive` | 5+ accesses in 7 days | +0.5 |
| `never-accessed-negative` | Injected 30+ days, 0 access events | -0.3 |
| `rapid-removal-negative` | Injected then removed within 1 hour | -0.5 |
| `session-retention-positive` | Accessed in 3+ separate sessions | +0.3 |

### CLI

```bash
skillpm memory rate <skill-ref> <1-5> [--reason "..."]   # Explicit rating
skillpm memory feedback [--since 7d]                     # View feedback log
```

Example:

```bash
$ skillpm memory rate clawhub/code-review 5 --reason "catches real issues"
Recorded: clawhub/code-review = 5/5 (mapped to +1.0)

$ skillpm memory rate community/perf-profiler 2 --reason "too slow, inaccurate"
Recorded: community/perf-profiler = 2/5 (mapped to -0.5)
```

### Files to Create/Modify

| Action | File |
|--------|------|
| Create | `internal/memory/feedback/feedback.go` |

---

## Phase 5: Consolidation Engine

**Goal**: Periodically "garden" the memory — strengthen effective skills, decay unused ones, surface recommendations.

### Data Model

```go
// internal/memory/consolidation/consolidation.go

type ConsolidationState struct {
    Version       int       `toml:"version"`
    LastRun       time.Time `toml:"last_run"`
    NextScheduled time.Time `toml:"next_scheduled"`
    Interval      string    `toml:"interval"`  // default: "24h"
    Stats         RunStats  `toml:"stats"`
}

type RunStats struct {
    SkillsEvaluated int      `toml:"skills_evaluated"`
    Strengthened    []string `toml:"strengthened"`
    Decayed         []string `toml:"decayed"`
    Archived        []string `toml:"archived"`       // removed from working memory
    Promoted        []string `toml:"promoted"`        // added to working memory
    Recommendations []string `toml:"recommendations"`
}

type Engine struct {
    stateRoot string
    scoring   *scoring.Engine
    feedback  *feedback.Collector
    eventLog  *eventlog.EventLog
}

func (e *Engine) Consolidate(ctx context.Context) (*RunStats, error)
func (e *Engine) ShouldRun() (bool, error)
func (e *Engine) Recommend() ([]Recommendation, error)

type Recommendation struct {
    Kind   string  `json:"kind"`    // "install", "remove", "promote", "archive"
    Skill  string  `json:"skill"`
    Reason string  `json:"reason"`
    Score  float64 `json:"score,omitempty"`
}
```

### Consolidation Pipeline

```
Consolidate():
  1. Recompute all scores with latest events + feedback
  2. Compare against previous scores
  3. Identify promotions (below threshold → above)
  4. Identify demotions (above threshold → below)
  5. Apply natural time-based decay to all scores
  6. Generate recommendations
  7. Persist new state + archive old events (>90d → compressed archive)
```

### Triggers

1. **Scheduler-driven**: Extend existing launchd/systemd unit — `skillpm sync && skillpm memory consolidate`
2. **On-demand**: `skillpm memory consolidate`
3. **Opportunistic**: During `inject` or `sync`, check `ShouldRun()` and consolidate if overdue

### CLI

```bash
skillpm memory consolidate [--dry-run]       # Run consolidation
skillpm memory recommend                     # Show recommendations
skillpm memory history [--since 30d]         # View consolidation history
```

Example `skillpm memory recommend`:

```
Recommendations:
  [install]  clawhub/auto-test-gen     — you write Go tests frequently, high community rating
  [archive]  community/perf-profiler   — unused for 45 days, score 0.08
  [promote]  clawhub/dep-updater       — your project has outdated deps, score rising to 0.42
```

### Doctor Integration

New check `checkMemoryHealth()` added to `internal/doctor/doctor.go`:
- Verify `memory/` directory exists
- Check `events.jsonl` is not corrupted (valid JSONL)
- Check `scores.toml` is parseable
- Verify consolidation is not overdue (warn if > 3× interval)

### Files to Create/Modify

| Action | File |
|--------|------|
| Create | `internal/memory/consolidation/consolidation.go` |
| Modify | `internal/doctor/doctor.go` — add `checkMemoryHealth()` |
| Modify | `internal/scheduler/manager.go` — extend plist/unit for consolidation |

---

## Phase 6: Adaptive Injection

**Goal**: Transform `skillpm inject` from "inject everything" to "inject the working set" — a context-relevant subset.

### New Method

```go
// internal/app/service.go

func (s *Service) AdaptiveInject(ctx context.Context, agentName string) (adapterapi.InjectResult, error) {
    // 1. Detect current context
    profile, _ := s.Memory.Context.Detect(s.ProjectRoot)

    // 2. Compute scores
    installed, _ := s.ListInstalled()
    board, _ := s.Memory.Scoring.Compute(installed, profile)

    // 3. Get working set
    workingSet := s.Memory.Scoring.WorkingSet(board)

    // 4. Inject only the working set
    return s.Inject(ctx, agentName, workingSet)
}
```

### CLI Changes

```go
// In newInjectCmd — add flag:
var adaptive bool
cmd.Flags().BoolVar(&adaptive, "adaptive", false, "inject context-relevant subset based on memory scores")
```

When `--adaptive` is set or `config.Memory.AdaptiveInject == true`, call `AdaptiveInject` instead of `Inject`.

### Sync Integration

When adaptive mode is enabled, `sync` re-injects only the working set:

```go
// In sync.Service.Run():
if memoryEnabled && adaptiveInject {
    workingSet := scoring.WorkingSet(board)
    adp.Inject(ctx, InjectRequest{SkillRefs: workingSet})
}
```

### CLI

```bash
skillpm inject --agent claude --adaptive     # Inject context-relevant subset
skillpm inject --agent claude --all          # Inject everything (existing behavior)
skillpm memory set-adaptive [on|off]         # Toggle adaptive mode globally
```

### Files to Create/Modify

| Action | File |
|--------|------|
| Modify | `internal/app/service.go` — add `AdaptiveInject()` |
| Modify | `cmd/skillpm/main.go` — add `--adaptive` flag to inject |
| Modify | `internal/sync/sync.go` — use working set when adaptive |

---

## Performance Analysis

| Operation | Cost | Frequency |
|-----------|------|-----------|
| `ScanAgent()` | ~132 `os.Stat()` calls (12 skills × 11 agents) | On sync / explicit observe |
| Score computation | Pure arithmetic on ~100 skills | On consolidation / adaptive inject |
| Event log append | ~200 bytes per event | On each observation |
| Event log query | Line scan of JSONL | On score computation |
| Event log growth | ~100 events/day × 200B = 20KB/day ≈ 7MB/year | Continuous |

Agent startup is completely unaffected — skills remain static files in the agent's directory.

---

## Privacy

1. All data in `~/.skillpm/memory/` — fully local, never transmitted.
2. Events record only skill refs and timestamps, never conversation content.
3. `skillpm memory purge` wipes all memory data.
4. No telemetry, no analytics reporting, no cloud sync.

---

## Backward Compatibility

1. `config.Memory.Enabled` defaults to `false` — zero behavior change for existing users.
2. All existing commands work identically when memory is disabled.
3. The `memory` CLI subcommand is always available but reports "memory system not enabled" when disabled.
4. Enable with `skillpm memory enable` or set `[memory] enabled = true` in config.
5. `--adaptive` on inject is opt-in; default inject behavior unchanged.

---

## Complete CLI Surface (New)

```
skillpm memory enable                        # Enable the memory system
skillpm memory disable                       # Disable the memory system
skillpm memory observe [--agent <name>]      # Run observation pass
skillpm memory events [--since] [--skill]    # Query event log
skillpm memory stats [--since]               # Aggregate statistics
skillpm memory context [--dir]               # Show detected context
skillpm memory scores [--context]            # Show activation scores
skillpm memory working-set                   # Show current working memory
skillpm memory explain <skill-ref>           # Score breakdown
skillpm memory rate <skill> <1-5> [--reason] # Explicit feedback
skillpm memory feedback [--since]            # View feedback log
skillpm memory consolidate [--dry-run]       # Run consolidation
skillpm memory recommend                     # Show recommendations
skillpm memory history [--since]             # Consolidation history
skillpm memory set-adaptive [on|off]         # Toggle adaptive inject
skillpm memory purge                         # Wipe all memory data
```

---

## Implementation Roadmap

| Phase | Scope | Estimated Effort | Depends On |
|-------|-------|-----------------|------------|
| **1. Observation Layer** | eventlog, mtime scanner, CLI, sync hooks | 2-3 weeks | — |
| **2. Context Engine** | project detection, front-matter parsing | 1-2 weeks | Phase 1 |
| **3. Scoring & Ranking** | algorithm, scoreboard, working memory | 2-3 weeks | Phase 1, 2 |
| **4. Feedback Loop** | explicit + implicit feedback, integration | 1-2 weeks | Phase 1 |
| **5. Consolidation** | gardening, recommendations, scheduler | 2-3 weeks | Phase 3, 4 |
| **6. Adaptive Injection** | smart inject, sync integration | 1-2 weeks | Phase 3, 5 |

**Total: 9-15 weeks**

Each phase is independently shippable. Phase 1 alone provides valuable usage visibility.

---

## Open Questions

1. **Event log compaction**: Should consolidation archive events older than 90 days? Or keep everything?
2. **Multi-machine sync**: Should memory state be sync-able across machines (e.g., via git)? Or strictly single-machine?
3. **Skill composition**: Should the memory system track which skills are used _together_ (co-activation patterns)?
4. **Community memory**: Should anonymized, aggregated usage patterns feed back into the leaderboard rankings?
5. **Per-project memory**: Should each project have its own memory state, or share global memory with project context overlays?

---

## References

- [skillpm Architecture](architecture.md) — current package map and data flow
- [Config Reference](config-reference.md) — existing config schema
- [Supported Agents](agents.md) — injection paths and detection logic
- [Self-Healing Doctor](doctor.md) — existing doctor checks (to extend)
