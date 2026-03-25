# code-reviewer

`code-reviewer` turns Claude Code or Codex into a focused production reviewer. It is designed for pull requests, risky refactors, release readiness checks, and regressions that need concrete findings instead of generic advice.

## What It Does

- Reviews diffs structurally, not just line-by-line
- Prioritizes correctness, security, performance, and rollout risk
- Reports missing tests when behavior changes are not covered
- Produces concise findings that an engineer can act on immediately

## Usage

### Claude Code

```text
Install: skillpm install clawhub/code-reviewer
Inject:  skillpm inject --agent claude clawhub/code-reviewer
Prompt:  "Use code-reviewer on the current diff. Findings first. Focus on regressions and missing tests."
```

### Codex

```text
Install: skillpm install clawhub/code-reviewer
Inject:  skillpm inject --agent codex clawhub/code-reviewer
Prompt:  "Use code-reviewer on cmd/skillpm/main.go and internal/app/service.go. Report only actionable findings."
```

## Output Shape

- Findings ordered by severity
- File references for every issue
- Production failure mode
- Smallest safe fix
- Residual risk when no bug is confirmed

## Test Cases

See [tests/cases.yaml](tests/cases.yaml) for repository-backed validation scenarios.
