# git-conventional

`git-conventional` keeps commits, changelog entries, and semantic version bumps aligned. It is useful when landing a feature set, preparing a release, or cleaning up a branch before review.

## What It Does

- Suggests conventional commit subjects and bodies
- Groups changes into changelog-ready release notes
- Recommends major, minor, or patch bumps with justification
- Surfaces ambiguous scopes and hidden breaking changes

## Usage

### Claude Code

```text
Install: skillpm install clawhub/git-conventional
Inject:  skillpm inject --agent claude clawhub/git-conventional
Prompt:  "Use git-conventional on the current branch and draft the best commit message plus changelog note."
```

### Codex

```text
Install: skillpm install clawhub/git-conventional
Inject:  skillpm inject --agent codex clawhub/git-conventional
Prompt:  "Use git-conventional to classify these changes and recommend the next semantic version."
```

## Good Outputs

- `fix(publish): preserve nested skill files during registry round trips`
- `feat(leaderboard): default to live trending data from configured registry`
- Release notes grouped into `Features`, `Fixes`, and `Breaking Changes`

## Test Cases

See [tests/cases.yaml](tests/cases.yaml) for repository-backed release scenarios.
