# CI Status Policy

> [Docs Index](index.md)

This document defines the continuous integration quality gates that must hold before any release.

## Required CI Checks

Every pull request and push to `main` must pass **all** of the following checks:

| Check | Job | What It Validates |
|-------|-----|-------------------|
| **lint** | `lint-test-build` | `gofmt -l .` returns no output |
| **vet** | `lint-test-build` | `go vet ./...` reports no issues |
| **test** | `lint-test-build` | `go test ./... -count=1` passes all unit + integration tests |
| **coverage gates** | `lint-test-build` | `tools/coverage-gate.sh` passes for critical packages |
| **build** | `lint-test-build` | `go build ./cmd/skillpm` compiles cleanly |
| **sync regression** | `sync-regression` | `make test-sync-regression` preserves the documented `sync` output contract |
| **race** | `race` | `go test -race ./... -count=1` detects no data races |

A PR **must not** be merged unless every required check is green.

Live-network E2E is intentionally excluded from the required `go test ./...` contract unless `SKILLPM_E2E_REAL_NETWORK=1` is set. This keeps unattended validation deterministic when external registries rate limit or degrade.

## Pass Rate Requirement

Maintain a **>= 95% pass rate** over a **rolling 20-run window** on the `main` branch.

How to measure:

```
Pass rate = (green runs in last 20) / 20 * 100
```

If the pass rate drops below 95% (i.e., more than 1 failure in the last 20 runs):

1. The release pipeline is **blocked** until the rate recovers.
2. The on-call engineer must triage all failures within 24 hours.
3. Flaky tests must be quarantined or fixed within 48 hours.

## Nightly E2E Trend {#nightly-trend}

The nightly E2E workflow (`nightly-e2e.yml`) runs 6 test lanes every night at 04:00 UTC:

| Lane | What It Covers |
|------|---------------|
| `integration` | Full integration test suite |
| `openclaw-clawhub` | Explicit `SKILLPM_E2E_REAL_NETWORK=1` smoke against `TestRealNetworkInstallAndInject/ClawHub_Standard_Slug` |
| `contract-probe` | API endpoint contract probing |
| `openapi-drift` | OpenAPI spec drift detection |
| `resilience` | Rate limiting and retry behavior |
| `rollback` | Rollback and recovery paths |

The `openclaw-clawhub` lane is the only default automation that opts into live ClawHub traffic. If ClawHub responds with HTTP 429 during that smoke path, the test is skipped as external rate-limit noise rather than treated as a product regression.

### Release Gate

**Nightly E2E must be green for 3 consecutive nights before any release.**

The nightly workflow produces:

- Per-lane JSON result artifacts (`nightly-results-<lane>.json`)
- A consolidated markdown summary in the GitHub Actions run summary

If any lane fails:

1. The 3-night counter **resets to zero**.
2. The failure must be triaged by end of next business day.
3. No release may proceed until 3 consecutive green nights are achieved.

### Trend Monitoring

Each nightly run uploads a JSON artifact with pass/fail status per lane. Over time these artifacts form a trend record. To review the trend:

1. Navigate to **Actions > nightly-e2e** in the GitHub repository.
2. Open the run summary for a consolidated view of all lanes.
3. Download individual `nightly-results-*` artifacts for programmatic analysis.

## Escalation Process

### Severity Levels

| Severity | Condition | Response Time |
|----------|-----------|---------------|
| **P0 — Release Blocker** | Nightly E2E fails for 3+ consecutive nights, or pass rate drops below 80% | Immediate (same day) |
| **P1 — Urgent** | Pass rate between 80-95%, or single nightly lane repeatedly failing | Within 24 hours |
| **P2 — Normal** | Isolated flaky test, benchmark near threshold | Within 48 hours |

### Escalation Steps

1. **Detect**: CI failure is noticed via GitHub Actions notification or daily check.
2. **Triage**: On-call engineer reviews the failure within the response time for its severity.
3. **Classify**: Determine if the failure is:
   - A real regression (fix the code)
   - A flaky test (quarantine and file a tracking issue)
   - An infrastructure issue (retry and report to platform team)
4. **Fix or Quarantine**:
   - Real regressions: open a PR with the fix, get it merged promptly.
   - Flaky tests: add `t.Skip("flaky: #issue")` with a linked tracking issue, then fix within one sprint.
5. **Verify**: Confirm the pass rate recovers above 95% and nightly E2E is green.
6. **Communicate**: Post a summary in the team channel if the issue affected release timelines.

### Policy Violations

If a release ships despite a policy violation (e.g., nightly E2E not green for 3 nights):

1. Conduct a post-incident review within 5 business days.
2. Document root cause and corrective actions.
3. Update this policy if the threshold needs adjustment.
