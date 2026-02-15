# skillpm

[![CI](https://github.com/eddieran/skillpm/actions/workflows/ci.yml/badge.svg)](https://github.com/eddieran/skillpm/actions/workflows/ci.yml)
[![Security](https://github.com/eddieran/skillpm/actions/workflows/security.yml/badge.svg)](https://github.com/eddieran/skillpm/actions/workflows/security.yml)
[![Nightly E2E](https://github.com/eddieran/skillpm/actions/workflows/nightly-e2e.yml/badge.svg)](https://github.com/eddieran/skillpm/actions/workflows/nightly-e2e.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENSE)

`skillpm` is a **local-first skill package manager for AI agents**.
It helps teams reliably install, upgrade, inject, sync, and audit skills across agent runtimes (e.g. Codex/OpenClaw) with rollback-safe operations.

---

## Why skillpm

- **Local-first by default**: no cloud control plane required for v1.
- **Operational safety**: staging + atomic commit + rollback model.
- **Agent interoperability**: stable adapter contract and runtime abstraction.
- **ClawHub-ready**: discovery + API fallback support for `clawhub.ai` compatible sources.
- **Production-minded quality gates**: unit/integration/e2e/security + CI coverage gates for critical packages.

---

## Key Features

- Source lifecycle: `source add/remove/list/update`
- Search/install/uninstall/upgrade with lockfile-backed provenance
- Inject/remove against supported agent adapters
- Sync engine with:
  - plan/apply behavior
  - `--dry-run` planning mode
  - action/risk summary details
- Harvest engine for agent-side skill discovery
- Diagnostics (`doctor`) and self-update flow

---

## Quick Start

### Build

```bash
make build
./bin/skillpm --help
```

### Add source

```bash
./bin/skillpm source add local https://example.com/skills.git --kind git
```

### Install skill

```bash
./bin/skillpm install local/demo
```

### Plan sync without mutating state

```bash
./bin/skillpm sync --dry-run
```

---

## CLI Surface (v1)

- `source`
- `search`
- `install`
- `uninstall`
- `upgrade`
- `inject`
- `remove`
- `sync`
- `schedule`
- `harvest`
- `validate`
- `doctor`
- `self update`

---

## Architecture at a glance

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

---

## Quality & Security

- CI: format / vet / test / build / race
- Security workflow + SBOM/release pipeline
- Critical package coverage gates enforced in CI (`tools/coverage-gate.sh`)

For vulnerability reporting, see [SECURITY.md](./SECURITY.md).

---

## Contributing

We welcome issues and PRs.

- Read [CONTRIBUTING.md](./CONTRIBUTING.md)
- Use issue templates for bug reports / feature requests
- Follow the [Code of Conduct](./CODE_OF_CONDUCT.md)

---

## Project Status

`skillpm` is actively evolving from the v1 foundation and currently focused on:

1. stronger sync UX/observability,
2. adapter reliability hardening,
3. release-readiness and ecosystem adoption.

---

## License

MIT © skillpm contributors
