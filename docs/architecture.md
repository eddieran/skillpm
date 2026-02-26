# Architecture

> [Docs Index](index.md)

## Package Map

```
cmd/skillpm/          CLI entry point (cobra commands)
internal/
├── app/              Use-case orchestration (Service facade)
├── config/           Schema, validation, persistence, project manifests
├── source/           Source providers (git clone/fetch, ClawHub API)
├── installer/        Staging → atomic commit → rollback on failure
├── adapter/          Runtime adapter implementations (file-based injection)
├── sync/             Sync engine (plan / apply pipeline)
├── resolver/         Version resolution & ref parsing
├── store/            State & lockfile I/O (state.toml, skills.lock)
├── harvest/          Agent-side skill discovery (SKILL.md walker)
├── leaderboard/      Curated trending skill rankings
├── security/         Content scanning (6 rules), policy enforcement
├── doctor/           Self-healing diagnostics (8 checks)
├── memory/           Procedural memory facade (Service)
│   ├── eventlog/     Append-only JSONL event store
│   ├── observation/  Filesystem scanner for skill usage events
│   ├── context/      Project context detection (type, frameworks, tasks)
│   ├── scoring/      4-factor activation scoring engine
│   ├── feedback/     Explicit + implicit feedback collection
│   └── consolidation/ Periodic score recomputation & recommendations
└── audit/            Append-only audit logging
pkg/adapterapi/       Stable adapter contract (public API)
```

## Data Flow

```
source add/update         install                  inject              agent reads
┌─────────────┐     ┌──────────────┐     ┌──────────────────┐     ┌────────────┐
│ Git / ClawHub│ ──→ │ Resolve +    │ ──→ │ Copy to agent's  │ ──→ │ ~/.claude/  │
│ Registry     │     │ Stage + Scan │     │ skills/ directory │     │   skills/   │
│              │     │ + Commit     │     │ + Write state    │     │             │
└─────────────┘     └──────────────┘     └──────────────────┘     └────────────┘
                          │                       │
                          ▼                       ▼
                    ~/.skillpm/             ~/.claude/skillpm/
                    ├── state.toml         └── injected.toml
                    ├── installed/
                    └── skills.lock
```

### Pipeline Steps

1. **Source** — `source add` registers a Git repo or ClawHub registry. `source update` fetches latest metadata.
2. **Install** — `install` resolves the skill ref + version, downloads content to a staging area, runs security scanning, then atomically commits to `~/.skillpm/installed/`. On failure, the staging area is cleaned up (rollback).
3. **Inject** — `inject --agent <name>` copies installed skill folders into the agent's native `skills/` directory and records the mapping in `injected.toml`.
4. **Sync** — `sync` orchestrates all three steps: update sources → upgrade skills → re-inject into agents.

### Sync Pipeline Detail

```
sync
├── 1. Update all sources (source update)
├── 2. Upgrade installed skills to latest (install with new versions)
├── 3. Re-inject into all enabled agents
│   ├── Success → record in reinjected list
│   ├── Skipped → agent runtime unavailable
│   └── Failed → adapter error
└── 4. Report (human or JSON output)
```

### Memory Data Flow

```
observe                    scoring                    adaptive inject
┌──────────────┐     ┌──────────────────┐     ┌──────────────────────┐
│ Scan agent   │ ──→ │ 4-factor score:  │ ──→ │ Working-memory subset│
│ skill dirs   │     │  Recency  (0.35) │     │ injected into agent  │
│ for changes  │     │  Frequency(0.25) │     │ (top N above thresh) │
│              │     │  Context  (0.25) │     │                      │
│ → events.jsonl     │  Feedback (0.15) │     └──────────────────────┘
└──────────────┘     │ → scores.toml    │
       ▲             └──────────────────┘
       │                      ▲
  feedback                    │
  (+1/0/-1)            consolidation
  → feedback.jsonl     (periodic recompute)
```

## State Files

| File | Location | Purpose |
|------|----------|---------|
| `config.toml` | `~/.skillpm/config.toml` | Global configuration |
| `state.toml` | `~/.skillpm/state.toml` | Installed skills + injection mappings |
| `skills.lock` | `.skillpm/skills.lock` | Pinned versions (project scope) |
| `injected.toml` | `~/.{agent}/skillpm/injected.toml` | Per-adapter injection state |
| `metadata.toml` | `~/.skillpm/installed/{name}@{ver}/` | Per-skill install metadata |
| `events.jsonl` | `~/.skillpm/memory/events.jsonl` | Append-only usage event log |
| `feedback.jsonl` | `~/.skillpm/memory/feedback.jsonl` | Skill feedback signals |
| `scores.toml` | `~/.skillpm/memory/scores.toml` | Computed activation scores |
| `consolidation.toml` | `~/.skillpm/memory/consolidation.toml` | Consolidation state (last run, schedule) |
| `context_profile.toml` | `~/.skillpm/memory/context_profile.toml` | Detected project context |
| `last_scan.toml` | `~/.skillpm/memory/last_scan.toml` | Observer scan state (mtimes) |

## Public API (`pkg/adapterapi/`)

The `adapterapi` package defines the stable contract for adapter implementations:

```go
type Adapter interface {
    Probe(ctx) (ProbeResult, error)
    Inject(ctx, InjectRequest) (InjectResult, error)
    Remove(ctx, RemoveRequest) (RemoveResult, error)
    ListInjected(ctx, ListInjectedRequest) (ListInjectedResult, error)
    HarvestCandidates(ctx, HarvestRequest) (HarvestResult, error)
    ValidateEnvironment(ctx) (ValidateResult, error)
}
```

All adapters currently use the same `fileAdapter` implementation, which copies skill folders and manages `injected.toml` state. The interface allows future adapter types (e.g., API-based injection for cloud agents).
