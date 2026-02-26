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
| `SKILLPM_HOME` | Override the skillpm home directory (default: `~/.skillpm`) |
| `SKILLPM_CONFIG` | Override the config file path |
| `SKILLPM_STATE_DIR` | Override the state directory |
| `SKILLPM_LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` |
| `SKILLPM_NO_COLOR` | Disable colored output when set to `1` |
| `SKILLPM_SCAN_ENABLED` | Override scan enabled setting (`true`/`false`) |
| `SKILLPM_SCAN_SEVERITY` | Override block severity threshold |
| `SKILLPM_FORCE` | Override force flag for all operations |

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
| `mode` | string | `"system"` | Sync mode |
| `interval` | string | `"6h"` | Auto-sync interval |

### `[security]`

```toml
[security]
profile = "strict"
require_signatures = true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `profile` | string | `"strict"` | Security profile |
| `require_signatures` | bool | `true` | Require signed skills |

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

### `[memory]`

```toml
[memory]
enabled = false
working_memory_max = 12
threshold = 0.3
recency_half_life = "7d"
observe_on_sync = false
adaptive_inject = false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable the procedural memory subsystem |
| `working_memory_max` | int | `12` | Maximum skills in working memory |
| `threshold` | float | `0.3` | Minimum activation score to enter working memory |
| `recency_half_life` | string | `"7d"` | Recency decay half-life: `"3d"`, `"7d"`, `"14d"` |
| `observe_on_sync` | bool | `false` | Auto-observe after sync |
| `adaptive_inject` | bool | `false` | Use adaptive injection by default |

See [Procedural Memory](procedural-memory.md) for details.

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
name = "clawhub"
kind = "clawhub"
site = "https://clawhub.ai/"
registry = "https://clawhub.ai/"
auth_base = "https://clawhub.ai/"
well_known = ["/.well-known/clawhub.json"]
api_version = "v1"
trust_tier = "review"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Unique source name |
| `kind` | string | yes | `git` or `clawhub` |
| `url` | string | git only | Git repository URL |
| `branch` | string | no | Git branch (default: repo default) |
| `scan_paths` | string[] | no | Subdirectories containing skills |
| `trust_tier` | string | yes | `review` or `trusted` |
| `site` | string | clawhub | Registry site URL |
| `registry` | string | clawhub | API registry URL |
| `auth_base` | string | clawhub | Authentication base URL |
| `well_known` | string[] | clawhub | Well-known discovery paths |
| `api_version` | string | clawhub | API version string |

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

A fresh `skillpm doctor` or first run generates this config:

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
api_version = "v1"
trust_tier = "review"

[[sources]]
name = "community"
kind = "clawhub"
site = "https://clawhub.ai/"
registry = "https://clawhub.ai/"
auth_base = "https://clawhub.ai/"
api_version = "v1"
trust_tier = "review"

[[adapters]]
name = "codex"
enabled = true
scope = "global"

[[adapters]]
name = "openclaw"
enabled = true
scope = "global"
```

Additional adapters are auto-enabled by `skillpm doctor` when their root directories are detected.

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
| `adapters` | array | Optional adapter overrides |

See [Project-Scoped Skills](project-scoped-skills.md) for usage.
