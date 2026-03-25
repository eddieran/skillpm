# Rollback Guidance

> [Docs Index](index.md) · [CLI Reference](cli-reference.md) · [Troubleshooting](troubleshooting.md)

This document covers how to recover from failed installs, failed syncs, corrupted state, and other emergency situations.

## Rollback a Failed Install

If `skillpm install` fails partway through (e.g., network error, security scan block, disk full), the skill may be partially written to disk but not fully registered in state.

### Steps

1. **Uninstall the failed skill** to clean up any partial state:

```bash
skillpm uninstall <source/skill>
```

2. **Re-install the previous version** (if upgrading):

```bash
skillpm install <source/skill@previous-version>
```

3. **Run doctor** to verify the environment is clean:

```bash
skillpm doctor
```

### Example: Rolling Back a Failed Upgrade

```bash
# Upgrade fails midway
skillpm upgrade my-repo/code-review
# => error: network timeout during download

# Uninstall the broken state
skillpm uninstall my-repo/code-review

# Re-install the known-good version
skillpm install my-repo/code-review@1.2.0

# Verify everything is consistent
skillpm doctor
```

## Rollback a Failed Sync

`skillpm sync` runs a multi-step pipeline: update sources, upgrade skills, re-inject agents. A failure at any stage can leave state partially updated.

### Steps

1. **Restore `state.toml` from backup.**

   Before each sync, back up the state file:

```bash
# Pre-sync backup (do this before running sync)
cp ~/.skillpm/state.toml ~/.skillpm/state.toml.bak
```

   If sync fails, restore the backup:

```bash
cp ~/.skillpm/state.toml.bak ~/.skillpm/state.toml
```

   For project-scoped state:

```bash
cp .skillpm/state.toml .skillpm/state.toml.bak   # before sync
cp .skillpm/state.toml.bak .skillpm/state.toml    # after failure
```

2. **Run doctor** to reconcile state with disk:

```bash
skillpm doctor
```

   Doctor will detect any mismatches between `state.toml`, the `installed/` directory, and agent injection directories, then auto-fix them.

3. **Re-run sync in dry-run mode** to verify the plan looks correct:

```bash
skillpm sync --dry-run
```

4. **Apply sync** once dry-run output is satisfactory:

```bash
skillpm sync
```

### Lockfile Recovery

If the lockfile (`skills.lock` or `.skillpm/skills.lock`) is corrupted or out of sync:

1. Delete the lockfile:

```bash
rm ~/.skillpm/skills.lock          # global
rm .skillpm/skills.lock            # project
```

2. Run doctor to regenerate it from state:

```bash
skillpm doctor
```

Doctor's lockfile check will backfill missing lock entries from the current state.

## Using `skillpm doctor` to Recover from Corruption

`skillpm doctor` is the primary recovery tool. It runs 8 checks in dependency order and auto-fixes most issues in a single pass.

### Common Corruption Scenarios

| Symptom | Doctor Check That Fixes It |
|---------|---------------------------|
| Missing `config.toml` | **config** -- creates config with defaults, enables detected adapters |
| Corrupt or unparseable `state.toml` | **state** -- resets to empty valid state |
| Orphan directories in `installed/` | **installed-dirs** -- removes dirs not tracked in state |
| Ghost entries in state (no dir on disk) | **installed-dirs** -- removes state entries with no backing dir |
| Stale injection refs to uninstalled skills | **injections** -- removes refs, cleans empty agent entries |
| Adapter `injected.toml` out of sync | **adapter-state** -- re-syncs with canonical state |
| Missing skill files in agent dirs | **agent-skills** -- restores from installed cache |
| Stale or missing lock entries | **lockfile** -- removes stale, backfills missing |
| Missing memory directory | **memory-health** -- auto-creates when memory is enabled |

### Recovery Procedure

```bash
# Run doctor (safe and idempotent)
skillpm doctor

# Verify with a second pass (should show all [ok])
skillpm doctor

# For machine-readable output
skillpm doctor --json
```

If doctor reports `error` status on any check, the issue requires manual intervention. See the Emergency Procedures section below.

## Emergency Procedures

When `skillpm doctor` cannot auto-repair the issue, use these manual cleanup steps.

### Manual File Cleanup Paths

All skillpm data lives under two roots:

| Scope | Root | Contents |
|-------|------|----------|
| Global | `~/.skillpm/` | `config.toml`, `state.toml`, `skills.lock`, `installed/`, `memory/` |
| Project | `.skillpm/` (in project root) | `skills.toml`, `state.toml`, `skills.lock`, `installed/` |

Agent injection directories (where skills are copied for agent consumption):

| Agent | Global Path | Project Path |
|-------|-------------|--------------|
| claude | `~/.claude/skills/` | `.claude/skills/` |
| codex | `~/.agents/skills/` | `.agents/skills/` |
| copilot | `~/.copilot/skills/` | `.github/skills/` |
| cursor | `~/.cursor/skills/` | `.cursor/skills/` |
| gemini | `~/.gemini/skills/` | `.gemini/skills/` |
| trae | `~/.trae/skills/` | `.trae/skills/` |
| opencode | `~/.config/opencode/skills/` | `.opencode/skills/` |
| kiro | `~/.kiro/skills/` | `.kiro/skills/` |
| openclaw | `~/.openclaw/workspace/skills/` | `skills/` |

### Full Global Reset

If global state is unrecoverably corrupt:

```bash
# 1. Remove all skillpm state (this will uninstall all skills)
rm -rf ~/.skillpm/

# 2. Remove injected skills from all agent directories
rm -rf ~/.claude/skills/*
rm -rf ~/.agents/skills/*
rm -rf ~/.copilot/skills/*
rm -rf ~/.cursor/skills/*
rm -rf ~/.gemini/skills/*
rm -rf ~/.trae/skills/*
rm -rf ~/.config/opencode/skills/*
rm -rf ~/.kiro/skills/*
rm -rf ~/.openclaw/workspace/skills/*

# 3. Re-initialize
skillpm doctor

# 4. Re-install skills from scratch
skillpm install <source/skill> ...
```

### Full Project Reset

If project-scoped state is unrecoverably corrupt:

```bash
# 1. Remove project skillpm state (preserves the manifest)
rm -f .skillpm/state.toml .skillpm/skills.lock
rm -rf .skillpm/installed/

# 2. Remove project-local injections
rm -rf .claude/skills/*
rm -rf .agents/skills/*
rm -rf .github/skills/*
rm -rf skills/*
# ... (repeat for other agents as needed)

# 3. Re-sync from the manifest
skillpm sync
```

The manifest (`.skillpm/skills.toml`) is the source of truth for project-scoped skills. As long as it is intact, `skillpm sync` will re-install everything declared in it.

### Recovering a Specific Skill

If a single skill is broken but the rest of the environment is fine:

```bash
# Uninstall the broken skill
skillpm uninstall <source/skill>

# Run doctor to clean up any residual state
skillpm doctor

# Re-install
skillpm install <source/skill>
```

### When All Else Fails

1. Back up your `config.toml` and project `.skillpm/skills.toml` manifests.
2. Perform a full reset (see above).
3. Restore your config and re-sync.
4. If the issue persists, file a bug report with the output of `skillpm doctor --json`.
