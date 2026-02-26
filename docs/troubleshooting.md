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

## Install blocked by security scan (`SEC_SCAN_*`)

Meaning: the skill content triggered one or more security scan rules.

Error codes:
- `SEC_SCAN_CRITICAL`: critical finding (e.g. `curl|bash`, reverse shell) — cannot be bypassed
- `SEC_SCAN_BLOCKED`: high or medium finding — may be bypassed with `--force` (except critical)

What to do:
1. Read the error message to identify which rule and pattern triggered the block.
2. Inspect the skill content manually to verify whether the flagged pattern is legitimate.
3. For medium-severity findings you trust, re-run with `--force`:

```bash
./bin/skillpm install my-repo/admin-tool --force
```

4. To disable a specific rule, add it to config:

```toml
[security.scan]
disabled_rules = ["SCAN_DANGEROUS_PATTERN"]
```

5. To disable scanning entirely (not recommended):

```toml
[security.scan]
enabled = false
```

## Install or upgrade appears stale

Checklist:
- run `source update` first
- verify lock/state files are writable
- confirm selected ref/version exists upstream

## Project scope errors

### `PRJ_NO_MANIFEST: no project manifest found`

You used `--scope project` but no `.skillpm/skills.toml` exists above the current directory.

Fix: run `skillpm init` in the project root first.

### `PRJ_INVALID_SCOPE: invalid scope "..."`

The `--scope` flag only accepts `global` or `project`.

### Project install doesn't affect global

This is by design. Project and global scopes are fully isolated with separate state directories, lockfiles, and injection paths. Use `--scope global` to explicitly target global scope from within a project.

## Known Limitations

- No cloud-hosted control plane (local-first operation only).
- Adapter behavior can differ across runtimes; validate in your target environment.
- Automation consumers must parse JSON output; human-readable text is not a stable API.
- Strict risk policy is intentionally conservative and may require manual follow-up in edge cases.
