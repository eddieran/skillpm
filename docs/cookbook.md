# Cookbook

> [Docs Index](index.md) · [Getting Started](getting-started.md) · [CLI Reference](cli-reference.md)

Self-contained recipes for common skillpm workflows. Each recipe includes concrete commands you can copy and adapt.

---

## Recipe 1: Team Skill Sharing via Git

Share a curated set of skills across your team using a project manifest committed to version control.

### Setup (project lead)

```bash
cd ~/myproject
skillpm init
skillpm install clawhub/steipete/code-review
skillpm install clawhub/testingshop/auto-test-gen
skillpm install community/secops/secret-scanner

# Commit the manifest and lockfile
git add .skillpm/skills.toml .skillpm/skills.lock
git commit -m "add skillpm project config"
```

Add to `.gitignore`:

```
.skillpm/installed/
.skillpm/state.toml
.skillpm/staging/
.skillpm/snapshots/
```

### Onboarding (team members)

```bash
git clone <repo> && cd <repo>
skillpm sync
skillpm inject --all
```

`sync` reads the manifest and lockfile, installs the exact pinned versions, and re-injects into all configured agents.

### Upgrading skills

```bash
skillpm upgrade
git add .skillpm/skills.toml .skillpm/skills.lock
git commit -m "upgrade skills to latest"
```

Teammates pull the commit and run `skillpm sync` to get the updated versions.

---

## Recipe 2: CI/CD Integration

Use skillpm as a gate in your CI pipeline to enforce skill policies and verify reproducible installs.

### Basic CI gate

```bash
# Install skillpm in CI
go build -o ./bin/skillpm ./cmd/skillpm

# Bootstrap environment
./bin/skillpm doctor --json

# Sync and verify — strict mode exits 2 on policy violations
./bin/skillpm sync --strict --json > sync-result.json
```

### Exit code handling

| Exit Code | Meaning | CI Action |
|-----------|---------|-----------|
| `0` | Success, acceptable risk posture | Pass |
| `2` | Strict policy failure | Fail the build |
| other non-zero | Runtime/validation error | Fail the build |

### GitHub Actions example

```yaml
- name: Sync skills (strict)
  run: |
    skillpm doctor --json
    skillpm sync --strict --json > sync-result.json

- name: Upload sync report
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: sync-result
    path: sync-result.json
```

### Dry-run for pull request checks

Run a non-mutating dry-run on PRs to preview what would change:

```bash
skillpm sync --dry-run --strict --json > sync-plan.json
```

If exit code is `2`, the PR introduces a policy violation. The JSON output includes `recommendedCommand` fields for resolution.

---

## Recipe 3: Multi-Agent Setup

Inject skills into every AI agent you use with a single command.

### Inject into all detected agents

```bash
# Doctor auto-detects installed agents
skillpm doctor

# Inject all installed skills into every enabled agent
skillpm inject --all
```

### Inject specific skills to specific agents

```bash
# Code-review skill to Claude, linter to Codex
skillpm inject --agent claude my-repo/code-review
skillpm inject --agent codex my-repo/linter
```

### Verify agent injection paths

```bash
# Check which agents are detected and enabled
skillpm doctor --json | grep -A2 '"name": "config"'

# List skills with scope info
skillpm list
```

### Per-agent directories

| Agent | Global Injection Path |
|-------|----------------------|
| Claude Code | `~/.claude/skills/{name}/` |
| Codex | `~/.agents/skills/{name}/` |
| Gemini CLI | `~/.gemini/skills/{name}/` |
| Copilot | `~/.copilot/skills/{name}/` |
| Cursor | `~/.cursor/skills/{name}/` |

See [Supported Agents](agents.md) for the full list including IDE agents and project-scoped paths.

---

## Recipe 4: Skill Discovery Workflow

Discover skills through registered sources, direct Git URLs, and the bundled
official examples in this repository.

### Search across registered sources

```bash
# Search all registered sources
skillpm search "code-review"

# Search a specific source
skillpm search "testing" --source hub
```

### Install directly from a Git URL

```bash
skillpm install https://github.com/anthropics/skills/tree/main/skills/skill-creator --force
```

The first install auto-registers a reusable source name for the backing
repository. After that, you can treat it like any other source.

### Inspect the bundled official skills while developing locally

```bash
ls skills
find skills -maxdepth 2 -name SKILL.md | sort
```

---

## Recipe 5: Risk-Aware Sync Review

Use `sync --dry-run` and `sync --strict` to preview changes before mutating the
workspace or CI environment.

### Plan first

```bash
skillpm sync --dry-run --json > sync-plan.json
```

The JSON output includes:
- `recommendedCommand`
- `recommendedCommands`
- `riskStatus`
- `nextStepHint`

### Gate in CI

```bash
skillpm sync --strict --json > sync-result.json
echo $?
```

Exit code `2` means sync completed or planned with risk items that strict mode
does not allow through.

---

## Recipe 6: Security Review Workflow

Audit your skills for security issues and enforce scan policies.

