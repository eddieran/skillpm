# Changelog

All notable changes to this project are documented in this file.

## [1.1.0] - 2026-02-26

### Added
- **self-healing doctor**: `skillpm doctor` revamped from read-only diagnostics into an idempotent self-healing tool
  - 7 checks in dependency order: config, state, installed-dirs, injections, adapter-state, agent-skills, lockfile
  - config check auto-creates missing config and enables detected agent adapters
  - state check resets corrupt `state.toml`
  - installed-dirs check removes orphan directories and ghost state entries
  - injections check removes stale refs to uninstalled skills
  - adapter-state check re-syncs adapter's `injected.toml` with canonical state
  - agent-skills check restores missing skill files from `installed/`
  - lockfile check removes stale entries and backfills missing from state
  - new output format: `[status] name message` with indented fix details
  - JSON output via `--json` for automation
- exported `adapter.ExtractSkillName()` and `adapter.AgentSkillsDirForScope()` for downstream use

### Changed
- `--enable-detected` flag removed from `doctor` — behavior absorbed into config check
- doctor is now fully idempotent: run twice, second pass is all `[ok]`

## [1.0.0] - 2026-02-26

### Added
- **project-scoped skill management**: install, sync, and inject skills at the project level
  - `skillpm init` creates `.skillpm/skills.toml` manifest in the current directory
  - `skillpm install <ref>` auto-detects project scope when run inside an initialized project
  - `skillpm list` shows installed skills with scope annotations (project/global)
  - `skillpm sync` reads from project manifest for reproducible team onboarding
  - `skillpm uninstall` removes skills from both state and project manifest
  - `--scope global|project` flag on all commands for explicit scope control
  - project and global skills are fully isolated (separate state dirs, lock files, injected paths)
  - project manifest (`.skillpm/skills.toml`) and lockfile (`.skillpm/skills.lock`) are designed to be committed to git
  - agent injection uses project-local paths (e.g., `<project>/.claude/skills/`) instead of global `~/.claude/skills/`
  - scope auto-detection walks up from CWD to find nearest `.skillpm/skills.toml`
  - project sources and adapters are merged with global config (project overrides by name)
- `skillpm list --json` outputs structured JSON with skillRef, version, and scope fields
- **security scanning**: pre-install content scanner with 6 built-in rules
  - dangerous pattern detection (`rm -rf /`, `curl|bash`, reverse shells, credential reads)
  - prompt injection detection (instruction overrides, Unicode tricks, concealment)
  - file type checks (ELF/Mach-O/PE binaries, shared libraries)
  - size anomaly detection (oversized files/content)
  - entropy analysis (base64/hex blobs, obfuscated payloads)
  - network indicator detection (hardcoded IPs, URL shorteners, non-standard ports)
- configurable scan policy via `[security.scan]` in config (enabled, block_severity, disabled_rules)
- scanning runs in install, upgrade, and sync pipelines; critical findings cannot be bypassed even with `--force`
- audit logging for scan results
- sync JSON contract draft for beta consumers (`docs/sync-contract-v1.md`)
- beta readiness checklist (`docs/beta-readiness.md`)

## [0.1.0-beta] - 2026-02-17

### Added
- `sync --strict` mode to fail on unacceptable risk posture
- machine-readable sync risk classification and richer JSON summary fields
- sync command recommendations: `recommendedCommand` and `recommendedCommands`
- risk observability improvements:
  - risk level/status/breakdown
  - risk hotspot and progress hotspot
  - recommended risk agent

### Changed
- strict risk policy exit code semantics clarified:
  - `0` success
  - `2` strict risk failure
  - other non-zero for runtime/validation failures
- improved dry-run risk handling and recommendation flow

### Fixed
- CI formatting issue in `cmd/skillpm/main_test.go` (gofmt)
