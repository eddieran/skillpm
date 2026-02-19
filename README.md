<p align="center">
  <strong>skillpm</strong><br>
  <em>The universal package manager for AI agent skills</em>
</p>

<p align="center">
  <a href="https://github.com/eddieran/skillpm/actions/workflows/ci.yml"><img src="https://github.com/eddieran/skillpm/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/eddieran/skillpm/actions/workflows/security.yml"><img src="https://github.com/eddieran/skillpm/actions/workflows/security.yml/badge.svg" alt="Security"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-green.svg" alt="License"></a>
</p>

---

Install skills once, inject everywhere. **skillpm** gives you version-controlled skill management across Claude, Codex, Cursor, Gemini, and OpenClaw — with atomic installs, rollback-safe sync, and zero cloud dependencies.

## Install

```bash
brew tap eddieran/tap && brew install skillpm
```

Or build from source:

```bash
make build && ./bin/skillpm --help
```

## Usage

```bash
# Add a skill source
skillpm source add my-repo https://github.com/org/skills.git --kind git

# Search & install
skillpm search "code-review"
skillpm install my-repo/code-review

# Inject into agent runtimes
skillpm inject --agent claude
skillpm inject --agent codex
skillpm inject --agent cursor
skillpm inject --agent gemini

# Remove from an agent
skillpm remove --agent claude code-review

# Browse trending skills
skillpm leaderboard
skillpm leaderboard --category security --limit 5

# Sync everything (plan first, then apply)
skillpm sync --dry-run
skillpm sync

# Diagnostics
skillpm doctor
```

## Supported Agents

Skills are injected as folders into each agent's native `skills/` directory:

| Agent | Injection Path |
|-------|---------------|
| Claude | `~/.claude/skills/{name}/SKILL.md` |
| Codex | `~/.codex/skills/{name}/SKILL.md` |
| Cursor | `~/.cursor/skills/{name}/SKILL.md` |
| Gemini | `~/.gemini/skills/{name}/SKILL.md` |
| OpenClaw | `~/.openclaw/workspace/skills/{name}/SKILL.md` |

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Sources** | Git repos, local dirs, or ClawHub registries that host skill packages |
| **Install** | Download + stage + atomic commit with automatic rollback on failure |
| **Inject** | Push installed skills into agent-native `skills/` directories |
| **Sync** | Reconcile source updates → upgrades → re-injections in one pass |
| **Harvest** | Discover candidate skills from agent-side artifacts |
| **Leaderboard** | Browse trending skills ranked by popularity with category filtering |

## Architecture

```
cmd/skillpm/        CLI entry point
internal/
├── app/            Use-case orchestration
├── config/         Schema, validation, persistence
├── source/         Source providers (git / clawhub)
├── installer/      Staging → commit → rollback
├── adapter/        Runtime adapter implementations
├── sync/           Sync engine (plan / apply)
├── resolver/       Version resolution & ref parsing
├── store/          State & lockfile I/O
├── harvest/        Agent-side skill discovery
├── leaderboard/    Curated trending skill rankings
├── security/       Policy & path safety
└── doctor/         Diagnostics
pkg/adapterapi/     Stable adapter contract (public API)
```

## 🏆 Trending Skills

<!-- LEADERBOARD_START -->
```
 #   SKILL                    CATEGORY    ⬇ DOWNLOADS   ★ RATE  SOURCE
 ──────────────────────────────────────────────────────────────────────────────
 🥇  code-review              tool             12,480      4.9  clawhub
 🥈  auto-test-gen            agent            11,230      4.8  clawhub
 🥉  secret-scanner           security          9,870      4.8  community
 4   doc-writer               tool              9,540      4.7  clawhub
 5   dep-updater              workflow          8,920      4.7  clawhub
 6   perf-profiler            tool              8,310      4.6  community
 7   schema-migrator          data              7,890      4.6  clawhub
 8   ci-optimizer             workflow          7,650      4.5  clawhub
 9   api-fuzzer               security          7,420      4.5  community
 10  refactor-agent           agent             7,100      4.5  clawhub
```
<!-- LEADERBOARD_END -->

> Updated daily by [`update-leaderboard.yml`](./.github/workflows/update-leaderboard.yml) · Run `skillpm leaderboard` locally for the full list.

## Sync Strict Mode

Enforce risk posture in CI pipelines:

```bash
# PR gate — fail on planned risk
skillpm sync --strict --dry-run --json > sync-plan.json

# Deploy gate — enforce clean apply
skillpm sync --strict --json > sync-apply.json
```

Exit codes: `0` success · `2` strict policy failure · non-zero runtime error.

## Documentation

- [Quick Start](./docs/quickstart.md)
- [Sync Contract v1](./docs/sync-contract-v1.md)
- [Beta Readiness](./docs/beta-readiness.md)
- [Troubleshooting](./docs/troubleshooting.md)
- [Changelog](./CHANGELOG.md)

## Contributing

Issues and PRs welcome. See [CONTRIBUTING.md](./CONTRIBUTING.md) and [Code of Conduct](./CODE_OF_CONDUCT.md).

For vulnerability reporting → [SECURITY.md](./SECURITY.md).

## License

Apache-2.0 © skillpm contributors
