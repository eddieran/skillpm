# skillpm Documentation

| Page | Description |
|------|-------------|
| [Quick Start](quickstart.md) | 5-minute first install + sync |
| [CLI Reference](cli-reference.md) | All commands, flags, exit codes |
| [Config Reference](config-reference.md) | `config.toml` schema |
| [Supported Agents](agents.md) | Injection paths & detection |
| [Security Scanning](security-scanning.md) | Rules, enforcement, policy |
| [Self-Healing Doctor](doctor.md) | 7 checks, auto-fix behavior |
| [Project-Scoped Skills](project-scoped-skills.md) | Team workflow with manifests |
| [Architecture](architecture.md) | Package map & data flow |
| [Sync Contract v1](sync-contract-v1.md) | JSON output schema for automation |
| [Troubleshooting](troubleshooting.md) | Common errors & fixes |
| [Beta Readiness](beta-readiness.md) | Release checklist |
| [Procedural Memory RFC](procedural-memory-rfc.md) | Agent self-adaptive memory design |

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Sources** | Git repos, local dirs, or ClawHub registries that host skill packages |
| **Install** | Download + stage + atomic commit with automatic rollback on failure |
| **Inject** | Push installed skills into agent-native `skills/` directories |
| **Sync** | Reconcile source updates → upgrades → re-injections in one pass |
| **Scope** | Project-local (`.skillpm/skills.toml`) or global (`~/.skillpm/`) isolation |
| **Doctor** | Self-healing diagnostics — detects and auto-fixes environment drift |
| **Leaderboard** | Browse trending skills ranked by popularity with category filtering |
