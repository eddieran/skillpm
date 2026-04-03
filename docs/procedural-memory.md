# Procedural Memory

> [Docs Index](index.md) · [CLI Reference](cli-reference.md) · [Config Reference](config-reference.md)

> Historical note: procedural memory was removed in `v4.0.0`. The commands,
> config, and file paths documented below are retained for archival context and
> migration reference only; they are not available in current releases.

Procedural memory is skillpm's cognitive subsystem for **self-adaptive skill activation**. It models how skills are used over time so the right skills surface at the right moment — without manual curation.

## Overview

When you install 30+ skills across 10 agents, not all skills are equally relevant at any given time. Procedural memory solves this by:

1. **Observing** which skills are accessed and when
2. **Understanding** the current project context
3. **Scoring** each skill's activation level using a 4-factor algorithm
4. **Collecting** explicit and implicit feedback
5. **Consolidating** scores periodically to strengthen frequently-used skills and decay unused ones
6. **Injecting** only the most relevant subset into agents

The result: agents see fewer, more relevant skills — reducing noise, improving response quality, and lowering token consumption.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Procedural Memory                           │
│                                                                 │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌──────────────┐   │
│  │Observa- │   │ Context │   │Feedback │   │Consolidation │   │
│  │tion     │   │ Engine  │   │Collector│   │   Engine     │   │
│  │         │   │         │   │         │   │              │   │
│  │ scans   │   │ detects │   │ +1/0/-1 │   │ periodic     │   │
│  │ agent   │──→│ project │──→│ ratings │──→│ recompute    │   │
│  │ dirs    │   │ type,   │   │ + event │   │ + promote/   │   │
│  │         │   │ frame-  │   │ signals │   │   demote     │   │
│  └────┬────┘   │ works,  │   └────┬────┘   └──────┬───────┘   │
│       │        │ tasks   │        │                │           │
│       ▼        └────┬────┘        ▼                ▼           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Activation Scoring Engine                   │   │
│  │                                                         │   │
│  │  Score = 0.35 × Recency                                │   │
│  │        + 0.25 × Frequency                              │   │
│  │        + 0.25 × ContextMatch                           │   │
│  │        + 0.15 × FeedbackBoost                          │   │
│  └────────────────────────┬────────────────────────────────┘   │
│                           │                                     │
│                           ▼                                     │
│                    ┌─────────────┐                              │
│                    │Working Set  │  top N skills above          │
│                    │(max 12)     │  threshold → inject          │
│                    └─────────────┘                              │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# 1. Enable the subsystem
skillpm memory enable

# 2. Record current skill usage
skillpm memory observe

# 3. Check which skills are most relevant
skillpm memory scores

# 4. Inject only the top skills into your agent
skillpm inject --agent claude --adaptive
```

## Enabling & Disabling

```bash
skillpm memory enable     # sets [memory].enabled = true in config.toml
skillpm memory disable    # sets [memory].enabled = false
```

All memory commands require an enabled subsystem. If disabled, commands return:

```
MEM_DISABLED: memory not enabled; run 'skillpm memory enable'
```

## The Six Layers

### Layer 1: Observation

The observer scans each enabled agent's skill directory for file modifications since the last scan. Changed files generate `access` events in the append-only event log.

```bash
skillpm memory observe               # scan all agents
skillpm memory events --since 7d     # query recent events
skillpm memory stats                 # per-skill usage summary
```

**Event log format** (`~/.skillpm/memory/events.jsonl`):

```jsonl
{"id":"evt_1","timestamp":"2026-02-26T10:00:00Z","skill_ref":"code-review","agent":"claude","kind":"access","scope":"global"}
```

Event kinds: `access`, `invoke`, `complete`, `error`, `feedback`.

### Layer 2: Context Engine

The context engine profiles the current working directory to understand the development environment:

```bash
skillpm memory context
```

It detects three dimensions:

| Dimension | Examples | Detection Method |
|-----------|----------|-----------------|
| **Project type** | `go`, `node`, `python`, `rust`, `java`, `ruby` | Marker files (`go.mod`, `package.json`, `Cargo.toml`, etc.) |
| **Frameworks** | `react`, `django`, `rails`, `spring`, `gin`, `express`, `next` | Config files and dependency declarations |
| **Task signals** | `feature`, `bugfix`, `refactor`, `test`, `docs` | Git branch name patterns and directory heuristics |

Context matching determines how well each skill fits the current environment.

### Layer 3: Activation Scoring

Every installed skill receives a composite activation score:

```
Score = 0.35 × Recency + 0.25 × Frequency + 0.25 × ContextMatch + 0.15 × FeedbackBoost
```

| Factor | Range | Algorithm |
|--------|-------|-----------|
| **Recency** | [0, 1] | Exponential decay: `e^(-λt)` where λ = ln(2) / half_life_days |
| **Frequency** | [0, 1] | Logarithmic: `ln(1 + count) / ln(101)`, capped at 1.0 |
| **ContextMatch** | [0, 1] | Weighted overlap of project type, frameworks, and task signals |
| **FeedbackBoost** | [0, 1] | Linear map: `(avg_rating + 1) / 2` where rating ∈ [-1, +1] |

```bash
skillpm memory scores                       # all skills
skillpm memory explain my-repo/code-review  # single skill breakdown
```

Example output:

```
SKILL                      ACTIVATION  RECENCY  FREQ   CONTEXT  FEEDBACK  IN-WM
my-repo/code-review        0.610       1.000    0.350  0.500    0.500     yes
my-repo/linter             0.425       0.500    0.200  0.750    0.500     yes
my-repo/deploy-helper      0.150       0.050    0.100  0.250    0.500     no
```

### Layer 4: Feedback

Explicit ratings let you boost or suppress skills:

```bash
skillpm memory rate my-repo/code-review +1    # thumbs up
skillpm memory rate my-repo/old-tool -1       # thumbs down
skillpm memory rate my-repo/neutral-tool 0    # reset to neutral
```

Ratings are stored in `~/.skillpm/memory/feedback.jsonl` and factor into the FeedbackBoost component. Aggregate ratings over the last 30 days are used in score computation.

```bash
skillpm memory feedback                # show all signals
skillpm memory feedback --since 7d     # recent signals only
```

### Layer 5: Consolidation

Consolidation periodically recomputes all scores, comparing against previous values to track transitions:

```bash
skillpm memory consolidate
```

Output includes:

| Metric | Description |
|--------|-------------|
| **Strengthened** | Skills whose score increased by >5% |
| **Decayed** | Skills whose score decreased by >5% |
| **Promoted** | Skills that entered working memory |
| **Demoted** | Skills that left working memory |

Recommendations for low-activation skills:

```bash
skillpm memory recommend
```

```
RECOMMENDATION  SKILL                SCORE   REASON
archive         my-repo/old-tool     0.08    very low activation (0.08)
```

Consolidation runs automatically on a 24-hour cycle. Use `skillpm memory consolidate` to trigger it manually.

### Layer 6: Adaptive Injection

Adaptive injection replaces the standard "inject all skills" behavior with "inject only the working-memory subset":

```bash
# Standard injection: all installed skills
skillpm inject --agent claude

