# skillpm 5-minute Quickstart

> [Docs Index](index.md)

Goal: complete one successful install + inject + sync flow with verifiable output.

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

## 2) Install one real skill

```bash
./bin/skillpm install https://github.com/anthropics/skills/tree/main/skills/skill-creator --force
```

Expected: install succeeds and state/lock metadata is written under `~/.skillpm/` (or `.skillpm/` in project scope).

> `skillpm install <URL>` auto-registers the backing repository as a source when needed.

## 3) Inspect installed state

```bash
./bin/skillpm list
./bin/skillpm status --json
```

Expected:
- `list` shows the installed skill
- `status --json` returns valid JSON with installed/source/adapter counts

> **Note**: All skills are scanned for dangerous content before installation. Critical findings always block. High and medium severity findings require `--force`. See `docs/troubleshooting.md` for details.

## 4) Inject into an agent

```bash
./bin/skillpm inject --agent codex anthropics_skills/skills/skill-creator
```

Expected: the skill is copied into the agent's native skills directory (for Codex: `~/.agents/skills/skill-creator/`).

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
./bin/skillpm install https://github.com/anthropics/skills/tree/main/skills/skill-creator --force

# Verify manifest and lockfile
cat .skillpm/skills.toml    # → lists "anthropics_skills/skills/skill-creator"
ls .skillpm/skills.lock     # → pinned version

# List shows scope
./bin/skillpm list
# → anthropics_skills/skills/skill-creator  (project)

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

## 9) Scaffold your own skill

Create a local skill directory you can edit and publish later:

```bash
./bin/skillpm create my-skill --template prompt
```

Expected: a new `my-skill/` directory containing `SKILL.md` appears in the current directory.

## Next Steps

- [CLI Reference](cli-reference.md) — full command documentation
- [Config Reference](config-reference.md) — customize `config.toml`
- [Supported Agents](agents.md) — all agent injection paths
- [Security Scanning](security-scanning.md) — scan rules and enforcement
- [Troubleshooting](troubleshooting.md) — common failures and fixes
- [Sync Contract v1](sync-contract-v1.md) — JSON schema for automation
