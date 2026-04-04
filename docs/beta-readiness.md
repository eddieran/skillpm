# Beta Readiness Checklist

Status target: **Public Beta** (small external audience)

## 1) Contract & Compatibility

- [x] `sync --json` contract documented (`docs/sync-contract-v1.md`)
- [x] compatibility policy documented (stable vs extensible fields)
- [x] schema version included in runtime JSON output and validated in tests

## 2) CLI Behavior

- [x] `--strict` behavior defined
- [x] exit code semantics documented (`0`, `2`, other non-zero)
- [x] strict-mode examples added for CI usage in README

## 2b) Security Scanning

- [x] Pre-install content scanning with 3 built-in rules
- [x] Severity-based enforcement (critical always blocks, high blocks by default, medium requires `--force`)
- [x] Configurable scan policy (`[security.scan]` in config)
- [x] Scanning integrated into install, upgrade, and sync pipelines
- [x] Unit tests for scanner engine and each rule
- [x] Integration tests for scan-blocked installs
- [x] E2E tests for CLI scan behavior

## 3) Testing Gates

- [x] mandatory regression set for: `noop`, `changed-with-risk`, `blocked`, `strict-fail`
  - Run via `make test-sync-regression`
- [x] CI status policy defined (`docs/ci-policy.md`)
- [x] nightly E2E trend monitored with summary artifacts (`docs/ci-policy.md#nightly-trend`)

## 4) Documentation for External Users

- [x] 5-minute quickstart for first successful sync (`docs/quickstart.md`)
- [x] troubleshooting page for common failures (`docs/troubleshooting.md`)
- [x] known limitations section (`docs/troubleshooting.md#known-limitations-beta`)

## 4b) Self-Healing Doctor

- [x] `skillpm doctor` revamped into idempotent self-healing tool
- [x] 7 dependency-ordered checks: config, state, installed-dirs, injections, adapter-state, agent-skills, lockfile
- [x] Auto-creates missing config and enables detected adapters
- [x] Removes orphan dirs, ghost entries, and stale injection refs
- [x] Re-syncs adapter state and restores missing skill files
- [x] Lockfile reconciliation (stale removal + backfill)
- [x] 22 unit tests covering all checks + idempotency verification
- [x] `--enable-detected` flag removed (absorbed into config check)

## 5) Release Hygiene

- [x] `CHANGELOG.md` present
- [x] release tags with release notes through `v4.0.0`
- [x] rollback guidance documented (`docs/rollback.md`)

## Suggested ship rule

Proceed to external beta when:
1. CI is stable for a rolling window,
2. strict/exit semantics are validated in docs + tests,
3. minimum onboarding docs and troubleshooting are complete.
