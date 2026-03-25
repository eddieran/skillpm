# doc-sync

`doc-sync` compares the repository's implementation against its documentation and flags factual drift. It is useful after CLI changes, API changes, release workflow changes, and any refactor that alters user-facing behavior.

## What It Does

- Compares CLI commands and flags against docs
- Detects stale README and cookbook examples
- Checks whether default behavior changed without a doc update
- Keeps findings factual and easy to patch

## Usage

### Claude Code

```text
Install: skillpm install clawhub/doc-sync
Inject:  skillpm inject --agent claude clawhub/doc-sync
Prompt:  "Use doc-sync on the leaderboard and publish changes. Report docs that are now stale."
```

### Codex

```text
Install: skillpm install clawhub/doc-sync
Inject:  skillpm inject --agent codex clawhub/doc-sync
Prompt:  "Use doc-sync on cmd/skillpm/main.go and docs/cli-reference.md. Focus on factual drift only."
```

## Good Targets

- CLI help versus `docs/cli-reference.md`
- Cookbook recipes versus the current implementation
- README examples versus real command behavior

## Test Cases

See [tests/cases.yaml](tests/cases.yaml) for repository-backed documentation drift scenarios.
