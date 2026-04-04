---
name: debug
description:
  Investigate test failures, build errors, and runtime bugs in the skillpm Go
  codebase; use when tests fail, builds break, or unexpected behavior occurs.
---

# Debug

## Goals

- Find the root cause of test failures, build errors, or runtime bugs.
- Use Go-native tooling for investigation.
- Produce a minimal reproduction and targeted fix.

## Toolchain

| Tool | Purpose |
|------|---------|
| `go test ./... -v` | Run all tests with verbose output |
| `go test ./... -run TestName` | Run a specific test by name |
| `go test ./... -count=1` | Bypass test cache |
| `go build ./...` | Check compilation across all packages |
| `go vet ./...` | Static analysis for common bugs |
| `gofmt -l .` | Find files with formatting issues |
| `gofmt -d .` | Show formatting diffs |
| `dlv test ./pkg/... -- -test.run TestName` | Debug a specific test interactively |
| `go test -race ./...` | Detect data races |
| `go test -coverprofile=coverage.out ./...` | Generate coverage profile |

## Quick Triage

1. **Build check**: `go build ./...` — does it compile?
2. **Vet check**: `go vet ./...` — any static analysis issues?
3. **Format check**: `gofmt -l .` — any formatting violations?
4. **Test run**: `go test ./... -v -count=1` — which tests fail?
5. **Targeted test**: `go test -v -run TestFailingName ./path/to/pkg/`

## Investigation Flow

1. **Reproduce the failure**:
   - Run the exact failing command and capture full output.
   - Note the package, test name, and error message.
2. **Isolate the scope**:
   - Run only the failing package: `go test -v ./internal/failing/...`
   - Run only the failing test: `go test -v -run TestName ./internal/failing/`
3. **Read the code**:
   - Open the test file and the code under test.
   - Trace the execution path from the test assertion backward.
4. **Check dependencies**:
   - `go mod verify` — are module checksums intact?
   - `go mod tidy` — are imports consistent with go.mod?
5. **Add diagnostic output** (temporary):
   - Use `t.Logf(...)` in tests for debug output (visible with `-v`).
   - Use `fmt.Fprintf(os.Stderr, ...)` for non-test code.
   - Remove all diagnostic code before committing.
6. **Fix and validate**:
   - Apply the minimal fix.
   - Run `go build ./... && go test ./... -count=1 && go vet ./...`
   - Run `gofmt -w .` if formatting changed.

## Common Failure Patterns

| Symptom | Likely cause | Action |
|---------|-------------|--------|
| `undefined:` | Missing import or unexported name | Check imports and capitalization |
| `cannot use X as type Y` | Interface mismatch or wrong type | Check method signatures |
| `--- FAIL: TestX` | Test assertion failure | Read expected vs actual values |
| `panic: runtime error` | Nil pointer, index out of range | Add nil checks, bounds checks |
| `go.sum mismatch` | Tampered or stale module cache | `go mod tidy && go mod verify` |
| Data race detected | Concurrent access without sync | Add mutex or use channels |

## Notes

- Always use `-count=1` to bypass test cache when debugging.
- Prefer `go test -v` for full output including `t.Log` statements.
- Remove all temporary debug code before the final commit.
- If a test is flaky, run it multiple times: `go test -count=5 -run TestFlaky ./...`
