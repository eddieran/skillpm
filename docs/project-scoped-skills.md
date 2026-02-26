# Project-Scoped Skills

> [Docs Index](index.md)

Like `npm install` (local) vs `npm install -g` (global), skillpm supports both project and global scopes. Teams can declare, pin, and share skills per-repository.

## Quick Workflow

```bash
# 1. Initialize the project
cd ~/myproject
skillpm init
# → creates .skillpm/skills.toml

# 2. Install skills (scope auto-detected)
skillpm install my-repo/code-review
# → updates .skillpm/skills.toml + .skillpm/skills.lock

# 3. Team member onboarding
git clone repo && cd repo
skillpm sync
# → installs pinned versions from manifest + lockfile

# 4. Verify
skillpm list
# → my-repo/code-review  (project)
```

## File Layout

```
<project>/
  .skillpm/
    skills.toml    # Project manifest — commit to git
    skills.lock    # Pinned versions — commit to git
    state.toml     # Runtime state — .gitignore
    installed/     # Skill content — .gitignore
    staging/       # Temporary staging — .gitignore
    snapshots/     # Rollback snapshots — .gitignore
```

### What to Commit

Add to version control:
- `.skillpm/skills.toml` — the manifest declaring skill dependencies
- `.skillpm/skills.lock` — pinned versions for reproducible installs

### What to Gitignore

Add to `.gitignore`:
```
.skillpm/installed/
.skillpm/state.toml
.skillpm/staging/
.skillpm/snapshots/
```

`skillpm init` prints these suggestions after creating the manifest.

## Scope Auto-Detection

When you run skillpm inside a directory containing `.skillpm/skills.toml` (or any parent directory), it automatically uses **project scope**. Outside a project, it defaults to **global scope**.

## Explicit `--scope` Flag

Override auto-detection with `--scope`:

```bash
# Force global scope from inside a project
skillpm install my-repo/helper --scope global

# Force project scope explicitly
skillpm install my-repo/helper --scope project

# List global skills from inside a project
skillpm list --scope global
```

## Team Onboarding Flow

1. **Project lead** initializes and adds skills:
   ```bash
   skillpm init
   skillpm install my-repo/code-review
   skillpm install clawhub/steipete/code-review
   git add .skillpm/skills.toml .skillpm/skills.lock
   git commit -m "add skillpm project config"
   ```

2. **Team members** clone and sync:
   ```bash
   git clone <repo> && cd <repo>
   skillpm sync
   ```
   This reads the manifest and lockfile, installs the exact pinned versions, and injects into configured agents.

3. **Updating skills** (by anyone):
   ```bash
   skillpm upgrade
   git add .skillpm/skills.toml .skillpm/skills.lock
   git commit -m "upgrade skills"
   ```

## Scope Isolation

Project and global scopes are fully isolated:
- Separate state directories
- Separate lockfiles
- Separate injection paths (e.g., `<project>/.claude/skills/` vs `~/.claude/skills/`)
- Installing at project scope never affects global state, and vice versa

See [Supported Agents](agents.md) for project-scoped injection paths.
