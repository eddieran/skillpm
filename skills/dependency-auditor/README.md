# dependency-auditor

`dependency-auditor` reviews dependency manifests across npm, pip, cargo, and Go modules. It is intended for release readiness checks, upgrade planning, and remediation work where security and maintenance risk matter.

## What It Does

- Audits direct and transitive dependency risk
- Flags missing or stale lockfiles
- Highlights license conflicts and unclear metadata
- Recommends the smallest safe upgrade plan

## Usage

### Claude Code

```text
Install: skillpm install clawhub/dependency-auditor
Inject:  skillpm inject --agent claude clawhub/dependency-auditor
Prompt:  "Use dependency-auditor on this repository. Focus on go.mod, go.sum, and any release-blocking dependency risk."
```

### Codex

```text
Install: skillpm install clawhub/dependency-auditor
Inject:  skillpm inject --agent codex clawhub/dependency-auditor
Prompt:  "Use dependency-auditor on the manifests in this monorepo and propose the smallest safe upgrade plan."
```

## Expected Output

- Critical and high-risk packages first
- Ecosystem-specific notes
- Upgrade sequencing
- Residual risk if the repo cannot move immediately

## Test Cases

See [tests/cases.yaml](tests/cases.yaml) for repository-backed audit scenarios.
