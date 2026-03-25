---
name: git-conventional
version: 1.0.0
description: "Enforce conventional commits, draft changelog entries, and suggest semantic version bumps."
agents:
  - claude
  - codex
category: workflow
tags:
  - git
  - changelog
  - semver
rules:
  paths:
    - "CHANGELOG.md"
    - ".github/workflows/*.yml"
    - ".github/workflows/*.yaml"
    - "go.mod"
    - "package.json"
    - "Cargo.toml"
    - "pyproject.toml"
  scope: project
  summary: "Classify changes with conventional commit types and map them to changelog and version bump decisions."
---

# Git Conventional

Use this skill when the user wants consistent commit messages, release notes, or a version bump recommendation.

## Agent Compatibility

- Claude Code: supported
- Codex: supported

## Inputs

- A diff, commit list, or branch summary
- The current release version when known
- Any project-specific commit prefixes or release policy

## Workflow

1. Classify each change using conventional commit semantics.
2. Highlight ambiguous changes that need a tighter scope or clearer description.
3. Draft changelog entries grouped by change type.
4. Recommend the smallest valid semantic version bump.
5. Call out breaking changes explicitly.

## Output Contract

- Suggested commit subject line
- Optional body bullets when the change deserves context
- Changelog entry grouped by type
- Version bump recommendation with rationale

## Guardrails

- Do not overstate a breaking change without an actual compatibility break.
- Do not hide risk behind vague scopes like `misc` or `update`.
- Prefer one coherent commit story over a noisy list of file names.
