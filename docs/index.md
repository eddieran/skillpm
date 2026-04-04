# skillpm Documentation

| Page | Description |
|------|-------------|
| [Getting Started](getting-started.md) | Installation, first skill, project setup, agent tips |
| [Quick Start](quickstart.md) | 5-minute first install + sync |
| [Cookbook](cookbook.md) | Copy-paste recipes for CI, team workflows, and troubleshooting |
| [CLI Reference](cli-reference.md) | All commands, flags, exit codes |
| [Config Reference](config-reference.md) | `config.toml` schema |
| [Supported Agents](agents.md) | Injection paths & detection |
| [Security Scanning](security-scanning.md) | Rules, enforcement, policy |
| [Self-Healing Doctor](doctor.md) | 7 checks, auto-fix behavior |
| [Project-Scoped Skills](project-scoped-skills.md) | Team workflow with manifests |
| [Architecture](architecture.md) | Package map & data flow |
| [Sync Contract v1](sync-contract-v1.md) | JSON output schema for automation |
| [Troubleshooting](troubleshooting.md) | Common errors & fixes |
| [CI Policy](ci-policy.md) | CI status policy, pass rate, nightly E2E trends |
| [Beta Readiness Checklist](beta-readiness.md) | Release checklist for external beta quality gates |
| [Rollback Guide](rollback.md) | Recovery procedures for failed installs/syncs |

## Historical Notes

Some older design documents are retained for project history but do not describe the current `v4.x` runtime:

| Page | Status |
|------|--------|
| [Procedural Memory](procedural-memory.md) | Historical user guide for a feature removed in `v4.0.0` |
| [Procedural Memory RFC](procedural-memory-rfc.md) | Historical design RFC for a feature removed in `v4.0.0` |

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Sources** | Git repos, local dirs, or ClawHub registries that host skill packages |
| **Install** | Download + stage + atomic commit with automatic rollback on failure |
| **Inject** | Push installed skills into agent-native `skills/` directories |
| **Sync** | Reconcile source updates → upgrades → re-injections in one pass |
| **Scope** | Project-local (`.skillpm/skills.toml`) or global (`~/.skillpm/`) isolation |
| **Doctor** | Self-healing diagnostics — detects and auto-fixes environment drift |
| **Dependencies** | DAG-based skill dependency resolution with cycle detection |
| **Create** | Scaffold new skills from templates (default, prompt, script) |
| **Publish** | Publish skills to ClawHub registries with token auth |
| **Bundles** | Named groups of skills for batch installation |
