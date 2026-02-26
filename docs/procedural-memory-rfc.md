# RFC-001: Procedural Memory for AI Agent Skills

| Field | Value |
|-------|-------|
| **RFC** | 001 |
| **Title** | Procedural Memory for AI Agent Skills |
| **Status** | Draft |
| **Author** | Eddie Ran ([@eddieran](https://github.com/eddieran)) |
| **Created** | 2026-02-26 |
| **Updated** | 2026-02-26 |
| **Discussion** | [GitHub Issues](https://github.com/eddieran/skillpm/issues) |

---

## Table of Contents

- [1. Abstract](#1-abstract)
- [2. Motivation](#2-motivation)
- [3. Design Principles](#3-design-principles)
- [4. Architecture Overview](#4-architecture-overview)
- [5. Phase 1: Observation Layer](#5-phase-1-observation-layer)
- [6. Phase 2: Context Engine](#6-phase-2-context-engine)
- [7. Phase 3: Activation Scoring](#7-phase-3-activation-scoring)
- [8. Phase 4: Feedback Loop](#8-phase-4-feedback-loop)
- [9. Phase 5: Memory Consolidation](#9-phase-5-memory-consolidation)
- [10. Phase 6: Adaptive Injection](#10-phase-6-adaptive-injection)
- [11. Configuration](#11-configuration)
- [12. CLI Surface](#12-cli-surface)
- [13. Performance](#13-performance)
- [14. Privacy and Security](#14-privacy-and-security)
- [15. Backward Compatibility](#15-backward-compatibility)
- [16. Implementation Roadmap](#16-implementation-roadmap)
- [17. Alternatives Considered](#17-alternatives-considered)
- [18. Open Questions](#18-open-questions)
- [19. References](#19-references)

---

## 1. Abstract

This RFC proposes evolving skillpm from a static skill package manager into a **procedural memory system** for AI agents. Drawing from cognitive science, the system introduces working memory, activation scoring, context-aware retrieval, and feedback-driven consolidation — enabling an agent's skill set to strengthen with use, fade with disuse, and adapt to the developer's working context automatically.

The design adds six incremental capabilities on top of the existing skillpm architecture:

1. **Observation** — passively detect when agents access skills
2. **Context profiling** — identify project type, tech stack, and task signals
3. **Activation scoring** — rank skills by recency, frequency, context relevance, and feedback
4. **Feedback collection** — gather explicit ratings and implicit behavioral signals
5. **Memory consolidation** — periodically strengthen effective skills and decay unused ones
6. **Adaptive injection** — inject only the most relevant skill subset into each agent

All data stays local. No cloud services. No new native dependencies. Fully backward-compatible.

---

## 2. Motivation

### The Problem

skillpm today operates as a fire-and-forget system:

```
source ──→ install ──→ inject (all) ──→ agent reads (all) ──→ ???
```

Once a skill folder lands in an agent's directory (e.g., `~/.claude/skills/code-review/`), skillpm has **zero visibility** into what happens next. It cannot answer basic questions:

- Which skills does the developer actually use?
- Which skills are ignored or counterproductive?
- Does a code-review skill matter more in a Go project than a data pipeline?
- Should a testing skill activate when the developer is on a `fix/*` branch?

Every agent receives every installed skill regardless of context, creating noise. There is no learning, no adaptation, no feedback loop.

### The Insight: Procedural Memory

In cognitive science, **procedural memory** governs how we perform tasks — riding a bicycle, typing on a keyboard, debugging a failing test. Key properties:

| Property | Cognitive Science | skillpm Equivalent |
|----------|-------------------|-------------------|
| **Strengthening** | Skills improve with practice | Frequently used skills get higher activation |
| **Decay** | Unused skills fade over time | Dormant skills drop below activation threshold |
| **Context-dependent retrieval** | A pianist's fingers "remember" on a keyboard | A Go debugging skill activates in a Go project |
| **Working memory** | Limited active capacity (~7 items) | Top-N skills injected into agent |
| **Long-term storage** | Vast but dormant library | All installed skills, most not actively injected |
| **Consolidation** | Sleep strengthens important memories | Periodic scoring recalculation and pruning |

### Proposed Architecture

```
source ──→ install ──→ observe ──→ profile ──→ score ──→ inject (relevant subset) ──→ agent
                                                 ↑                                       │
                                                 └──── consolidate ◄──── feedback ◄──────┘
```

The agent's skill set becomes a living memory — ranked, context-aware, and feedback-driven.

---

## 3. Design Principles

| # | Principle | Rationale |
|---|-----------|-----------|
| 1 | **Local-first** | All memory data stays in `~/.skillpm/memory/`. Nothing leaves the machine. |
| 2 | **Opt-in** | Disabled by default. Existing workflows are unaffected until explicitly enabled. |
| 3 | **Backward-compatible** | All existing commands work identically. Memory is a purely additive layer. |
| 4 | **Lightweight** | Observation uses ~132 file stat calls. Zero impact on agent startup. |
| 5 | **No new native deps** | Flat files (JSONL + TOML) only. No SQLite, no cgo. Pure Go cross-compilation preserved. |
| 6 | **Privacy-preserving** | Events record skill refs and timestamps only. Never conversation content. |
| 7 | **Incrementally shippable** | Each phase delivers standalone value. Phase 1 alone provides useful visibility. |

---

## 4. Architecture Overview

### Package Structure

```
internal/memory/
├── memory.go                  Service facade — wires all subsystems
├── observation/
│   └── observation.go         File-system scanner, event emitter
├── eventlog/
│   └── eventlog.go            Append-only JSONL event store with query API
├── context/
│   └── context.go             Project type, framework, and task signal detection
├── scoring/
│   └── scoring.go             Activation algorithm and scoreboard persistence
├── feedback/
│   └── feedback.go            Explicit ratings and implicit signal inference
└── consolidation/
    └── consolidation.go       Periodic strengthen, decay, archive, and recommend
```

### Storage Layout

```
~/.skillpm/memory/
├── events.jsonl               Append-only usage event log
├── context.toml               Cached environment context profile
├── scores.toml                Activation scoreboard
├── feedback.jsonl             Append-only feedback signal log
└── consolidation.toml         Consolidation state and history
```

### Service Integration

The memory system is exposed as a single `Service` facade, wired into the existing `app.Service`:

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

func New(stateRoot string, cfg config.MemoryConfig) (*Service, error)
func (s *Service) IsEnabled() bool
```

```go
// internal/app/service.go
type Service struct {
    // ... existing fields ...
    Memory *memory.Service
}
```

When `memory.enabled = false` (the default), `New()` returns a no-op service. All existing code paths are unaffected.

---

## 5. Phase 1: Observation Layer

**Goal**: Record skill usage events without modifying any agent runtime.

### Detection Strategy

Since skillpm cannot modify agent runtimes (Claude Code, Codex CLI, etc.), usage must be inferred indirectly:

| Strategy | Mechanism | Tradeoffs |
|----------|-----------|-----------|
| **A. File-system timestamps** | Compare `SKILL.md` mtime/atime against last scan | Passive, zero config, works across all 11 agents. macOS disables atime by default; mtime serves as conservative proxy. |
| **B. Explicit event drop** | Skills opt-in via front-matter declaring observation hooks | Precise and agent-agnostic, but requires skill author participation. |
| **C. Session wrapper** | `skillpm observe -- <agent-command>` wraps invocation | Captures session-level metadata, but changes user workflow. |

**Recommended approach**: Start with **A** (passive) + **B** (opt-in). Add **C** as a future enhancement.

### Data Model

```go
// internal/memory/observation/observation.go

type EventKind string

const (
    EventAccess   EventKind = "access"    // Detected via file timestamp
    EventInvoke   EventKind = "invoke"    // Explicit invocation reported
    EventComplete EventKind = "complete"  // Task completed using skill
    EventError    EventKind = "error"     // Skill-related error observed
    EventFeedback EventKind = "feedback"  // User-provided feedback
)

type UsageEvent struct {
    ID        string            `json:"id"`          // ULID for ordering
    Timestamp time.Time         `json:"timestamp"`
    SkillRef  string            `json:"skill_ref"`   // e.g. "clawhub/code-review"
    Agent     string            `json:"agent"`        // e.g. "claude"
    Kind      EventKind         `json:"kind"`
    Scope     string            `json:"scope"`        // "global" or "project"
    Context   EventContext      `json:"context,omitempty"`
    Fields    map[string]string `json:"fields,omitempty"`
}

type EventContext struct {
    ProjectRoot string `json:"project_root,omitempty"`
    ProjectType string `json:"project_type,omitempty"`
    TaskType    string `json:"task_type,omitempty"`
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
    AgentCounts map[string]int  // events per agent
}
```

### Wire Format

Each line in `events.jsonl` is a self-contained JSON object:

```json
{"id":"01J5XK9M...","timestamp":"2026-02-26T10:00:00Z","skill_ref":"clawhub/code-review","agent":"claude","kind":"access","scope":"global","fields":{"method":"mtime"}}
```

### Scanning Algorithm

```go
func (o *Observer) ScanAgent(agent, skillsDir string, lastScan time.Time) []UsageEvent {
    var events []UsageEvent
    filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
        if d.Name() != "SKILL.md" {
            return nil
        }
        info, _ := os.Stat(path)
        accessTime := fileAccessTime(info) // platform-specific: atime on Linux, mtime on macOS
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

### Integration Points

| Hook | Location | Behavior |
|------|----------|----------|
| Post-inject | `app.Service.Inject()` | Record `EventInvoke` for each injected skill |
| Post-sync | `sync.Service.Run()` | Auto-trigger observation pass when `observe_on_sync = true` |
| Directory layout | `store.EnsureLayout()` | Create `memory/` subdirectory |
| Scheduler | launchd/systemd unit | Run `skillpm memory observe` alongside `skillpm sync` |

### Files Changed

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

## 6. Phase 2: Context Engine

**Goal**: Profile the developer's working environment to enable context-aware skill activation.

### Context Profile

```go
// internal/memory/context/context.go

type Profile struct {
    ProjectType string             `toml:"project_type"`  // "go", "python", "typescript", etc.
    Frameworks  []string           `toml:"frameworks"`    // ["cobra", "gin", "react", ...]
    TaskSignals []string           `toml:"task_signals"`  // ["testing", "debugging", "feature"]
    BuildSystem string             `toml:"build_system"`  // "make", "npm", "cargo"
    Languages   map[string]float64 `toml:"languages"`     // {"go": 0.8, "markdown": 0.15}
    DetectedAt  time.Time          `toml:"detected_at"`
}

type Engine struct{}

func (e *Engine) Detect(dir string) (Profile, error)
```

### Detection Methods

**Project type** — marker file scanning:

| Project Type | Marker Files |
|-------------|-------------|
| Go | `go.mod`, `go.sum` |
| Python | `pyproject.toml`, `setup.py`, `requirements.txt` |
| TypeScript | `tsconfig.json`, `package.json` |
| Rust | `Cargo.toml` |
| Java | `pom.xml`, `build.gradle` |
| Ruby | `Gemfile` |
| C# | `*.csproj`, `*.sln` |

**Task signals** — git branch and commit analysis:

| Signal | Detection |
|--------|-----------|
| `debugging` | Branch name contains `fix` or `bug` |
| `feature` | Branch name contains `feat` or `feature` |
| `testing` | Recent commits mention `test` or test files changed |
| `refactor` | Recent commits mention `refactor` or `cleanup` |

### Skill Context Affinity

Skills may declare affinity via optional `SKILL.md` front-matter:

```yaml
---
context:
  project_types: [go, python]
  task_signals: [testing, debugging]
  frameworks: [cobra]
---
```

This metadata is extracted at install time and stored alongside the skill:

```go
type SkillContextAffinity struct {
    ProjectTypes []string `toml:"project_types,omitempty"`
    TaskSignals  []string `toml:"task_signals,omitempty"`
    Frameworks   []string `toml:"frameworks,omitempty"`
}
```

### Files Changed

| Action | File |
|--------|------|
| Create | `internal/memory/context/context.go` |
| Modify | `internal/installer/installer.go` — parse front-matter at install time |

---

## 7. Phase 3: Activation Scoring

**Goal**: Assign each installed skill an activation score that determines its working memory membership.

### Cognitive Model Mapping

| Cognitive Concept | skillpm Implementation |
|-------------------|----------------------|
| **Working memory** | Skills actively injected into agents (limited capacity, default 12) |
| **Long-term memory** | All installed skills (unlimited) |
| **Activation level** | Score in `[0.0, 1.0]` determining injection priority |
| **Activation threshold** | Minimum score to enter working memory (default 0.3) |

### Scoring Formula

```
ActivationScore = w_R × Recency + w_F × Frequency + w_C × ContextMatch + w_FB × FeedbackBoost
```

| Component | Weight | Formula | Range |
|-----------|--------|---------|-------|
| **Recency** | 0.35 | `exp(-λ × daysSinceLastUse)` where `λ = ln(2) / halfLifeDays` | [0, 1] |
| **Frequency** | 0.25 | `log(1 + eventCount) / log(101)`, clamped | [0, 1] |
| **Context** | 0.25 | Jaccard-like overlap between current context and skill affinity | [0, 1] |
| **Feedback** | 0.15 | `(avgRating + 1) / 2`, mapping [-1, +1] to [0, 1] | [0, 1] |

Weights are configurable. Defaults sum to 1.0.

**Recency** uses exponential decay with a configurable half-life (default 7 days). A skill used 1 hour ago scores ~1.0; a skill unused for 21 days scores ~0.125.

**Context match** computes a normalized overlap:

```go
func computeContextMatch(ctx Profile, affinity SkillContextAffinity) float64 {
    score, checks := 0.0, 0.0

    if len(affinity.ProjectTypes) > 0 {
        checks++
        for _, pt := range affinity.ProjectTypes {
            if pt == ctx.ProjectType { score++; break }
        }
    }
    if len(affinity.Frameworks) > 0 {
        checks++
        score += float64(intersect(affinity.Frameworks, ctx.Frameworks)) /
                 float64(len(affinity.Frameworks))
    }
    if len(affinity.TaskSignals) > 0 {
        checks++
        score += float64(intersect(affinity.TaskSignals, ctx.TaskSignals)) /
                 float64(len(affinity.TaskSignals))
    }

    if checks == 0 { return 0.5 }  // no affinity declared → neutral
    return score / checks
}
```

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

### Example Output

```
$ skillpm memory scores

SKILL                       SCORE   R      F      C      FB    STATUS
clawhub/code-review         0.87    0.95   0.72   0.90   0.80  [active]
clawhub/auto-test-gen       0.65    0.80   0.45   0.70   0.50  [active]
community/secret-scanner    0.42    0.30   0.60   0.40   0.50  [active]
clawhub/doc-writer          0.28    0.10   0.30   0.50   0.50  [dormant]
community/perf-profiler     0.15    0.05   0.10   0.30   0.50  [dormant]

Working memory: 3/12 slots | Threshold: 0.30
```

```
$ skillpm memory explain clawhub/code-review

Skill: clawhub/code-review
Activation Score: 0.87

Components:
  Recency    (35%):  0.95  last used 2h ago
  Frequency  (25%):  0.72  18 events in last 30d
  Context    (25%):  0.90  matches: go project, cobra framework
  Feedback   (15%):  0.80  4 positive, 1 negative

Status: ACTIVE (in working memory)
Agents: claude (12 events), codex (6 events)
```

### Files Changed

| Action | File |
|--------|------|
| Create | `internal/memory/scoring/scoring.go` |

---

## 8. Phase 4: Feedback Loop

**Goal**: Collect explicit user ratings and implicit behavioral signals to refine activation scores.

### Feedback Model

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

### Explicit Feedback

Users rate skills on a 1-5 scale, mapped to [-1.0, +1.0]:

```
$ skillpm memory rate clawhub/code-review 5 --reason "catches real issues"
Recorded: clawhub/code-review = 5/5 (+1.0)

$ skillpm memory rate community/perf-profiler 2 --reason "too slow, inaccurate"
Recorded: community/perf-profiler = 2/5 (-0.5)
```

### Implicit Feedback Rules

| Rule | Condition | Inferred Rating |
|------|-----------|----------------|
| `frequent-use-positive` | 5+ accesses in 7 days | +0.5 |
| `never-accessed-negative` | Injected 30+ days ago, 0 access events | -0.3 |
| `rapid-removal-negative` | Injected then removed within 1 hour | -0.5 |
| `session-retention-positive` | Accessed in 3+ separate sessions | +0.3 |

### Files Changed

| Action | File |
|--------|------|
| Create | `internal/memory/feedback/feedback.go` |

---

## 9. Phase 5: Memory Consolidation

**Goal**: Periodically "garden" the memory — strengthen effective skills, decay unused ones, archive dormant skills, and surface actionable recommendations.

### Consolidation Pipeline

```
Consolidate():
  1. Recompute all activation scores with latest events + feedback
  2. Compare against previous scores to detect movement
  3. Identify promotions  (score crossed above threshold)
  4. Identify demotions   (score dropped below threshold)
  5. Apply natural time-based decay to all scores
  6. Generate recommendations for the user
  7. Archive stale events (>90 days) to compressed storage
  8. Persist new consolidation state
```

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
    Archived        []string `toml:"archived"`
    Promoted        []string `toml:"promoted"`
    Recommendations []string `toml:"recommendations"`
}

type Recommendation struct {
    Kind   string  `json:"kind"`    // "install", "remove", "promote", "archive"
    Skill  string  `json:"skill"`
    Reason string  `json:"reason"`
    Score  float64 `json:"score,omitempty"`
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
```

### Triggers

| Trigger | Mechanism |
|---------|-----------|
| **Scheduled** | Existing launchd/systemd scheduler extended to run consolidation after sync |
| **On-demand** | `skillpm memory consolidate` CLI command |
| **Opportunistic** | `inject` and `sync` check `ShouldRun()` and consolidate if overdue |

### Example Output

```
$ skillpm memory recommend

Recommendations:
  [install]  clawhub/auto-test-gen      you write Go tests frequently; high community rating
  [archive]  community/perf-profiler    unused for 45 days, score 0.08
  [promote]  clawhub/dep-updater        project has outdated deps, score rising to 0.42
```

### Doctor Integration

A new `checkMemoryHealth()` check is added to the existing doctor system:

- Verify `memory/` directory exists and is writable
- Validate `events.jsonl` contains well-formed JSONL
- Validate `scores.toml` is parseable
- Warn if consolidation is overdue (> 3x configured interval)

### Files Changed

| Action | File |
|--------|------|
| Create | `internal/memory/consolidation/consolidation.go` |
| Modify | `internal/doctor/doctor.go` — add `checkMemoryHealth()` |
| Modify | `internal/scheduler/manager.go` — extend plist/unit for post-sync consolidation |

---

## 10. Phase 6: Adaptive Injection

**Goal**: Replace "inject everything" with "inject the working set" — a context-relevant, memory-ranked subset of skills.

### Implementation

```go
// internal/app/service.go

func (s *Service) AdaptiveInject(ctx context.Context, agentName string) (adapterapi.InjectResult, error) {
    // 1. Detect current context
    profile, _ := s.Memory.Context.Detect(s.ProjectRoot)

    // 2. Compute activation scores with context
    installed, _ := s.ListInstalled()
    board, _ := s.Memory.Scoring.Compute(installed, profile)

    // 3. Select working set (top-N above threshold)
    workingSet := s.Memory.Scoring.WorkingSet(board)

    // 4. Inject only the working set
    return s.Inject(ctx, agentName, workingSet)
}
```

### CLI

```bash
# Context-aware injection (working memory only)
skillpm inject --agent claude --adaptive

# Traditional injection (all skills — unchanged default)
skillpm inject --agent claude --all

# Toggle adaptive mode globally
skillpm memory set-adaptive on
```

When `config.memory.adaptive_inject = true`, the `inject` command without explicit skill refs defaults to adaptive behavior. The `--all` flag explicitly overrides to legacy behavior.

### Sync Integration

When adaptive mode is enabled, the sync engine re-injects only the working set instead of all previously injected skills:

```go
// In sync.Service.Run():
if memoryEnabled && adaptiveInject {
    workingSet := scoring.WorkingSet(board)
    adp.Inject(ctx, InjectRequest{SkillRefs: workingSet})
}
```

### Files Changed

| Action | File |
|--------|------|
| Modify | `internal/app/service.go` — add `AdaptiveInject()` |
| Modify | `cmd/skillpm/main.go` — add `--adaptive` flag to `inject` |
| Modify | `internal/sync/sync.go` — use working set when adaptive mode is enabled |

---

## 11. Configuration

All memory settings live under a new `[memory]` section in `config.toml`:

```toml
[memory]
enabled = false               # Opt-in; must be explicitly enabled
working_memory_max = 12       # Maximum skills in the active working set
threshold = 0.3               # Minimum activation score for working memory
recency_half_life = "7d"      # Activation score halves every 7 days of disuse
observe_on_sync = true        # Auto-run observation pass after each sync
adaptive_inject = false       # Use working set for inject (Phase 6)
```

```go
// internal/config/types.go

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

## 12. CLI Surface

All new commands live under the `skillpm memory` subcommand:

| Command | Phase | Description |
|---------|-------|-------------|
| `memory enable` | 1 | Enable the memory system |
| `memory disable` | 1 | Disable the memory system |
| `memory observe [--agent NAME]` | 1 | Run one observation pass |
| `memory events [--since DURATION] [--skill REF]` | 1 | Query the event log |
| `memory stats [--since DURATION]` | 1 | Show aggregate usage statistics |
| `memory context [--dir PATH]` | 2 | Show detected environment context |
| `memory scores [--context]` | 3 | Show activation scores for all skills |
| `memory working-set` | 3 | Show the current working memory set |
| `memory explain SKILL_REF` | 3 | Breakdown of a skill's activation score |
| `memory rate SKILL_REF RATING [--reason TEXT]` | 4 | Record explicit feedback (1-5 scale) |
| `memory feedback [--since DURATION]` | 4 | View the feedback log |
| `memory consolidate [--dry-run]` | 5 | Run a consolidation pass |
| `memory recommend` | 5 | Show skill recommendations |
| `memory history [--since DURATION]` | 5 | View consolidation history |
| `memory set-adaptive [on\|off]` | 6 | Toggle adaptive injection mode |
| `memory purge` | 1 | Delete all memory data |

---

## 13. Performance

| Operation | Cost | Frequency |
|-----------|------|-----------|
| Observation scan | ~132 `os.Stat()` calls (12 skills x 11 agents) | Per sync or explicit observe |
| Score computation | Pure arithmetic on ~100 skills | Per consolidation or adaptive inject |
| Event log append | ~200 bytes/event, `O(1)` | Per observation event |
| Event log query | Sequential JSONL scan | Per score computation |
| Event log growth | ~100 events/day x 200B = **~7 MB/year** | Continuous |

**Agent startup is completely unaffected.** Skills remain static files in the agent's directory. The memory system only runs during explicit skillpm commands.

---

## 14. Privacy and Security

1. **All data is local.** Memory state lives exclusively in `~/.skillpm/memory/`. No data is ever transmitted to external services.
2. **No conversation content.** Events record only skill identifiers and timestamps. The content of agent conversations is never captured.
3. **User-controlled lifecycle.** `skillpm memory purge` irrecoverably deletes all memory data. `skillpm memory disable` stops all observation and scoring.
4. **No telemetry.** No analytics, no usage reporting, no phone-home behavior.
5. **Append-only logs.** Event and feedback logs are append-only JSONL files, preserving a complete audit trail.

---

## 15. Backward Compatibility

| Concern | Resolution |
|---------|------------|
| Existing commands | All work identically. Memory adds no new required flags. |
| Default behavior | `memory.enabled = false`. Zero behavior change until explicitly opted in. |
| Config schema | New `[memory]` section is optional. Missing section treated as defaults. |
| inject command | Default behavior unchanged. `--adaptive` is opt-in. |
| sync command | Default behavior unchanged. Adaptive mode requires explicit config. |
| Doctor | New memory health check runs only when memory is enabled. |

---

## 16. Implementation Roadmap

| Phase | Scope | Effort | Dependencies |
|-------|-------|--------|-------------|
| **1. Observation Layer** | Event log, mtime scanner, CLI commands, sync hooks | 2-3 weeks | None |
| **2. Context Engine** | Project detection, front-matter parsing, context CLI | 1-2 weeks | Phase 1 |
| **3. Activation Scoring** | Scoring algorithm, scoreboard, working memory selection | 2-3 weeks | Phases 1, 2 |
| **4. Feedback Loop** | Explicit ratings, implicit inference, feedback CLI | 1-2 weeks | Phase 1 |
| **5. Consolidation** | Periodic gardening, recommendations, scheduler + doctor integration | 2-3 weeks | Phases 3, 4 |
| **6. Adaptive Injection** | Smart inject, sync integration, adaptive toggle | 1-2 weeks | Phases 3, 5 |

**Total estimated effort: 9-15 weeks**

Each phase is independently shippable and delivers standalone value. Phase 1 alone provides useful visibility into skill usage patterns.

### Dependency Graph

```
Phase 1 (Observation)
├──→ Phase 2 (Context)
│    └──→ Phase 3 (Scoring) ──→ Phase 5 (Consolidation) ──→ Phase 6 (Adaptive Inject)
└──→ Phase 4 (Feedback) ──────→ Phase 5 (Consolidation)
```

---

## 17. Alternatives Considered

### SQLite for event storage

Using SQLite (via `go-sqlite3`) would provide indexed queries and better performance at scale. However, it introduces a cgo dependency that breaks pure-Go cross-compilation — a core skillpm design constraint. At the expected event volume (~100/day), JSONL line scanning performs well within acceptable latency. If needed, a pure-Go embedded store like [bbolt](https://github.com/etcd-io/bbolt) could be adopted later without cgo.

### Agent-side instrumentation

Modifying agent runtimes to emit usage telemetry directly would provide the most accurate data. However, skillpm supports 11 different agents from different vendors. Instrumenting each is impractical and would create tight coupling. The passive file-system observation approach works uniformly across all agents.

### Cloud-based memory sync

A cloud service could synchronize memory state across machines and aggregate community usage patterns. This was rejected for Phase 1 because it contradicts the local-first principle, adds operational complexity, and raises privacy concerns. It remains a candidate for future exploration.

### Real-time scoring (on every command)

Computing scores on every skillpm invocation was considered but rejected. Scores change slowly (driven by daily usage patterns, not per-second events), and on-demand computation would add noticeable latency to commands like `inject`. Periodic batch computation during consolidation is more appropriate.

---

## 18. Open Questions

1. **Event log compaction** — Should consolidation archive events older than 90 days to compressed storage, or retain everything indefinitely?

2. **Multi-machine memory** — Should memory state be portable across machines (e.g., committed to a git repo alongside project manifests)? Or is it strictly single-machine?

3. **Co-activation patterns** — Should the system track which skills are frequently used *together* to suggest skill bundles?

4. **Community memory** — Could anonymized, aggregated usage patterns improve the leaderboard rankings? What privacy guarantees would be needed?

5. **Per-project memory** — Should each project maintain its own activation scores, or overlay project context on a shared global memory?

---

## 19. References

- [skillpm Architecture](architecture.md) — package map and data flow
- [Config Reference](config-reference.md) — existing `config.toml` schema
- [Supported Agents](agents.md) — injection paths and detection logic
- [Self-Healing Doctor](doctor.md) — existing doctor checks (to extend with memory health)
- Anderson, J.R. (1983). *The Architecture of Cognition* — ACT theory of memory activation
- Baddeley, A. (2000). "The episodic buffer: a new component of working memory?" — working memory model
