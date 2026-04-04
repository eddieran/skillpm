# skillpm

Universal package manager for AI agent skills — install once, inject into Codex, Codex, Gemini, Copilot, OpenCode, Kiro, OpenClaw, and more.

## Repo layout

```text
skillpm/
├── cmd/skillpm/         # CLI entrypoint (cobra commands)
├── internal/            # Private packages (core logic)
├── pkg/                 # Public packages (importable API)
├── skills/              # Bundled skill definitions
├── tools/               # Dev/CI helper scripts
├── test/                # Integration & E2E tests
├── docs/                # Documentation site
├── .github/workflows/   # CI, security, release, leaderboard
├── Makefile             # Build/test/lint targets
├── go.mod / go.sum      # Go module definition
└── WORKFLOW.md          # Symphony orchestration config
```

## Quick reference

```bash
# Build
make build              # → bin/skillpm

# Test
make test               # go test ./... -count=1
go test ./... -v        # verbose

# Lint & format
make lint               # gofmt -l . && go vet ./...
gofmt -w .              # auto-format

# Run
./bin/skillpm --help
```

## Conventions

- Go 1.26+
- Commit format: `type(scope): imperative summary` (feat, fix, refactor, test, docs, chore)
- Branch naming: `<identifier>-short-description` (e.g., `SKP-42-add-search`)
- PR label: `symphony` for agent-created PRs
- No `replace` directives in go.mod for releases
- All exported APIs in `pkg/`, internal logic in `internal/`
- Tests co-located with source (`*_test.go`)

## Read first

- `AGENTS.md` (this file) — repo orientation
- `README.md` — user-facing docs and install instructions
- `docs/architecture.md` — package map and data flow
- `CONTRIBUTING.md` — contribution guidelines
