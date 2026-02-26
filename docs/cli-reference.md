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
| `--adaptive` | `false` | Inject only skills in working memory (requires memory enabled) |

```bash
skillpm inject --agent claude
skillpm inject --agent codex my-repo/code-review
skillpm inject --all
skillpm inject --agent claude --adaptive   # only working-memory skills
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

## `memory` — Procedural memory management

Skills strengthen with use and decay with disuse. See [Procedural Memory](procedural-memory.md) for details.

### `memory enable` / `memory disable`

Toggle the memory subsystem.

```bash
skillpm memory enable
skillpm memory disable
```

### `memory observe`

Scan agent skill directories and record usage events.

```bash
skillpm memory observe
```

### `memory events`

Query raw usage events.

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | `""` | Duration filter (e.g., `7d`, `24h`) |
| `--skill` | `""` | Filter by skill ref |
| `--agent` | `""` | Filter by agent name |
| `--kind` | `""` | Filter by event kind (`access`, `invoke`, `complete`, `error`) |

```bash
skillpm memory events --since 7d
skillpm memory events --skill my-repo/code-review --json
```

### `memory stats`

Per-skill usage statistics.

```bash
skillpm memory stats
skillpm memory stats --json
```

### `memory context`

Detect current project context (type, frameworks, task signals).

```bash
skillpm memory context
skillpm memory context --json
```

### `memory scores`

Show activation scores for all installed skills.

```bash
skillpm memory scores
skillpm memory scores --json
```

### `memory working-set`

Show skills currently in working memory.

```bash
skillpm memory working-set
skillpm memory working-set --json
```

### `memory explain <skill>`

Detailed score breakdown for a single skill.

```bash
skillpm memory explain my-repo/code-review
```

### `memory rate <skill> [+1|0|-1]`

Record explicit feedback on a skill.

```bash
skillpm memory rate my-repo/code-review +1
skillpm memory rate my-repo/linter -1
```

### `memory feedback`

Show all feedback signals.

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | `""` | Duration filter |

```bash
skillpm memory feedback
skillpm memory feedback --since 30d --json
```

### `memory consolidate`

Run the consolidation pipeline (recompute scores, promote/demote).

```bash
skillpm memory consolidate
skillpm memory consolidate --json
```

### `memory recommend`

Get action recommendations based on current scores.

```bash
skillpm memory recommend
skillpm memory recommend --json
```

### `memory set-adaptive [on|off]`

Toggle adaptive injection mode.

```bash
skillpm memory set-adaptive on
skillpm memory set-adaptive off
```

### `memory purge`

Delete all memory data files.

```bash
skillpm memory purge
```

---

## `doctor` — Self-healing diagnostics

Detect and auto-fix environment drift. Runs 8 checks in dependency order. Idempotent — safe to run repeatedly.

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
