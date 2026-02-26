# Supported Agents

> [Docs Index](index.md)

Skills are injected as folders into each agent's native `skills/` directory.

## CLI Agents

| Agent | Config Key | Injection Path | Detection Path | Docs |
|-------|-----------|---------------|----------------|------|
| Claude Code | `claude` | `~/.claude/skills/{name}/` | `~/.claude/` | [code.claude.com](https://code.claude.com/docs/en/skills) |
| Codex | `codex` | `~/.codex/skills/{name}/` | `~/.codex/` | [developers.openai.com](https://developers.openai.com/codex/skills/) |
| Gemini CLI | `gemini` | `~/.gemini/skills/{name}/` | `~/.gemini/` | [geminicli.com](https://geminicli.com/docs/cli/skills/) |
| GitHub Copilot CLI | `copilot` | `~/.copilot/skills/{name}/` | `~/.copilot/` | [docs.github.com](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills) |
| OpenCode | `opencode` | `~/.config/opencode/skills/{name}/` | `~/.config/opencode/` | [opencode.ai](https://opencode.ai/docs/skills/) |
| Kiro | `kiro` | `~/.kiro/skills/{name}/` | `~/.kiro/` | [kiro.dev](https://kiro.dev/docs/skills/) |
| OpenClaw | `openclaw` | `~/.openclaw/workspace/skills/{name}/` | `~/.openclaw/state/` | [docs.openclaw.ai](https://docs.openclaw.ai/tools/skills) |

## IDE / Desktop Agents

| Agent | Config Key | Injection Path | Detection Path | Docs |
|-------|-----------|---------------|----------------|------|
| VS Code (Copilot) | `vscode` | `~/.copilot/skills/{name}/` | `~/.vscode/` | [code.visualstudio.com](https://code.visualstudio.com/docs/copilot/customization/agent-skills) |
| Cursor | `cursor` | `~/.cursor/skills/{name}/` | `~/.cursor/` | [cursor.com](https://cursor.com/docs/context/skills) |
| TRAE | `trae` | `~/.trae/skills/{name}/` | `~/.trae/` | [docs.trae.ai](https://docs.trae.ai/ide/skills) |
| Antigravity | `antigravity` | `~/.gemini/skills/{name}/` | `~/.gemini/` | [antigravity.google](https://antigravity.google/docs/skills) |

## Shared Paths

- **VS Code + GitHub Copilot CLI** both inject into `~/.copilot/skills/`. Skills installed for one are visible to the other.
- **Antigravity + Gemini CLI** both inject into `~/.gemini/skills/`. Same sharing behavior.

To distinguish their internal state, skillpm uses separate target directories:
- Copilot CLI: `~/.copilot/skillpm/`
- VS Code: `~/.copilot/skillpm-vscode/`
- Gemini CLI: `~/.gemini/skillpm/`
- Antigravity: `~/.gemini/skillpm-antigravity/`

## Project-Scoped Injection Paths

When inside a project (`skillpm init`), skills are injected to project-local directories instead:

| Agent | Project Injection Path |
|-------|----------------------|
| Claude Code | `<project>/.claude/skills/{name}/` |
| Codex | `<project>/.codex/skills/{name}/` |
| Gemini / Antigravity | `<project>/.gemini/skills/{name}/` |
| Copilot / VS Code | `<project>/.copilot/skills/{name}/` |
| Cursor | `<project>/.cursor/skills/{name}/` |
| TRAE | `<project>/.trae/skills/{name}/` |
| OpenCode | `<project>/.opencode/skills/{name}/` |
| OpenClaw | `<project>/.openclaw/skills/{name}/` |
| Kiro | `<project>/.kiro/skills/{name}/` |

## How Detection Works

`skillpm doctor` auto-detects available agents by checking whether their root directory exists on disk. The logic (`DetectAvailable()` in `internal/adapter/detect.go`):

1. For each known agent, check if its root directory exists (e.g., `~/.claude/` for Claude).
2. OpenClaw uses `OPENCLAW_STATE_DIR` env var, falling back to `~/.openclaw/state/`.
3. Detected agents are auto-enabled in the config on first run or when doctor runs.
4. Detection is deduplicated — each agent name appears at most once.

## Enabling Adapters

Adapters can be enabled manually in `config.toml` or auto-enabled by `skillpm doctor`:

```toml
[[adapters]]
name = "claude"
enabled = true
scope = "global"
```

See [Config Reference](config-reference.md) for the full adapter schema.
