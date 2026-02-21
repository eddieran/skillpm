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

Install skills once, inject everywhere. **skillpm** gives you version-controlled skill management across Claude, Codex, Gemini, Copilot, Cursor, TRAE, OpenCode, Kiro, Antigravity, and OpenClaw — with atomic installs, rollback-safe sync, and zero cloud dependencies.

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
skillpm inject --agent gemini
skillpm inject --agent copilot
skillpm inject --agent trae
skillpm inject --agent opencode
skillpm inject --agent kiro
skillpm inject --agent cursor
skillpm inject --agent antigravity

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

Skills are injected as folders into each agent's native `skills/` directory.

### 🖥️ CLI Agents

| Agent | Injection Path | Docs |
|-------|---------------|------|
| Claude Code | `~/.claude/skills/{name}/` | [code.claude.com](https://code.claude.com/docs/en/skills) |
| Codex | `~/.codex/skills/{name}/` | [developers.openai.com](https://developers.openai.com/codex/skills/) |
| Gemini CLI | `~/.gemini/skills/{name}/` | [geminicli.com](https://geminicli.com/docs/cli/skills/) |
| GitHub Copilot CLI | `~/.copilot/skills/{name}/` | [docs.github.com](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills) |
| OpenCode | `~/.config/opencode/skills/{name}/` | [opencode.ai](https://opencode.ai/docs/skills/) |
| Kiro | `~/.kiro/skills/{name}/` | [kiro.dev](https://kiro.dev/docs/skills/) |
| OpenClaw | `~/.openclaw/workspace/skills/{name}/` | [docs.openclaw.ai](https://docs.openclaw.ai/tools/skills) |

### 🖱️ IDE / Desktop Agents

| Agent | Injection Path | Docs |
|-------|---------------|------|
| VS Code (Copilot) | `~/.copilot/skills/{name}/` | [code.visualstudio.com](https://code.visualstudio.com/docs/copilot/customization/agent-skills) |
| Cursor | `~/.cursor/skills/{name}/` | [cursor.com](https://cursor.com/docs/context/skills) |
| TRAE | `~/.trae/skills/{name}/` | [docs.trae.ai](https://docs.trae.ai/ide/skills) |
| Antigravity | `~/.gemini/skills/{name}/` | [antigravity.google](https://antigravity.google/docs/skills) |

> **Note**: VS Code + GitHub Copilot CLI share `~/.copilot/skills/`. Antigravity shares `~/.gemini/skills/` with Gemini CLI.

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Sources** | Git repos, local dirs, or ClawHub registries that host skill packages |
| **Install** | Download + stage + atomic commit with automatic rollback on failure |
| **Inject** | Push installed skills into agent-native `skills/` directories |
| **Sync** | Reconcile source updates → upgrades → re-injections in one pass |
| **Harvest** | Discover candidate skills from agent-side artifacts |
| **Leaderboard** | Browse trending skills ranked by popularity with category filtering |

## Security Scanning

Every skill is scanned before installation. The built-in scanner runs six rule categories against skill content and ancillary files:

| Rule | What it detects | Default severity |
|------|----------------|-----------------|
| Dangerous patterns | `rm -rf /`, `curl\|bash`, reverse shells, credential reads | Critical / High |
| Prompt injection | Instruction overrides, Unicode tricks, concealment | High |
| File type | ELF/Mach-O/PE binaries, shared libraries | High |
| Size anomaly | Oversized files or skill content | Medium |
| Entropy analysis | Base64/hex blobs, obfuscated payloads | High / Medium |
| Network indicators | Hardcoded IPs, URL shorteners, non-standard ports | High / Medium |

**Enforcement:**
- **Critical** findings always block installation (even with `--force`)
- **High** findings block by default
- **Medium** findings block unless `--force` is passed
- **Low/Info** findings are logged but never block

```bash
# Install is blocked when dangerous content is detected
skillpm install my-repo/suspicious-skill
# SEC_SCAN_BLOCKED: ...

# Medium findings can be bypassed with --force
skillpm install my-repo/admin-tool --force
```

Scanning is configurable in `~/.skillpm/config.toml`:

```toml
[security.scan]
enabled = true
block_severity = "high"
disabled_rules = []
```

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
├── security/       Policy, path safety & content scanning
└── doctor/         Diagnostics
pkg/adapterapi/     Stable adapter contract (public API)
```

## 🏆 Trending Skills

<!-- LEADERBOARD_START -->
```
  #    SKILL                      CATEGORY        ⬇ DLs  INSTALL COMMAND
  ─────────────────────────────────────────────────────────────────────────────────────
  🥇   steipete/code-review       tool           12,480  skillpm install clawhub/steipete/code-review
  🥈   testingshop/auto-test-gen  agent          11,230  skillpm install clawhub/testingshop/auto-test-gen
  🥉   secops/secret-scanner      security        9,870  skillpm install community/secops/secret-scanner
  4    docsify/doc-writer         tool            9,540  skillpm install clawhub/docsify/doc-writer
  5    semverbot/dep-updater      workflow        8,920  skillpm install clawhub/semverbot/dep-updater
  6    perfops/perf-profiler      tool            8,310  skillpm install community/perfops/perf-profiler
  7    datamaster/schema-migrator data            7,890  skillpm install clawhub/datamaster/schema-migrator
  8    ci-ninja/ci-optimizer      workflow        7,650  skillpm install clawhub/ci-ninja/ci-optimizer
  9    secops/api-fuzzer          security        7,420  skillpm install community/secops/api-fuzzer
  10   cleancode/refactor-agent   agent           7,100  skillpm install clawhub/cleancode/refactor-agent
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
