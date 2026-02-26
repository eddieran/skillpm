# skillpm 5-minute Quickstart

> [Docs Index](index.md)

Goal: complete one successful local install + sync flow with verifiable output.

## 0) Prerequisites

- Go toolchain installed
- A writable local workspace
- `git` available in `PATH`

## 1) Build CLI

```bash
make build
./bin/skillpm --help
```

Expected: help output shows core commands (`source`, `install`, `sync`, ...).

## 2) Register a source

```bash
./bin/skillpm source add local https://example.com/skills.git --kind git
./bin/skillpm source list
```

Expected: `local` source appears in list.

## 3) Install one skill

```bash
./bin/skillpm install local/demo
```

Expected: install succeeds and lock/state metadata is written.

> **Note**: All skills are scanned for dangerous content before installation. If scanning detects critical or high severity issues, the install is blocked. Use `--force` to bypass medium-severity findings. See `docs/troubleshooting.md` for details.

## 4) Browse the leaderboard

```bash
./bin/skillpm leaderboard
./bin/skillpm leaderboard --category tool --limit 5
```

Expected: formatted table with rankings, download counts, and ratings.

## 5) Run dry-run sync plan

```bash
./bin/skillpm sync --dry-run --json > sync-plan.json
```

Expected:
- command exits `0`
- `sync-plan.json` is valid JSON
- includes summary fields documented in `docs/sync-contract-v1.md`

## 6) Enforce strict policy (optional gate)

```bash
./bin/skillpm sync --strict --dry-run --json > sync-plan-strict.json
```

Exit codes:
- `0`: acceptable risk posture
- `2`: strict policy failure
- other non-zero: runtime/validation failure

## 7) Project-scoped skills (team workflow)

```bash
# Initialize a project
mkdir myproject && cd myproject
./bin/skillpm init
# → creates .skillpm/skills.toml

# Install at project scope (auto-detected)
./bin/skillpm install local/demo

# Verify manifest and lockfile
cat .skillpm/skills.toml    # → lists "local/demo"
ls .skillpm/skills.lock     # → pinned version

# List shows scope
./bin/skillpm list
# → local/demo  (project)

# Team member sync (reads manifest + lockfile)
./bin/skillpm sync
```

Expected: skills installed to `.skillpm/installed/`, manifest and lockfile created. Global state at `~/.skillpm/` is unaffected.

## 8) Self-healing diagnostics

If anything feels off, run doctor — it auto-detects and fixes environment drift:

```bash
./bin/skillpm doctor
```

Expected: each check shows `[ok]` or `[fixed]` with a summary line.

## Next Steps

- [CLI Reference](cli-reference.md) — full command documentation
- [Config Reference](config-reference.md) — customize `config.toml`
- [Supported Agents](agents.md) — all agent injection paths
- [Security Scanning](security-scanning.md) — scan rules and enforcement
- [Troubleshooting](troubleshooting.md) — common failures and fixes
- [Sync Contract v1](sync-contract-v1.md) — JSON schema for automation
- [Beta Readiness](beta-readiness.md) — release checklist
