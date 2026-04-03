# Self-Healing Doctor

> [Docs Index](index.md)

`skillpm doctor` detects and auto-fixes environment drift in a single idempotent pass — like `go mod tidy` for your skill setup.

## Usage

```bash
skillpm doctor           # human-readable output
skillpm doctor --json    # machine-readable output
```

## Design Philosophy

- **Idempotent**: run it twice and the second pass shows all `[ok]`.
- **Dependency-ordered**: checks run in sequence so earlier fixes feed later checks.
- **Non-destructive**: fixes only repair drift; they never delete intentional state.
- **Zero flags**: no configuration needed. Just run it.

## Checks

Doctor runs 7 checks in this order:

| # | Check | What It Fixes |
|---|-------|--------------|
| 1 | **config** | Creates missing `config.toml` with defaults. Auto-enables adapters for detected agents (e.g., if `~/.claude/` exists, enables the `claude` adapter). |
| 2 | **state** | Resets corrupt `state.toml` to an empty valid state. |
| 3 | **installed-dirs** | Removes orphan directories (on disk but not in state). Removes ghost state entries (in state but directory missing). |
| 4 | **injections** | Removes stale injection refs pointing to uninstalled skills. Removes empty agent entries. |
| 5 | **adapter-state** | Re-syncs each adapter's `injected.toml` with canonical state. If an adapter's list diverges from state, doctor re-injects to reconcile. |
| 6 | **agent-skills** | Restores missing skill files in agent directories (e.g., `~/.claude/skills/code-review/`). Copies from the installed cache. |
| 7 | **lockfile** | Removes stale lock entries (in lock but not in state). Backfills missing lock entries (in state but not in lock). |

## Status Values

Each check reports one of:

| Status | Meaning |
|--------|---------|
| `ok` | No issues found |
| `fixed` | Issue detected and automatically repaired |
| `warn` | Non-critical issue that couldn't be auto-fixed |
| `error` | Critical issue that couldn't be repaired |

## Example Output

```
[ok   ] config           config valid
[ok   ] state            state valid
[fixed] installed-dirs   installed dirs reconciled
  -> removed 1 orphan dir: unknown_skill@v0.0.0
[ok   ] injections       injection refs valid
[ok   ] adapter-state    adapter state synced
[ok   ] agent-skills     agent skill files present
[ok   ] lockfile         3 lock entries verified

done: 1 fixed
```

## JSON Output

```bash
skillpm doctor --json
```

```json
{
  "healthy": true,
  "scope": "global",
  "checks": [
    {
      "name": "config",
      "status": "ok",
      "message": "config valid"
    },
    {
      "name": "installed-dirs",
      "status": "fixed",
      "message": "installed dirs reconciled",
      "fix": "removed orphan dir: unknown_skill@v0.0.0"
    }
  ],
  "fixed": 1,
  "warnings": 0,
  "errors": 0
}
```

| Field | Type | Description |
|-------|------|-------------|
| `healthy` | bool | `false` if any check has `error` status |
| `scope` | string | `"global"` or `"project"` |
| `checks` | array | One entry per check |
| `checks[].name` | string | Check identifier |
| `checks[].status` | string | `ok`, `fixed`, `warn`, `error` |
| `checks[].message` | string | Human-readable summary |
| `checks[].fix` | string | Description of what was repaired (only if `fixed`) |
| `fixed` | int | Total checks with `fixed` status |
| `warnings` | int | Total checks with `warn` status |
| `errors` | int | Total checks with `error` status |

## When to Run Doctor

- **After first install** — creates config and enables detected agents.
- **After manually editing config or state files** — validates and repairs inconsistencies.
- **When injections seem stale** — reconciles adapter state with canonical state.
- **Before CI pipelines** — ensures the environment is clean.
- **When in doubt** — it's safe and idempotent.
