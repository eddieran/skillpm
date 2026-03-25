# Contributing to skillpm

Thanks for your interest in improving `skillpm`.

## Development setup

```bash
go test ./...
go vet ./...
```

`go test ./...` is the deterministic default suite and skips live-network E2E unless you opt in.
Run the ClawHub smoke test explicitly when you want external coverage:

```bash
SKILLPM_E2E_REAL_NETWORK=1 go test ./test/e2e -run 'TestRealNetworkInstallAndInject/ClawHub_Standard_Slug' -count=1 -v
```

## Contribution flow

1. Fork and create a feature branch.
2. Keep changes scoped and reviewable.
3. Add/update tests for behavior changes.
4. Run local checks before PR:
   - `go test ./...`
   - `go vet ./...`
   - `./tools/coverage-gate.sh`
   - `SKILLPM_E2E_REAL_NETWORK=1 go test ./test/e2e -run 'TestRealNetworkInstallAndInject/ClawHub_Standard_Slug' -count=1 -v` when you need the live ClawHub smoke lane
5. Open a PR with context, tradeoffs, and test evidence.

## Commit style

Preferred prefixes:
- `feat:` new feature
- `fix:` bug fix
- `test:` tests
- `docs:` docs
- `ci:` CI/CD
- `refactor:` internals without behavior change

## PR expectations

- Keep interface changes explicit.
- Note backward compatibility impact.
- Include command/output snippets for CLI behavior changes.

## Security-sensitive changes

If your change affects trust policy, signatures, or filesystem safety, include dedicated tests and call this out in the PR description.
