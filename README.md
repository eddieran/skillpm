<p align="center">
  <strong>skillpm</strong><br>
  <em>Local-first skill package manager for AI agents</em>
</p>

<p align="center">
  <a href="https://github.com/eddieran/skillpm/actions/workflows/ci.yml"><img src="https://github.com/eddieran/skillpm/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/eddieran/skillpm/actions/workflows/security.yml"><img src="https://github.com/eddieran/skillpm/actions/workflows/security.yml/badge.svg" alt="Security"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-green.svg" alt="License"></a>
</p>

---

Install, upgrade, inject, and sync skills across agent runtimes (Codex, OpenClaw) with rollback-safe operations — no cloud control plane required.

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

# Inject into an agent runtime
skillpm inject my-repo/code-review --agent codex

# Browse trending skills
skillpm leaderboard
skillpm leaderboard --category security --limit 5

# Sync everything (plan first, then apply)
skillpm sync --dry-run
skillpm sync

# Diagnostics
skillpm doctor
```

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Sources** | Git repos, local dirs, or ClawHub registries that host skill packages |
| **Install** | Download + stage + atomic commit with automatic rollback on failure |
| **Inject** | Push installed skills into agent runtimes via adapter contracts |
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
