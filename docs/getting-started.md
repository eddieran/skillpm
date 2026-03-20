# Getting Started with skillpm

> [Docs Index](index.md) · [CLI Reference](cli-reference.md) · [Cookbook](cookbook.md)

This guide walks you through installing skillpm, discovering your first skill, injecting it into your AI agent, and setting up a project for team collaboration.

## Prerequisites

| Tool | Minimum Version | Check |
|------|----------------|-------|
| **Go** | 1.21+ | `go version` |
| **Git** | 2.x | `git --version` |

Both must be available in your `PATH`.

## Installation

### Option A: Homebrew (recommended)

```bash
brew tap eddieran/tap && brew install skillpm
skillpm version
```

### Option B: `go install`

```bash
go install github.com/eddieran/skillpm@latest
skillpm version
```

### Option C: Build from source

```bash
git clone https://github.com/eddieran/skillpm.git
cd skillpm
make build
./bin/skillpm version
```

### Option D: Binary download

Download the latest release from [GitHub Releases](https://github.com/eddieran/skillpm/releases), extract the archive, and move the `skillpm` binary to a directory in your `PATH`:

```bash
# Example for macOS arm64
tar xzf skillpm_darwin_arm64.tar.gz
sudo mv skillpm /usr/local/bin/
skillpm version
```

## Initial Setup

Run `doctor` to bootstrap your environment. It creates the config file, detects installed agents, and enables their adapters automatically:

```bash
skillpm doctor
```

Expected output:

```
[ok   ] config           config valid
[ok   ] state            state valid
[ok   ] installed-dirs   installed dirs ok
[ok   ] injections       injection refs valid
[ok   ] adapter-state    adapter state synced
[ok   ] agent-skills     agent skill files present
[ok   ] lockfile         lock entries verified

done: 0 fixed
```

If any checks show `[fixed]`, doctor auto-repaired the issue. Run it again to confirm everything is `[ok]`.

## Your First Skill

This walkthrough covers the full lifecycle: register a source, search, install, inject, and verify.

### 1. Register a source

Sources are where skillpm finds skills. You can use Git repositories or ClawHub registries:

```bash
# Add a Git-based source
skillpm source add my-repo https://github.com/org/skills.git --kind git

# Or add the ClawHub registry
skillpm source add hub https://clawhub.ai/ --kind clawhub

# Verify
skillpm source list
```

### 2. Search for a skill

```bash
skillpm search "code-review"
```

You can restrict results to a specific source:

```bash
skillpm search "code-review" --source hub
```

### 3. Install the skill

Install using the `source/skill` format:

```bash
skillpm install my-repo/code-review
```

Or install directly from any Git URL without registering a source first:

```bash
skillpm install https://github.com/anthropics/skills/tree/main/skills/skill-creator
```

All skills are automatically scanned for dangerous content before installation. If the scan detects critical or high severity issues, the install is blocked. Use `--force` to bypass medium-severity findings if you trust the content.

### 4. Inject into your agent

Push the installed skill into your agent's native `skills/` directory:

```bash
skillpm inject --agent claude
```

Or inject into all detected agents at once:

```bash
skillpm inject --all
```

### 5. Verify

```bash
# List installed skills
skillpm list

# Check that the skill files exist in the agent directory
ls ~/.claude/skills/
```

You should see the skill folder in the agent's skill directory, ready for the agent to use.

## Understanding Scopes

skillpm has two scopes that work like `npm install` (local) vs `npm install -g` (global):

| Scope | State Directory | Use Case |
|-------|----------------|----------|
| **Global** | `~/.skillpm/` | Personal skills available everywhere |
| **Project** | `<project>/.skillpm/` | Team-shared skills pinned per repository |

### Auto-detection

When you run skillpm inside a directory containing `.skillpm/skills.toml` (or any parent that has one), it automatically uses **project scope**. Outside a project, it defaults to **global scope**.

### Explicit override

Force a specific scope with `--scope`:

```bash
# Install globally even when inside a project
skillpm install my-repo/helper --scope global

# Install at project scope explicitly
skillpm install my-repo/helper --scope project

# List only global skills
skillpm list --scope global
```

### Scope isolation

Project and global scopes are fully isolated. Installing at project scope never affects global state, and vice versa. Each scope has its own state files, lockfiles, and injection paths.

## Project Setup

Set up skillpm for a team repository so everyone shares the same skills at the same versions.

### 1. Initialize the project

```bash
cd ~/myproject
skillpm init
```

This creates `.skillpm/skills.toml` -- the project manifest.

### 2. Install skills

```bash
skillpm install my-repo/code-review
skillpm install clawhub/steipete/code-review
```

Since you are inside a project, these install at project scope automatically. The manifest and lockfile are updated.

### 3. Commit the manifest

```bash
git add .skillpm/skills.toml .skillpm/skills.lock
git commit -m "add skillpm project config"
```

Add these to `.gitignore`:

```
.skillpm/installed/
.skillpm/state.toml
.skillpm/staging/
.skillpm/snapshots/
```

`skillpm init` prints these suggestions after creating the manifest.

### 4. Team members onboard

When a teammate clones the repository:

```bash
git clone <repo> && cd <repo>
skillpm sync
```

This reads the manifest and lockfile, installs the exact pinned versions, and injects into their configured agents.

### 5. Upgrade skills

Anyone on the team can upgrade and commit the new lockfile:

```bash
skillpm upgrade
git add .skillpm/skills.toml .skillpm/skills.lock
git commit -m "upgrade skills"
```

## Agent-Specific Tips

### Claude Code

- **Config key**: `claude`
- **Global injection path**: `~/.claude/skills/{name}/`
- **Project injection path**: `<project>/.claude/skills/{name}/`

```bash
skillpm inject --agent claude
```

Claude Code reads skills from its `skills/` directory automatically. No additional configuration needed.

### Codex (OpenAI)

- **Config key**: `codex`
- **Global injection path**: `~/.agents/skills/{name}/`
- **Project injection path**: `<project>/.agents/skills/{name}/`

```bash
skillpm inject --agent codex
```

### Gemini CLI

- **Config key**: `gemini`
- **Global injection path**: `~/.gemini/skills/{name}/`
- **Project injection path**: `<project>/.gemini/skills/{name}/`

```bash
skillpm inject --agent gemini
```

Gemini CLI and Antigravity share the same injection path (`~/.gemini/skills/`). Skills installed for one are visible to the other.

### Cursor

- **Config key**: `cursor`
- **Global injection path**: `~/.cursor/skills/{name}/`
- **Project injection path**: `<project>/.cursor/skills/{name}/`

```bash
skillpm inject --agent cursor
```

### Copilot (CLI + VS Code)

- **Config key**: `copilot` (CLI) / `vscode` (VS Code)
- **Global injection path**: `~/.copilot/skills/{name}/`
- **Project injection path**: `<project>/.github/skills/{name}/`

```bash
skillpm inject --agent copilot
```

GitHub Copilot CLI and VS Code Copilot share the same global injection path. Repository-local skills follow GitHub's documented `.github/skills/` contract.

### Multi-agent injection

Push skills to every detected agent in a single command:

```bash
skillpm inject --all
```

## Creating Your Own Skills

Use `skillpm create` to scaffold a new skill from a template:

```bash
# Default template
skillpm create my-skill

# Prompt-based skill
skillpm create my-prompt --template prompt

# Script-based skill
skillpm create my-script --template script
```

This creates a directory with a ready-to-use `SKILL.md` including frontmatter. Edit the generated file to add your skill's instructions.

### Declaring Dependencies

Add a `deps` field to your SKILL.md frontmatter to declare dependencies on other skills:

```yaml
---
name: my-skill
version: 1.0.0
deps: [clawhub/base-skill, clawhub/util-skill]
---
```

Or use block list format:

```yaml
---
name: my-skill
version: 1.0.0
deps:
  - clawhub/base-skill
  - clawhub/util-skill
---
```

When someone installs your skill, dependencies are resolved and installed automatically.

### Publishing to ClawHub

Once your skill is ready, publish it:

```bash
export CLAWHUB_TOKEN="your-token"
skillpm publish ./my-skill --version 1.0.0
```

## Next Steps

- [Creating Your Own Skills](#creating-your-own-skills) -- scaffold, declare dependencies, and publish skills
- [Cookbook](cookbook.md) -- common recipes for team sharing, CI/CD, multi-agent setups
- [CLI Reference](cli-reference.md) -- all commands, flags, and exit codes
- [Procedural Memory](procedural-memory.md) -- self-adaptive skill activation
- [Security Scanning](security-scanning.md) -- scan rules and enforcement
- [Supported Agents](agents.md) -- full list of agents and injection paths
- [Troubleshooting](troubleshooting.md) -- common errors and fixes
