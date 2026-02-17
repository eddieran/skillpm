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

## 3) Testing Gates

- [ ] mandatory regression set for: `noop`, `changed-with-risk`, `blocked`, `strict-fail`
- [ ] CI status policy defined (e.g. >=95% pass rate over recent runs)
- [ ] nightly e2e trend monitored and linked in release notes

## 4) Documentation for External Users

- [ ] 5-minute quickstart for first successful sync
- [ ] troubleshooting page for common failures
- [ ] known limitations section

## 5) Release Hygiene

- [x] `CHANGELOG.md` present
- [ ] first beta tag (`v0.x`) with release notes
- [ ] rollback guidance documented

## Suggested ship rule

Proceed to external beta when:
1. CI is stable for a rolling window,
2. strict/exit semantics are validated in docs + tests,
3. minimum onboarding docs and troubleshooting are complete.
