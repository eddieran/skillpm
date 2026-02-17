# skillpm 5-minute Quickstart

Goal: complete one successful local install + sync flow with verifiable output.

## 0) Prerequisites

- Go toolchain installed
- A writable local workspace
- `git` available in `PATH`

## 1) Build CLI

```bash
make build
./bin/skillpm --help
```

Expected: help output shows core commands (`source`, `install`, `sync`, ...).

## 2) Register a source

```bash
./bin/skillpm source add local https://example.com/skills.git --kind git
./bin/skillpm source list
```

Expected: `local` source appears in list.

## 3) Install one skill

```bash
./bin/skillpm install local/demo
```

Expected: install succeeds and lock/state metadata is written.

## 4) Run dry-run sync plan

```bash
./bin/skillpm sync --dry-run --json > sync-plan.json
```

Expected:
- command exits `0`
- `sync-plan.json` is valid JSON
- includes summary fields documented in `docs/sync-contract-v1.md`

## 5) Enforce strict policy (optional gate)

```bash
./bin/skillpm sync --strict --dry-run --json > sync-plan-strict.json
```

Exit codes:
- `0`: acceptable risk posture
- `2`: strict policy failure
- other non-zero: runtime/validation failure

## Next

- Troubleshoot common failures: `docs/troubleshooting.md`
- Contract details for automation: `docs/sync-contract-v1.md`
- Release readiness status: `docs/beta-readiness.md`
