# Supported Agents

> [Docs Index](index.md)

skillpm no longer treats every adapter as equally verified. This page records the discovery contract that `skillpm inject` targets for each agent and whether that contract is backed by official public documentation.

## Compatibility Matrix

| Agent | Config Key | Status | skillpm Global Target | skillpm Project Target | Discovery Mechanism | Format / Validation |
|-------|------------|--------|-----------------------|------------------------|---------------------|--------------------|
| Claude Code | `claude` | Verified | `~/.claude/skills/{name}/` | `<project>/.claude/skills/{name}/` | Claude loads personal, project, plugin, and nested `.claude/skills/` directories below the files you work on. | `SKILL.md` required. `description` recommended. `name` optional. |
| Codex | `codex` | Verified | `~/.agents/skills/{name}/` | `<project>/.agents/skills/{name}/` | Codex scans `.agents/skills` from the current working directory up to the repository root, plus user/admin/system locations. | `SKILL.md` required. Missing `name` or `description` triggers inject warnings because discoverability depends on them. |
| Gemini CLI | `gemini` | Verified | `~/.gemini/skills/{name}/` | `<project>/.gemini/skills/{name}/` | Gemini discovers workspace and user skills from `.gemini/skills` or the `.agents/skills` alias. Within the same tier, `.agents/skills` wins. | `SKILL.md` with `name` and `description`. Optional `scripts/`, `references/`, and `assets/`. |
| GitHub Copilot CLI | `copilot` | Verified | `~/.copilot/skills/{name}/` | `<project>/.github/skills/{name}/` | Copilot supports project skills in `.github/skills` or `.claude/skills`, and personal skills in `~/.copilot/skills` or `~/.claude/skills`. | `SKILL.md` with `name` and `description`. `name` must be lowercase with hyphens. |
| OpenCode | `opencode` | Verified | `~/.config/opencode/skills/{name}/` | `<project>/.opencode/skills/{name}/` | OpenCode also loads `.claude/skills` and `.agents/skills`, walking upward from the current working directory to the git worktree root. | `SKILL.md` with required `name` and `description`. Directory name must match `name`. |
| Kiro | `kiro` | Verified | `~/.kiro/skills/{name}/` | `<project>/.kiro/skills/{name}/` | Kiro loads workspace skills from `.kiro/skills/` and global skills from `~/.kiro/skills/`, with workspace taking precedence. | `SKILL.md` with required `name` and `description`. Directory name must match `name` and use lowercase letters, numbers, and hyphens. |
| OpenClaw | `openclaw` | Verified | `~/.openclaw/workspace/skills/{name}/` | `<project>/skills/{name}/` | OpenClaw loads workspace `skills/`, shared `~/.openclaw/skills`, and bundled skills. Workspace wins over shared, and shared wins over bundled. | `SKILL.md` with required `name` and `description`. |
| Antigravity | `antigravity` | Best-effort alias | `~/.gemini/skills/{name}/` | `<project>/.gemini/skills/{name}/` | Inferred from Gemini CLI IDE integration, which lists Antigravity as a supported IDE. No separate Antigravity skills contract is publicly documented. | Uses the Gemini CLI layout. Not independently validated. |
| Cursor | `cursor` | Best-effort | `~/.cursor/skills/{name}/` | `<project>/.cursor/skills/{name}/` | No current public Cursor discovery contract was validated in official docs during this audit. | `skillpm` only verifies `SKILL.md` presence. Treat as experimental. |
| TRAE | `trae` | Best-effort | `~/.trae/skills/{name}/` | `<project>/.trae/skills/{name}/` | No current public TRAE discovery contract was validated in official docs during this audit. | `skillpm` only verifies `SKILL.md` presence. Treat as experimental. |

## Shared / Alias Adapters

| Agent | Config Key | Contract |
|-------|------------|----------|
| VS Code (Copilot) | `vscode` | Uses the same skill contract as Copilot. Global inject target is `~/.copilot/skills/{name}/`; project inject target is `<project>/.github/skills/{name}/`. |

## Notes on Validation

- `skillpm inject` now validates the requested skill before writing adapter state.
- For verified agents that require `name` or `description`, `skillpm inject` synthesizes missing frontmatter into the copied `SKILL.md` and then validates the adapted result.
- Verified adapters fail fast when a copied skill cannot satisfy the documented discovery contract for that agent.
- Best-effort adapters remain available, but `skillpm` no longer claims end-to-end verification for them.
- OpenClaw project scope now targets the workspace root `skills/` directory, not `<project>/.openclaw/skills/`.
- Copilot and VS Code project scope now target `.github/skills/`, matching GitHub’s documented repository skill locations.
- Codex now injects to `.agents/skills/`, matching Codex’s repository and user discovery rules.

## Shared Paths

- VS Code and GitHub Copilot CLI share the same global user path: `~/.copilot/skills/`.
- Antigravity and Gemini CLI share the same `~/.gemini/skills/` and `<project>/.gemini/skills/` targets inside skillpm.
- OpenCode also discovers compatible skills from `.claude/skills/` and `.agents/skills/`, even though skillpm writes to `.opencode/skills/` by default.

To distinguish internal state for shared paths, skillpm keeps separate adapter state directories:

- Copilot CLI: `~/.copilot/skillpm/`
- VS Code: `~/.copilot/skillpm-vscode/`
- Gemini CLI: `~/.gemini/skillpm/`
- Antigravity: `~/.gemini/skillpm-antigravity/`

## How Detection Works

`skillpm doctor` auto-detects available agents by checking whether their root directory exists on disk. The logic (`DetectAvailable()` in `internal/adapter/detect.go`):

1. For each known agent, check if its root directory exists.
2. OpenClaw uses `OPENCLAW_STATE_DIR` when set, falling back to `~/.openclaw/state/`.
3. Detected agents are auto-enabled in the config on first run or when doctor runs.
4. Detection is deduplicated so each agent name appears at most once.

Detection answers "is this client installed?" It does not imply the adapter is fully verified. Use the matrix above for verification status.

## Enabling Adapters

Adapters can be enabled manually in `config.toml` or auto-enabled by `skillpm doctor`:

```toml
[[adapters]]
name = "claude"
enabled = true
scope = "global"
```

See [Config Reference](config-reference.md) for the full adapter schema.
