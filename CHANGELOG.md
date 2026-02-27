# Changelog

All notable changes to this project are documented in this file.

## [2.1.1] - 2026-02-27

### Changed
- **shared `fsutil` package**: consolidated 9 duplicate atomic write implementations into `fsutil.AtomicWrite` with consistent error handling (tmp cleanup on rename failure)
- **shared managed marker constants**: unified `<!-- skillpm:managed -->` marker across bridge, rules, and doctor into `fsutil.ManagedMarkerPrefix` / `fsutil.ManagedMarkerSimple` / `fsutil.IsManagedFile()`
- **bridge WriteRankings wired**: `memory consolidate` now writes `skillpm-rankings.md` to Claude Code auto memory after consolidation
- **N+1 LoadState fix**: `syncRulesForSkills` loads state once instead of per-skill
- **Gemini parser fix**: dual-parse now uses mutual exclusion (session object vs message array), preventing duplicate hits

### Removed
- dead `LastScanPath()` from `store/paths.go`
- dead `ObserveOnSync` config field (defined but never referenced in code)
- 9 private `atomicWrite` / inline atomic write patterns replaced by shared utility

## [2.1.0] - 2026-02-27

### Added
- **observation v2**: session transcript parsing replaces unreliable mtime scanning
  - `ClaudeParser`: parses Claude Code JSONL transcripts, detects `Skill` tool invocations and `Read` of SKILL.md files
  - `CodexParser`: parses Codex CLI JSONL sessions for function calls referencing skills
  - `GeminiParser`: parses Gemini CLI JSON chat files for skill function calls
  - `OpenCodeParser`: parses OpenCode individual message JSON files for tool calls
  - `MtimeScanner`: retained as fallback for agents without observable transcripts (Cursor, Copilot, TRAE, Kiro)
  - `SessionParser` interface for pluggable agent support
  - `ScanState` TOML: per-file byte offset tracking for incremental JSONL parsing
  - `SkillIndex`: maps skill directory names to full SkillRef strings
  - incremental parsing: JSONL files use byte offset seek; JSON files use mtime comparison
  - deduplication by SessionID + SkillDirName + Kind
  - 60-day GC for stale scan state entries
- **rules injection** (opt-in, Claude Code only): skill guidance becomes path-scoped
  - `rules/extract.go`: extracts YAML frontmatter `rules.paths` from SKILL.md, with keyword-based auto-detection fallback
  - `rules/rules.go`: `Engine` with `Sync`, `Cleanup`, `ListManaged`, `Generate` methods
  - writes to `~/.claude/rules/skillpm/<skill-name>.md` (dedicated subdirectory, never touches user rules)
  - `<!-- skillpm:managed ref=... checksum=... -->` ownership marker for safe updates
  - atomic writes (tmp + rename)
  - auto-triggered on `inject --agent claude` and `remove --agent claude`
  - config: `rules_injection = false` (opt-in), `rules_scope` (global/project)
- **memory bridge** (opt-in): bidirectional sync between skillpm and Claude Code auto memory
  - `bridge/reader.go`: parses Claude Code's `MEMORY.md` for structured signals (package manager, test framework, frameworks, languages, preferences)
  - `bridge/writer.go`: writes `skillpm-rankings.md` topic file to Claude Code's project memory directory
  - `bridge/bridge.go`: `Service` facade with `ReadContext`, `WriteRankings`, `Cleanup`, `Available` methods
  - Claude Code project path resolution (URL-encoded path encoding)
  - rankings include active/inactive skills with score reasons (recently used, strong context match, etc.)
  - never touches MEMORY.md — writes only to dedicated `skillpm-rankings.md`
  - config: `bridge_enabled = false` (opt-in)
- **doctor checks**: 2 new diagnostic checks
  - check 9 (`rules`): verifies rules directory state, counts managed rule files
  - check 10 (`bridge`): verifies Claude Code memory directory, validates rankings file marker

