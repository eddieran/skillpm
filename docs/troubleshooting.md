# Troubleshooting

Common issues when running `skillpm` locally.

## `sync --strict` exits with code `2`

Meaning: execution completed, but risk posture failed strict policy.

What to do:
1. Re-run with JSON output and inspect risk fields.
2. Review recommended command(s) in JSON (`recommendedCommand` / `recommendedCommands`).
3. Resolve hotspot source, then re-run strict gate.

```bash
./bin/skillpm sync --strict --json > sync.json
```

## Non-zero exit code that is not `2`

Meaning: runtime/validation/execution error.

What to do:
1. Run `doctor` to validate environment.
2. Retry in dry-run mode to isolate planning issues.
3. Check source reachability and local file permissions.

```bash
./bin/skillpm doctor
./bin/skillpm sync --dry-run
```

## Source add/update failures

Likely causes:
- invalid URL
- auth/network failure
- unsupported source type

Checklist:
- verify URL is reachable
- re-run with a known-good public repo
- confirm `--kind` matches source type

## Install or upgrade appears stale

Checklist:
- run `source update` first
- verify lock/state files are writable
- confirm selected ref/version exists upstream

## Known Limitations (Beta)

- No cloud-hosted control plane in v1 (local-first operation only).
- Adapter behavior can differ across runtimes; validate in your target environment.
- Automation consumers must parse JSON output; human-readable text is not a stable API.
- Strict risk policy is intentionally conservative and may require manual follow-up in edge cases.
