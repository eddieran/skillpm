---
name: doc-sync
version: 1.0.0
description: "Detect stale documentation by comparing code, CLI surface area, and docs for drift."
agents:
  - claude
  - codex
category: workflow
tags:
  - docs
  - api
  - cli
rules:
  paths:
    - "README.md"
    - "docs/**/*.md"
    - "cmd/**/*.go"
    - "internal/**/*.go"
  scope: project
  summary: "Compare implementation details against docs and report any stale commands, flags, or behavioral guarantees."
---

# Doc Sync

Use this skill when code changes may have made READMEs, CLI docs, API docs, or examples stale.

## Agent Compatibility

- Claude Code: supported
- Codex: supported

## Inputs

- The changed code or commit range
- The documentation surface that claims to describe that code
- Release notes, CLI help output, or API schemas when available

## Workflow

1. Enumerate the behavior that changed in code.
2. Find the documentation pages that claim to describe that behavior.
3. Compare signatures, commands, flags, examples, and defaults.
4. Report stale docs before proposing rewrites.
5. Prefer minimal, accurate doc updates over broad rewording.

## Output Contract

- Stale documentation findings first
- Exact files that drifted
- The implementation fact that now differs
- The smallest accurate doc update

## Guardrails

- Do not rewrite docs that are already correct.
- Do not invent undocumented features.
- Distinguish between wording polish and factual drift.
