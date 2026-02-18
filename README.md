# skillpm

[![CI](https://github.com/eddieran/skillpm/actions/workflows/ci.yml/badge.svg)](https://github.com/eddieran/skillpm/actions/workflows/ci.yml)
[![Security](https://github.com/eddieran/skillpm/actions/workflows/security.yml/badge.svg)](https://github.com/eddieran/skillpm/actions/workflows/security.yml)
[![Nightly E2E](https://github.com/eddieran/skillpm/actions/workflows/nightly-e2e.yml/badge.svg)](https://github.com/eddieran/skillpm/actions/workflows/nightly-e2e.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-green.svg)](./LICENSE)

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

For a beta onboarding flow, see: `docs/quickstart.md`.

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

### Common Aliases

Top-level aliases:
- `source` → `src`, `sources`
- `search` → `find`, `lookup`
- `install` → `i`, `add`
- `uninstall` → `un`, `del`
- `upgrade` → `up`, `update`
- `inject` → `attach`
- `remove` → `detach`, `eject`
- `sync` → `reconcile`, `recon`
- `harvest` → `collect`, `gather`
- `schedule` → `sched`, `sch`, `scheduler`, `cron`, `auto`, `timer`, `automation`
- `self` → `me`, `myself`

`schedule` subcommand aliases:
- `install` → `add`, `on`, `enable`, `set`, `start`, `update`, `create`, `resume`, `up`, `every`, `apply`, `configure`
- `list` → `ls`, `status`, `st`, `stat`, `show`, `get`, `info`, `query`, `inspect`, `check`, `overview`
- `remove` → `rm`, `off`, `disable`, `stop`, `del`, `delete`, `uninstall`, `clear`, `pause`, `down`, `unset`, `cancel`

`self` subcommand aliases:
- `self update` → `self upgrade`, `self up`, `self fetch`, `self pull`, `self get`, `self latest`

`self-update` shortcut aliases:
- `self-update` → `selfupdate`, `update-self`, `upgrade-self`, `self-refresh`, `refresh-self`, `self-pull`, `latest`

`self-upgrade` shortcut aliases:
- `self-upgrade` → `selfupgrade`, `upgrade-selfpm`, `update-selfpm`

`self-fetch` shortcut aliases:
- `self-fetch` → `selffetch`, `fetch-self`, `pull-self`, `get-self`

`self-get` shortcut aliases:
- `self-get` → `selfget`, `get-selfpm`, `fetch-selfpm`, `pull-selfpm`

`self-latest` shortcut aliases:
- `self-latest` → `selflatest`, `latest-selfpm`, `self-newest`

`self-sync` shortcut aliases:
- `self-sync` → `selfsync`, `sync-self`, `sync-selfpm`

Examples:
```bash
./bin/skillpm cron status
./bin/skillpm schedule
./bin/skillpm schedule status
./bin/skillpm schedule 15m
./bin/skillpm schedule start --interval 15m
./bin/skillpm schedule stop
./bin/skillpm self up
```

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

## Sync Strict Mode & Exit Codes

Use strict mode in automation when risk posture must be enforced:

```bash
./bin/skillpm sync --strict
```

Exit code behavior:
- `0`: sync succeeded and risk posture is acceptable
- `2`: strict policy failure (risk present but execution completed)
- other non-zero: runtime/validation/execution failure

CI usage examples:
```bash
# Pull-request gate: fail fast on planned risk
./bin/skillpm sync --strict --dry-run --json > sync-plan.json

# Deploy gate: enforce clean apply run before release
./bin/skillpm sync --strict --json > sync-apply.json
```

## Machine-readable Contract (Beta)

For external integrations, use JSON output and follow the contract document:
- `docs/sync-contract-v1.md`

Do not parse human-readable console text for automation logic.

## Beta Readiness

Track release readiness here:
- `docs/beta-readiness.md`
- `docs/troubleshooting.md`
- `CHANGELOG.md`

## Project Status

`skillpm` is actively evolving from the v1 foundation and currently focused on:

1. stronger sync UX/observability,
2. adapter reliability hardening,
3. release-readiness and ecosystem adoption.

---

## License

Apache-2.0 © skillpm contributors
