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
| `--kind` | `""` | Source type: `git`, `dir`, or `clawhub` |
| `--branch` | `"main"` | Git branch to track |
| `--trust-tier` | `"review"` | Trust tier: `review`, `trusted`, or `untrusted` |

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

Dependencies declared in SKILL.md frontmatter are resolved and installed automatically.

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

## `status` — Show current health and inventory

Display a compact summary of the current scope, health, installed skill count,
configured sources, and enabled adapters.

```bash
skillpm status
skillpm status --json
```

---

## `doctor` — Self-healing diagnostics

Detect and auto-fix environment drift. Runs 7 checks in dependency order.
Idempotent — safe to run repeatedly.

```bash
skillpm doctor
skillpm doctor --json
```

See [Self-Healing Doctor](doctor.md) for check details.

---

## Historical Note

The following command groups were removed in `v4.0.0` and are not available in
current builds:

- `memory`
- `leaderboard`
- `schedule`

Use `skillpm search`, direct Git URL installs, and manual or external
automation around `skillpm sync` instead.

---

## `init` — Initialize a project

Create a `.skillpm/skills.toml` project manifest in the current directory.

```bash
cd ~/myproject
skillpm init
```

See [Project-Scoped Skills](project-scoped-skills.md) for the full workflow.

---

## `create <name>` — Scaffold a new skill

Create a new skill directory with a template SKILL.md.

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Parent directory for the skill |
| `--template` | `default` | Template: `default`, `prompt`, `script` |

```bash
skillpm create my-skill
skillpm create my-prompt --template prompt
skillpm create my-script --template script --dir ~/skills
```

---

## `publish [skill-dir]` — Publish a skill to a registry

Publish a skill from a local directory to a ClawHub registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--source` | `clawhub` | Registry source name |
| `--version` | `""` | Version to publish (default: from SKILL.md or 0.1.0) |

Requires the `CLAWHUB_TOKEN` environment variable.

```bash
skillpm publish ./my-skill --source clawhub --version 1.0.0
skillpm publish .
```

---

## `bundle` — Manage skill bundles

Create, list, and install named groups of skills defined in the project manifest.

### `bundle create <name> <skill-ref>...`

Create a named bundle in the project manifest.

```bash
skillpm bundle create web-dev clawhub/react clawhub/typescript clawhub/eslint
```

### `bundle list`

List all bundles defined in the project manifest.

```bash
skillpm bundle list
skillpm bundle list --json
```

### `bundle install <name>`

Install all skills in a bundle.

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Bypass medium-severity security findings |
| `--lockfile` | `""` | Path to `skills.lock` |

```bash
skillpm bundle install web-dev
skillpm bundle install web-dev --force
```

---

## `list` — List installed skills

Show all installed skills with version and scope information.

```bash
skillpm list
skillpm list --json
skillpm list --scope global
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