### Changed
- `observation.Observer` constructor now accepts optional `skillRefs` for session transcript parsing
- `memory.New()` accepts optional `skillRefs` variadic parameter (backward compatible)
- `app.Service` passes installed skill refs to memory subsystem for observation v2
- `store.ScanStatePath()` added for new scan state file location
- 3 new config fields: `rules_injection`, `rules_scope`, `bridge_enabled` (all default false)

## [2.0.1] - 2026-02-27

### Fixed
- **scan-path URL resolution**: `skillpm install` with a GitHub URL pointing to a skill directory (e.g. `https://github.com/org/repo/tree/main/skills`) now auto-expands into individual skill installs instead of failing with `SRC_GIT_RESOLVE: skill "skills" not found in scan paths [.]`
- **manifest entries for expanded installs**: project manifest now records each resolved skill individually when installing from a scan-path directory URL

## [2.0.0] - 2026-02-26

### Added
- **procedural memory subsystem**: skills now strengthen with use and decay with disuse — agents self-adapt to your workflow
  - 6-layer cognitive architecture: observation, context, scoring, feedback, consolidation, adaptive injection
  - `skillpm memory enable` / `disable` — toggle the memory subsystem
  - `skillpm memory observe` — scan agent skill directories and record usage events to append-only JSONL event log
  - `skillpm memory events` — query raw usage events with `--since`, `--skill`, `--agent`, `--kind` filters
  - `skillpm memory stats` — per-skill usage statistics (event count, last access, agents)
  - `skillpm memory context` — detect project context (project type, frameworks, task signals)
  - `skillpm memory scores` — show 4-factor activation scores for all installed skills
  - `skillpm memory working-set` — show skills currently in working memory
  - `skillpm memory explain <skill>` — detailed score breakdown for a single skill
  - `skillpm memory rate <skill> [+1|0|-1]` — record explicit feedback on a skill
  - `skillpm memory feedback` — show all feedback signals
  - `skillpm memory consolidate` — run the consolidation pipeline (recompute scores, promote/demote skills)
  - `skillpm memory recommend` — get action recommendations (archive low-activation skills)
  - `skillpm memory set-adaptive [on|off]` — toggle adaptive injection mode
  - `skillpm memory purge` — delete all memory data files
  - `skillpm inject --adaptive` — inject only the working-memory subset of skills
- **activation scoring algorithm**: `Score = 0.35×Recency + 0.25×Frequency + 0.25×ContextMatch + 0.15×Feedback`
  - exponential decay recency with configurable half-life (3d, 7d, 14d)
  - logarithmic frequency scaling
  - project-type, framework, and task-signal context matching
  - explicit feedback boost (+1/0/-1 ratings)
- **context engine**: auto-detects project type (go, node, python, rust, java, ruby), frameworks (react, django, rails, spring, etc.), task signals (feature, bugfix, refactor, test, docs), and build systems
- **consolidation engine**: periodic score recomputation with promote/demote tracking, archival recommendations, and configurable interval
- **memory health check**: `skillpm doctor` now includes a `memory-health` check (8th check) that verifies and auto-creates the memory directory
- **`[memory]` config section**: new configuration block with `enabled`, `working_memory_max`, `threshold`, `recency_half_life`, `observe_on_sync`, `adaptive_inject` settings
- 7 new internal packages: `memory`, `memory/eventlog`, `memory/observation`, `memory/context`, `memory/scoring`, `memory/feedback`, `memory/consolidation`
- ~131 unit tests across all memory packages with >80% coverage
- 4 E2E test functions covering memory lifecycle, adaptive injection, JSON output, and non-interference
- benchmark suite for all memory packages with regression detection via `tools/bench-compare.sh`
- benchmark CI job in GitHub Actions

### Changed
- `memory.New()` returns `(*Service, error)` to propagate initialization failures
- consolidation `saveState`/`saveScores` now return and propagate errors
- eventlog `Truncate` properly handles write/close errors and cleans up temp files on failure
- doctor `checkMemoryHealth` handles stat errors and non-directory edge cases
- CI benchmark job uses correct `bench-compare.sh` invocation (single arg)

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
