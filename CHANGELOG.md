# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added
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
