# Architecture

> [Docs Index](index.md)

## Package Map

```
cmd/skillpm/          CLI entry point (cobra commands)
internal/
├── app/              Use-case orchestration (Service facade)
├── adapter/          Runtime adapter implementations (file-based injection)
├── audit/            Append-only audit logging
├── config/           Schema, validation, persistence, project manifests
├── doctor/           Self-healing diagnostics (7 checks)
├── fsutil/           Shared filesystem helpers (atomic write, markers, copy)
├── harvest/          Agent-side skill discovery (SKILL.md walker)
├── importer/         Import local skills into managed state
├── installer/        Staging → atomic commit → rollback on failure
├── resolver/         Version resolution, ref parsing, dependency graph
│   ├── depgraph      DAG-based dependency resolution with topological sort
│   └── frontmatter   SKILL.md YAML frontmatter parser (deps extraction)
├── security/         Content scanning (3 rules), policy enforcement
├── selfupdate/       Signed CLI self-update workflow
├── source/           Source providers (git clone/fetch, ClawHub API)
├── store/            State, lockfile, and path I/O
└── sync/             Sync engine (plan / apply pipeline)
pkg/adapterapi/       Stable adapter contract (public API)
```

> Historical procedural-memory and scheduler documents remain in `docs/` for context, but those subsystems were removed in `v4.0.0` and are not part of the current runtime.

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
2. **Install** — `install` resolves the skill ref + version, expands transitive dependencies via DAG topological sort, downloads content to a staging area, runs security scanning, then atomically commits to `~/.skillpm/installed/`. On failure, the staging area is cleaned up (rollback).
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

### Dependency Resolution

```
install my-skill (has deps: [base-skill, util-skill])
┌──────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ Parse deps   │ ──→ │ Build DepGraph   │ ──→ │ TopologicalSort  │
│ from SKILL.md│     │ (DAG of edges)   │     │ (detect cycles)  │
│ frontmatter  │     │                  │     │ → install order   │
└──────────────┘     └──────────────────┘     └──────────────────┘
```

## State Files

| File | Location | Purpose |
|------|----------|---------|
| `config.toml` | `~/.skillpm/config.toml` | Global configuration |
| `state.toml` | `~/.skillpm/state.toml` | Installed skills + injection mappings |
| `skills.lock` | `.skillpm/skills.lock` | Pinned versions (project scope) |
| `injected.toml` | `~/.{agent}/skillpm/injected.toml` | Per-adapter injection state |
| `metadata.toml` | `~/.skillpm/installed/{name}@{ver}/` | Per-skill install metadata |
| `audit.log` | `~/.skillpm/audit.log` | Append-only audit trail for installs/uninstalls |

> **Note:** No new state files were added for dependency resolution. The existing types (e.g., `state.toml` entries, `metadata.toml`) now carry a `Deps []string` field to track declared dependencies.

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
