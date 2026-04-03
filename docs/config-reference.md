# Config Reference

> [Docs Index](index.md)

## File Locations

| File | Purpose |
|------|---------|
| `~/.skillpm/config.toml` | Global configuration |
| `.skillpm/skills.toml` | Project manifest (per-repository) |
| `.skillpm/skills.lock` | Pinned versions (per-repository) |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `CLAWHUB_TOKEN` | Authentication token for publishing skills to ClawHub registries |
| `OPENCLAW_STATE_DIR` | Override OpenClaw's state directory when resolving global paths |
| `OPENCLAW_CONFIG_PATH` | Override OpenClaw's config path when resolving global paths |
| `OPENCLAW_WORKSPACE_DIR` | Override OpenClaw's workspace directory when resolving global paths |
| `SKILLPM_SELF_UPDATE_TARGET` | Advanced override for the executable replaced by `skillpm self update` |
| `SKILLPM_UPDATE_MANIFEST_URL` | Advanced override for the self-update manifest URL |
| `SKILLPM_UPDATE_MANIFEST_BASE` | Advanced override for the self-update manifest base URL |

Prefer CLI flags such as `--config` and TOML settings for normal operation.
The `SKILLPM_*` variables above are primarily for self-update overrides and
test harnesses.

---

## Schema (v1)

### Top-Level Fields

```toml
version = 1
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `version` | int | `1` | Schema version (always `1`) |

### `[sync]`

```toml
[sync]
mode = "system"
interval = "6h"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `"system"` | Compatibility field retained in the v1 schema |
| `interval` | string | `"6h"` | Compatibility field retained in the v1 schema |

`skillpm` no longer ships built-in scheduler commands in `v4.x`, but the
`[sync]` block remains in the config schema for backward compatibility with
existing config files.

### `[security]`

```toml
[security]
profile = "strict"
require_signatures = true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `profile` | string | `"strict"` | Trust policy profile; `strict` denies installs from `untrusted` sources |
| `require_signatures` | bool | `true` | Require signatures when running `skillpm self update` |

### `[security.scan]`

```toml
[security.scan]
enabled = true
block_severity = "high"
disabled_rules = []
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable security scanning on install/upgrade |
| `block_severity` | string | `"high"` | Minimum severity that blocks: `critical`, `high`, `medium`, `low`, `info` |
| `disabled_rules` | string[] | `[]` | Rule IDs to skip (e.g., `["SCAN_DANGEROUS_PATTERN"]`) |

See [Security Scanning](security-scanning.md) for rule details.

### Removed in `v4.0.0`

The following config sections were removed in `v4.0.0` and are no longer part
of the supported schema:

- `[memory]`
- `[hooks]`

If you still have these sections in an older config, remove them before
expecting the current CLI and docs to line up with your local file.

### `[storage]`

```toml
[storage]
root = "~/.skillpm"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `root` | string | `"~/.skillpm"` | Root directory for all skillpm data |

### `[logging]`

```toml
[logging]
level = "info"
format = "text"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `"info"` | Log level: `debug`, `info`, `warn`, `error` |
| `format` | string | `"text"` | Output format: `text` or `json` |

### `[[sources]]`

Each source is declared as a TOML array entry.

```toml
[[sources]]
name = "anthropic"
kind = "git"
url = "https://github.com/anthropics/skills.git"
branch = "main"
scan_paths = ["skills"]
trust_tier = "review"

[[sources]]
name = "local-skills"
kind = "dir"
url = "/Users/alice/skills"
trust_tier = "trusted"

[[sources]]
name = "clawhub"
kind = "clawhub"
site = "https://clawhub.ai/"
registry = "https://clawhub.ai/"
auth_base = "https://clawhub.ai/"
well_known = ["/.well-known/clawhub.json", "/.well-known/clawdhub.json"]
api_version = "v1"
trust_tier = "review"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Unique source name |
| `kind` | string | yes | `git`, `dir`, or `clawhub` |
| `url` | string | git/dir only | Git repository URL or local directory path |
| `branch` | string | no | Optional Git branch override. If omitted in raw config, clone the repository default branch. `skillpm source add` defaults this to `main` unless you override it. |
| `scan_paths` | string[] | no | Subdirectories containing skills |
| `trust_tier` | string | yes | `review`, `trusted`, or `untrusted` |
| `site` | string | clawhub | Registry site URL |
| `registry` | string | clawhub | API registry URL |
| `auth_base` | string | clawhub | Authentication base URL |
| `well_known` | string[] | clawhub | Well-known discovery paths |
| `api_version` | string | clawhub | API version string |
| `cached_registry` | string | no | Cached registry URL learned from ClawHub metadata discovery |
| `min_cli_version` | string | no | Minimum `skillpm` version requested by ClawHub metadata |

### `[[adapters]]`

Each agent adapter is declared as a TOML array entry.

```toml
[[adapters]]
name = "claude"
enabled = true
scope = "global"

