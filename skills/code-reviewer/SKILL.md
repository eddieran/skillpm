---
name: code-reviewer
version: 1.0.0
description: "Production-grade structural code review for correctness, security, performance, and release risk."
agents:
  - claude
  - codex
category: tool
tags:
  - review
  - security
  - performance
rules:
  paths:
    - "**/*.go"
    - "**/*.py"
    - "**/*.ts"
    - "**/*.tsx"
    - ".github/workflows/*.yml"
    - ".github/workflows/*.yaml"
  scope: project
  summary: "Review changed code for defects, regressions, unsafe assumptions, and missing tests."
---

# Code Reviewer

Use this skill when the user wants a real engineering review instead of lint-style commentary.

## Agent Compatibility

- Claude Code: supported
- Codex: supported

## Inputs

- A diff, commit range, pull request, or a focused set of changed files
- Any failing test output, benchmark output, or bug report that motivated the review

## Workflow

1. Determine the exact review scope before commenting.
2. Read surrounding code, not just the modified lines, until the behavior is clear.
3. Prioritize findings by user impact:
   - correctness and data loss
   - security and trust boundaries
   - concurrency and state corruption
   - performance regressions
   - migration and rollout risk
4. Call out missing coverage whenever behavior changed without tests.
5. Distinguish between confirmed defects, plausible risks, and open questions.

## Output Contract

- Findings first, ordered by severity.
- Each finding includes:
  - a short title
  - affected file paths
  - the concrete failure mode
  - why the issue matters in production
  - the smallest safe fix
- If no material findings remain, say so explicitly and mention residual testing gaps.

## Guardrails

- Do not flood the user with style nits.
- Do not speculate without a code path or failure scenario.
- Do not propose broad rewrites unless the current design is unsound.
