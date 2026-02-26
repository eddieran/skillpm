# CLI Reference

> [Docs Index](index.md)

All commands support `--json` for machine-readable output and `--scope <global|project>` for explicit scope selection (auto-detected when omitted). Use `--config <path>` to override the config file location.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `2` | Strict policy failure (`sync --strict`) |
| non-zero | Runtime or validation error |

---

## `source` — Manage skill sources

### `source add <name> <url-or-site>`

Register a new skill source.

| Flag | Default | Description |
|------|---------|-------------|
| `--kind` | `""` | Source type: `git` or `clawhub` |
| `--branch` | `""` | Git branch to track |
| `--trust-tier` | `""` | Trust tier: `review`, `trusted` |

```bash
skillpm source add my-repo https://github.com/org/skills.git --kind git
skillpm source add hub https://clawhub.ai/ --kind clawhub
```

### `source list`

List all configured sources.

```bash
skillpm source list
```

### `source update [name]`

Fetch latest metadata from one or all sources.

```bash
skillpm source update          # update all
skillpm source update my-repo  # update one
```

### `source remove <name>`

Remove a source from the config.

```bash
skillpm source remove my-repo
```

---

## `search <query>` — Search available skills

| Flag | Default | Description |
|------|---------|-------------|
| `--source` | `""` | Restrict search to a specific source |

```bash
skillpm search "code-review"
skillpm search "test" --source clawhub
```

---

## `install <source/skill[@constraint]>...` — Install skills

Install one or more skills. Supports source-qualified refs and direct Git URLs.

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Bypass medium-severity security findings |
| `--lockfile` | `""` | Path to `skills.lock` |

```bash
skillpm install my-repo/code-review
skillpm install clawhub/steipete/code-review@^1.0
skillpm install https://github.com/anthropics/skills/tree/main/skills/skill-creator --force
```

---

## `uninstall <source/skill>...` — Uninstall skills

Remove installed skills from state and disk.

| Flag | Default | Description |
|------|---------|-------------|
| `--lockfile` | `""` | Path to `skills.lock` |

```bash
skillpm uninstall my-repo/code-review
```

---

## `upgrade [source/skill ...]` — Upgrade installed skills

Upgrade specific skills or all installed skills to latest versions.

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Bypass medium-severity security findings |
| `--lockfile` | `""` | Path to `skills.lock` |

```bash
skillpm upgrade                        # upgrade all
skillpm upgrade my-repo/code-review    # upgrade one
```

---

## `inject [source/skill ...]` — Inject skills into agents

Push installed skills into an agent's native `skills/` directory.

| Flag | Default | Description |
|------|---------|-------------|
| `--agent` | `""` | Target agent name (required unless `--all`) |
| `--all` | `false` | Inject into all enabled agents |

```bash
skillpm inject --agent claude
skillpm inject --agent codex my-repo/code-review
skillpm inject --all
```

---

## `sync` — Reconcile state

Run the full sync pipeline: update sources → upgrade skills → re-inject agents.

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Show planned actions without mutating state |
| `--strict` | `false` | Exit `2` if any risk items are present |
| `--force` | `false` | Bypass medium-severity security findings |
| `--lockfile` | `""` | Path to `skills.lock` |

```bash
skillpm sync --dry-run              # preview changes
skillpm sync                        # apply changes
skillpm sync --strict --json        # CI gate
```

---

## `doctor` — Self-healing diagnostics

Detect and auto-fix environment drift. Runs 7 checks in dependency order. Idempotent — safe to run repeatedly.

```bash
skillpm doctor
skillpm doctor --json
```

See [Self-Healing Doctor](doctor.md) for check details.

---

## `leaderboard` — Browse trending skills

| Flag | Default | Description |
|------|---------|-------------|
| `--category` | `""` | Filter: `agent`, `tool`, `workflow`, `data`, `security` |
| `--limit` | `15` | Maximum entries to display |

```bash
skillpm leaderboard
skillpm leaderboard --category security --limit 5
skillpm leaderboard --json
```

---

## `init` — Initialize a project

Create a `.skillpm/skills.toml` project manifest in the current directory.

```bash
cd ~/myproject
skillpm init
```

See [Project-Scoped Skills](project-scoped-skills.md) for the full workflow.

---

## `list` — List installed skills

Show all installed skills with version and scope information.

```bash
skillpm list
skillpm list --json
skillpm list --scope global
```

---

## `schedule` — Manage scheduler settings

### `schedule`

Show or set the sync schedule.

```bash
skillpm schedule              # show current
skillpm schedule 15m          # set interval
```

### `schedule install [interval]`

Enable the scheduler with an interval.

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | `""` | Scheduler interval (e.g., `15m`, `1h`) |

```bash
skillpm schedule install 30m
```

### `schedule list`

Show current scheduler settings.

```bash
skillpm schedule list
```

### `schedule remove`

Disable the scheduler.

```bash
skillpm schedule remove
```

---

## `self update` — Update skillpm

Update the skillpm binary.

| Flag | Default | Description |
|------|---------|-------------|
| `--channel` | `stable` | Release channel |

```bash
skillpm self update
skillpm self update --channel beta
```

---

## `version` — Show version

```bash
skillpm version
skillpm version --json
```
