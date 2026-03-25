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

Install skills once, inject everywhere. **skillpm** gives you verified injection for Claude, Codex, Gemini, Copilot, OpenCode, Kiro, and OpenClaw, plus best-effort adapters for Antigravity, Cursor, and TRAE — with atomic installs, rollback-safe sync, project-scoped manifests, procedural memory, and zero cloud dependencies.

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
# Add a source
skillpm source add my-repo https://github.com/org/skills.git --kind git

# Search & install
skillpm search "code-review"
skillpm install my-repo/code-review

# Or install directly from any Git URL
skillpm install https://github.com/anthropics/skills/tree/main/skills/skill-creator --force

# Inject into agents
skillpm inject --agent claude
skillpm inject --agent codex

# Uninstall a skill
skillpm uninstall code-review

# Browse trending skills
skillpm leaderboard

# Sync everything
skillpm sync --dry-run
skillpm sync

# Procedural memory — skills adapt to your workflow
skillpm memory enable
skillpm memory observe
skillpm memory scores
skillpm memory working-set
skillpm inject --agent claude --adaptive

# Self-healing diagnostics
skillpm doctor

# Create a new skill from template
skillpm create my-skill --template prompt

# Publish a skill to ClawHub
skillpm publish ./my-skill --version 1.0.0

# Manage skill bundles
skillpm bundle create web-dev clawhub/react clawhub/typescript
skillpm bundle install web-dev
```

## Supported Agents

| Agent | Status | skillpm inject target | Docs |
|-------|--------|-----------------------|------|
| Claude Code | Verified | `~/.claude/skills/` | [code.claude.com](https://code.claude.com/docs/en/skills) |
| Codex | Verified | `~/.agents/skills/` | [developers.openai.com](https://developers.openai.com/codex/skills/) |
| Gemini CLI | Verified | `~/.gemini/skills/` | [geminicli.com](https://geminicli.com/docs/cli/skills/) |
| GitHub Copilot CLI | Verified | `~/.copilot/skills/` | [docs.github.com](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills) |
| OpenCode | Verified | `~/.config/opencode/skills/` | [opencode.ai](https://opencode.ai/docs/skills/) |
| Kiro | Verified | `~/.kiro/skills/` | [kiro.dev](https://kiro.dev/docs/skills/) |
| OpenClaw | Verified | `~/.openclaw/workspace/skills/` | [docs.openclaw.ai](https://docs.openclaw.ai/tools/skills) |
| Antigravity | Best-effort alias | `~/.gemini/skills/` | [geminicli.com](https://geminicli.com/docs/ide-integration/) |
| Cursor | Best-effort | `~/.cursor/skills/` | [cursor.com](https://cursor.com/docs/context/skills) |
| TRAE | Best-effort | `~/.trae/skills/` | [trae.ai](https://www.trae.ai/blog) |

VS Code uses the same skill contract as Copilot and is documented in the full matrix below.

> Full details: [Supported Agents](./docs/agents.md)

## Trending Skills

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

## Documentation

- [Docs Index](./docs/index.md) — navigation hub
- [Getting Started](./docs/getting-started.md) — installation, first skill, project setup
- [Quick Start](./docs/quickstart.md) — 5-minute first install
- [Cookbook](./docs/cookbook.md) — recipes for teams, CI/CD, multi-agent, memory
- [CLI Reference](./docs/cli-reference.md) — all commands, flags, exit codes
- [Config Reference](./docs/config-reference.md) — `config.toml` schema
- [Supported Agents](./docs/agents.md) — injection paths & detection
- [Procedural Memory](./docs/procedural-memory.md) — self-adaptive skill activation
- [Security Scanning](./docs/security-scanning.md) — rules, enforcement, policy
- [CI Policy](./docs/ci-policy.md) -- CI status policy and nightly E2E trends
- [Rollback Guide](./docs/rollback.md) -- recovery procedures for failed installs
- [Self-Healing Doctor](./docs/doctor.md) — 8 checks, auto-fix behavior
- [Project-Scoped Skills](./docs/project-scoped-skills.md) — team workflow
- [Architecture](./docs/architecture.md) — package map & data flow
- [Sync Contract v1](./docs/sync-contract-v1.md) — JSON output schema
- [Troubleshooting](./docs/troubleshooting.md) — common errors & fixes
- [Changelog](./CHANGELOG.md)

## Contributing

Issues and PRs welcome. See [CONTRIBUTING.md](./CONTRIBUTING.md) and [Code of Conduct](./CODE_OF_CONDUCT.md).

For vulnerability reporting → [SECURITY.md](./SECURITY.md).

## License

Apache-2.0 © skillpm contributors
