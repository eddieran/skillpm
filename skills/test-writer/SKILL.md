---
name: test-writer
version: 1.0.0
description: "Generate production-grade unit tests for Go, Python, and TypeScript with clear edge-case coverage."
agents:
  - claude
  - codex
category: agent
tags:
  - testing
  - go
  - python
  - typescript
rules:
  paths:
    - "**/*.go"
    - "**/*.py"
    - "**/*.ts"
    - "**/*.tsx"
    - "**/*.js"
  scope: project
  summary: "Write tests that exercise behavior, edge cases, and failure modes instead of snapshotting implementation details."
---

# Test Writer

Use this skill when the user wants high-signal unit tests from an existing source file or module.

## Agent Compatibility

- Claude Code: supported
- Codex: supported

## Inputs

- One or more source files
- Existing test style, helpers, and framework conventions when they exist
- Bug reports or regressions that the tests must lock in

## Workflow

1. Read the source file and nearby tests before writing anything.
2. Infer the existing test framework and naming conventions.
3. Cover the main behavior, edge cases, and failure paths.
4. Prefer deterministic tests over brittle snapshots.
5. Add only the smallest helpers needed to keep the tests readable.

## Output Contract

- Explain the behavioral areas covered.
- Add or update tests in the repository's existing style.
- Mention untestable paths, hidden dependencies, or coverage gaps.

## Guardrails

- Do not mock what the codebase already exercises cheaply with real objects.
- Do not encode internal implementation details when public behavior is enough.
- Do not stop at a happy-path test if error paths matter.