### Scan on install

Security scanning is automatic. Every `install`, `upgrade`, and `sync` operation scans skill content:

```bash
# This will be blocked if the skill contains dangerous patterns
skillpm install my-repo/untrusted-skill
```

### Review a blocked install

When an install is blocked:

```
SEC_SCAN_BLOCKED: [HIGH] SCAN_DANGEROUS_PATTERN (SKILL.md: Code execution via subprocess.run); use --force to proceed
```

1. Read the error to identify the rule and pattern.
2. Inspect the skill content manually.
3. If you trust the content, bypass with `--force`:

```bash
skillpm install my-repo/admin-tool --force
```

Critical findings cannot be bypassed, even with `--force`.

### Configure scan policies

In `~/.skillpm/config.toml`:

```toml
[security.scan]
enabled = true
block_severity = "high"     # block high and critical (default)
disabled_rules = []         # e.g. ["SCAN_PROMPT_INJECTION"]
```

### Strict mode for CI

Use `--strict` in CI pipelines to fail on any risk items:

```bash
skillpm sync --strict --json > audit.json
echo "Exit code: $?"
```

Exit code `2` means the strict policy was violated. Parse the JSON output for details.

---

## Recipe 7: Manual or External Sync Cadence

`v4.x` removed the built-in scheduler. The supported workflow is to run sync
manually or wrap it in your own cron, CI, or task runner.

### Manual sync

```bash
# Preview what would change
skillpm sync --dry-run

# Apply changes
skillpm sync
```

### External automation example

```bash
# Cron, CI, or task runner command
skillpm sync --strict --json > sync-result.json
```

---

## Recipe 8: Troubleshooting Common Issues

### Skills not appearing in agent

```bash
# Step 1: Verify the skill is installed
skillpm list

# Step 2: Check that the agent adapter is enabled
skillpm doctor

# Step 3: Re-inject
skillpm inject --agent claude
```

### State corruption or drift

```bash
# Doctor detects and auto-fixes most issues
skillpm doctor
```

Doctor runs 7 checks in dependency order: config, state, installed-dirs, injections, adapter-state, agent-skills, and lockfile. It is idempotent -- safe to run repeatedly.

### Install blocked by security scan

```bash
# Read the error message for the specific rule
skillpm install my-repo/suspicious-skill

# If you trust it, bypass medium/high findings
skillpm install my-repo/suspicious-skill --force
```

Critical findings (destructive commands, reverse shells, crypto mining) cannot be bypassed.

### Project scope not detected

```bash
# Ensure you're inside a directory with .skillpm/skills.toml
ls .skillpm/skills.toml

# If not found, initialize the project
skillpm init
```

### Publishing fails because auth is missing

```bash
export CLAWHUB_TOKEN="your-token"
skillpm publish ./my-skill --version 1.0.0
```

### Getting help

```bash
# CLI help
skillpm --help
skillpm <command> --help

# JSON output for debugging
skillpm list --json
skillpm doctor --json
skillpm sync --dry-run --json
```

---

## Recipe 9: Skill Bundles for Team Stacks

Group related skills into bundles for easy batch installation.

### Define bundles

```bash
# Create a bundle in the project manifest
skillpm bundle create web-dev clawhub/react clawhub/typescript clawhub/eslint

# Create a security bundle
skillpm bundle create security community/secops/secret-scanner community/secops/api-fuzzer
```

### List and install bundles

```bash
# See all defined bundles
skillpm bundle list

# Install all skills in a bundle
skillpm bundle install web-dev

# Force-install if scan blocks
skillpm bundle install security --force
```

### Share with the team

Bundles are stored in `.skillpm/skills.toml`. Commit and share:

```bash
git add .skillpm/skills.toml
git commit -m "add web-dev and security bundles"
```

Teammates run `skillpm bundle install web-dev` after pulling.

---

## Recipe 10: Create, Test, and Publish Skills

Build a skill from scratch and publish it to the ClawHub registry.

### Scaffold

```bash
skillpm create my-linter --template default
cd my-linter
```

### Edit the skill

Open `SKILL.md` and customize the frontmatter and instructions:

```yaml
---
name: my-linter
version: 1.0.0
deps: [clawhub/eslint-config]
---
# My Linter

Instructions for the AI agent...
```

### Test locally

```bash
# Install from local directory
skillpm install ./my-linter

# Inject and verify
skillpm inject --agent claude
```

### Publish

```bash
export CLAWHUB_TOKEN="your-token"
skillpm publish ./my-linter --version 1.0.0
```

Others can now install it:

```bash
skillpm install clawhub/my-linter
```

---

## Next Steps

- [CLI Reference](cli-reference.md) -- full command documentation
- [Security Scanning](security-scanning.md) -- scan rules and enforcement details
- [Project-Scoped Skills](project-scoped-skills.md) -- team manifest workflow
- [Sync Contract v1](sync-contract-v1.md) -- JSON output schema for automation
- [Troubleshooting](troubleshooting.md) -- extended error reference
