# Troubleshooting

> [Docs Index](index.md) · [CLI Reference](cli-reference.md) · [Config Reference](config-reference.md)

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
1. Run `doctor` to auto-detect and fix environment drift.
2. Retry in dry-run mode to isolate planning issues.
3. Check source reachability and local file permissions.

```bash
./bin/skillpm doctor
./bin/skillpm sync --dry-run
```

## Environment drift / stale state

Symptoms: injections reference uninstalled skills, orphan directories in `installed/`, adapter's `injected.toml` out of sync with state.

Fix: run `doctor` — it detects and auto-repairs all of these in one pass:

```bash
skillpm doctor
```

The doctor runs 7 checks in dependency order:
1. **config** — creates missing config, enables detected adapters
2. **state** — resets corrupt state
3. **installed-dirs** — removes orphan dirs and ghost state entries
4. **injections** — removes stale refs
5. **adapter-state** — re-syncs injected.toml
6. **agent-skills** — restores missing skill files
7. **lockfile** — reconciles lock with state

Doctor is idempotent — run it again and the second pass will show all `[ok]`.

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

Meaning: the skill content triggered one or more security scan rules. See [Security Scanning](security-scanning.md) for the full rule reference.

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

See [Project-Scoped Skills](project-scoped-skills.md) for the full project workflow.

## Project scope errors

### `PRJ_NO_MANIFEST: no project manifest found`

You used `--scope project` but no `.skillpm/skills.toml` exists above the current directory.

Fix: run `skillpm init` in the project root first.

### `PRJ_INVALID_SCOPE: invalid scope "..."`

The `--scope` flag only accepts `global` or `project`.

### Project install doesn't affect global

This is by design. Project and global scopes are fully isolated with separate state directories, lockfiles, and injection paths. Use `--scope global` to explicitly target global scope from within a project.

## Memory errors (`MEM_*`)

### `MEM_DISABLED: memory not enabled`

You ran a memory command but the subsystem is off.

Fix: `skillpm memory enable`

### `MEM_INIT: create memory dir`

The memory directory could not be created (permissions issue).

Fix: check that `~/.skillpm/` is writable, then retry `skillpm memory enable`.

### Scores show `activation_level: 0` for all skills

Likely cause: the observer hasn't been run, or the skill ref in the event log doesn't match the installed skill ref (e.g., `my-skill` vs `local/my-skill`).

Fix:
1. Run `skillpm memory observe` to record events.
2. Run `skillpm memory scores` to recompute.

The scoring engine uses a basename fallback for SkillRef matching, so `local/my-skill` will match events recorded under `my-skill`.

### `MEM_CONSOLIDATE_*` errors

The consolidation pipeline failed to persist scores or state.

Fix: check disk space and file permissions under `~/.skillpm/memory/`. Run `skillpm doctor` to verify the memory directory.

## Known Limitations

- No cloud-hosted control plane (local-first operation only).
- Adapter behavior can differ across runtimes; validate in your target environment.
- Automation consumers must parse JSON output; human-readable text is not a stable API.
- Strict risk policy is intentionally conservative and may require manual follow-up in edge cases.
- Memory observation is filesystem-based (mtime scanning) — it does not track real-time agent execution.