[[adapters]]
name = "codex"
enabled = true
scope = "global"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | — | Agent name (see [Supported Agents](agents.md)) |
| `enabled` | bool | `false` | Whether this adapter is active |
| `scope` | string | `"global"` | Default scope: `global` or `project` |

Supported adapter names: `claude`, `codex`, `copilot`, `cursor`, `gemini`, `antigravity`, `kiro`, `opencode`, `trae`, `vscode`, `openclaw`.

---

## Default Config

When `~/.skillpm/config.toml` does not exist, `skillpm` creates it with these
defaults:

```toml
version = 1

[sync]
mode = "system"
interval = "6h"

[security]
profile = "strict"
require_signatures = true

[security.scan]
enabled = true
block_severity = "high"

[storage]
root = "~/.skillpm"

[logging]
level = "info"
format = "text"

[[sources]]
name = "anthropic"
kind = "git"
url = "https://github.com/anthropics/skills.git"
branch = "main"
scan_paths = ["skills"]
trust_tier = "review"

[[sources]]
name = "clawhub"
kind = "clawhub"
site = "https://clawhub.ai/"
registry = "https://clawhub.ai/"
auth_base = "https://clawhub.ai/"
well_known = ["/.well-known/clawhub.json", "/.well-known/clawdhub.json"]
api_version = "v1"
trust_tier = "review"

[[sources]]
name = "community"
kind = "clawhub"
site = "https://clawhub.ai/"
registry = "https://clawhub.ai/"
auth_base = "https://clawhub.ai/"
well_known = ["/.well-known/clawhub.json", "/.well-known/clawdhub.json"]
api_version = "v1"
trust_tier = "review"

[[adapters]]
name = "antigravity"
enabled = true
scope = "global"

[[adapters]]
name = "claude"
enabled = true
scope = "global"

[[adapters]]
name = "codex"
enabled = true
scope = "global"

[[adapters]]
name = "copilot"
enabled = true
scope = "global"

[[adapters]]
name = "cursor"
enabled = true
scope = "global"

[[adapters]]
name = "gemini"
enabled = true
scope = "global"

[[adapters]]
name = "kiro"
enabled = true
scope = "global"

[[adapters]]
name = "openclaw"
enabled = true
scope = "global"

[[adapters]]
name = "opencode"
enabled = true
scope = "global"

[[adapters]]
name = "trae"
enabled = true
scope = "global"

[[adapters]]
name = "vscode"
enabled = true
scope = "global"
```

`skillpm doctor` can also re-enable or backfill adapter entries in an existing
config when it detects a supported agent on disk.

---

## Project Manifest (`.skillpm/skills.toml`)

```toml
version = 1

[[skills]]
ref = "my-repo/code-review"
constraint = "^1.0"
```

| Field | Type | Description |
|-------|------|-------------|
| `version` | int | Schema version |
| `sources` | array | Optional source overrides for this project |
| `skills` | array | Skill dependencies |
| `skills[].ref` | string | Source-qualified skill reference |
| `skills[].constraint` | string | Version constraint |
| `skills[].deps` | string[] | Skill dependencies (auto-resolved on install) |
| `adapters` | array | Optional adapter overrides |

### `[[bundles]]`

Named groups of skills for batch installation.

```toml
[[bundles]]
name = "web-dev"
skills = ["clawhub/react", "clawhub/typescript", "clawhub/eslint"]

[[bundles]]
name = "security"
skills = ["community/secops/secret-scanner", "community/secops/api-fuzzer"]
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique bundle name |
| `skills` | string[] | Skill references to include |

Use `skillpm bundle create/list/install` to manage bundles. See [CLI Reference](cli-reference.md).

See [Project-Scoped Skills](project-scoped-skills.md) for usage.