# Adaptive injection: only working-memory skills
skillpm inject --agent claude --adaptive
```

To make adaptive injection the default:

```bash
skillpm memory set-adaptive on
```

When adaptive is the default, `skillpm inject --agent claude` automatically uses the working set. If memory is disabled or no scores exist, it falls back to injecting all skills.

## Configuration

All settings live under `[memory]` in `config.toml`:

```toml
[memory]
enabled = false           # toggle the subsystem
working_memory_max = 12   # max skills in working memory
threshold = 0.3           # minimum score to enter working memory
recency_half_life = "7d"  # decay speed: "3d" (fast), "7d" (default), "14d" (slow)
observe_on_sync = false   # auto-observe after sync
adaptive_inject = false   # use adaptive injection by default
```

| Setting | Effect |
|---------|--------|
| `working_memory_max = 12` | At most 12 skills can be in the working set |
| `threshold = 0.3` | Skills need ≥0.3 activation to qualify |
| `recency_half_life = "3d"` | Skills decay to 50% relevance in 3 days (aggressive) |
| `recency_half_life = "14d"` | Skills decay to 50% relevance in 14 days (conservative) |

## Storage Layout

All memory data lives under `~/.skillpm/memory/`:

```
~/.skillpm/memory/
├── events.jsonl           # append-only usage event log
├── feedback.jsonl         # explicit and implicit feedback signals
├── scores.toml            # computed activation scores (ScoreBoard)
├── consolidation.toml     # consolidation state (last_run, next_scheduled)
├── context_profile.toml   # detected project context
└── last_scan.toml         # observer scan timestamps
```

All `.jsonl` files are append-only. TOML state files use atomic write (tmp + rename) to prevent corruption.

## Data Lifecycle

```
Day 1: Install 20 skills, enable memory
         → all skills start at 0 activation (no events yet)

Day 2: Work on a Go project using code-review and linter skills
         → observe records access events
         → scores: code-review=0.6, linter=0.5, others=0.15

Day 5: Rate code-review +1, rate old-tool -1
         → feedback stored in feedback.jsonl
         → code-review boosted, old-tool suppressed

Day 7: Memory consolidation runs
         → code-review promoted to working memory
         → deploy-helper decayed (not used in 7 days)
         → old-tool recommended for archival

Day 8: Switch to a Python/Django project
         → context engine detects python + django
         → django-helper scores high on ContextMatch
         → adaptive inject gives agent: code-review, linter, django-helper
```

## Purging Memory

To reset all memory data and start fresh:

```bash
skillpm memory purge
```

This removes all files under `~/.skillpm/memory/` but does not disable the subsystem. New data will accumulate from the next observation.

## Error Codes

| Code | Description |
|------|-------------|
| `MEM_DISABLED` | Memory subsystem is not enabled |
| `MEM_INIT` | Failed to create memory directory |
| `MEM_EVENTLOG_APPEND` | Failed to write event to log |
| `MEM_EVENTLOG_QUERY` | Failed to read event log |
| `MEM_EVENTLOG_TRUNCATE` | Failed to truncate old events |
| `MEM_CONSOLIDATE_RUN` | Scoring computation failed |
| `MEM_CONSOLIDATE_SAVE` | Failed to persist scores |
| `MEM_CONSOLIDATE_STATE` | Failed to update consolidation state |
| `MEM_LAYOUT_STAT` | Memory directory stat failed |
| `MEM_LAYOUT_CREATE` | Memory directory creation failed |
| `MEM_LAYOUT_TYPE` | Memory path exists but is not a directory |

## Design Principles

- **Zero cloud dependencies** — all data is local files (JSONL + TOML)
- **Append-only logs** — event and feedback data is never rewritten, only appended
- **Atomic persistence** — TOML state files use tmp-file + rename pattern
- **Graceful degradation** — if memory is disabled or scores are empty, falls back to standard injection
- **No new dependencies** — built entirely on stdlib + existing `go-toml/v2`
- **CPU < 5% overhead** — benchmarked to add negligible latency to inject/sync operations
