# skillpm

`skillpm` is a local-first skill package manager for AI agents.

## Status

This repository implements the v1 foundational architecture from `internal_docs`:

- Frozen CLI surface (`source`, `search`, `install`, `uninstall`, `upgrade`, `inject`, `remove`, `sync`, `schedule`, `harvest`, `validate`, `doctor`, `self update`)
- TOML config and lockfile schema support
- ClawHub discovery and API fallback logic
- Staging + atomic install transaction model with rollback behavior
- Adapter contract + file-backed adapter runtime (Codex/OpenClaw)
- Unit/integration/e2e/security/conformance tests

## Quick Start

```bash
# Build
make build

# Show help
./bin/skillpm --help

# Add source
./bin/skillpm source add local https://example.com/skills.git --kind git

# Install skill
./bin/skillpm install local/demo
```

## Project Layout

```text
cmd/skillpm/         # CLI wiring
internal/app/        # use-case orchestration
internal/config/     # config schema and persistence
internal/source/     # source providers (git/clawhub)
internal/importer/   # skill shape normalization
internal/store/      # state and lockfile I/O
internal/resolver/   # ref parsing and version resolution
internal/installer/  # staging, commit, rollback
internal/adapter/    # adapter runtime implementations
internal/sync/       # sync orchestration
internal/harvest/    # candidate capture
internal/security/   # policy and path safety
internal/doctor/     # diagnostics
internal/audit/      # structured audit logging
pkg/adapterapi/      # stable adapter contract
test/e2e/            # command-level scenarios
```

## Development

```bash
go test ./...
go vet ./...
```
