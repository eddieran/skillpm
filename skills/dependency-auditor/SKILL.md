---
name: dependency-auditor
version: 1.0.0
description: "Audit npm, pip, cargo, and go dependencies for vulnerabilities, license risk, and stale versions."
agents:
  - claude
  - codex
category: security
tags:
  - dependencies
  - security
  - licenses
rules:
  paths:
    - "package.json"
    - "package-lock.json"
    - "pnpm-lock.yaml"
    - "yarn.lock"
    - "requirements.txt"
    - "requirements-dev.txt"
    - "pyproject.toml"
    - "poetry.lock"
    - "Cargo.toml"
    - "Cargo.lock"
    - "go.mod"
    - "go.sum"
  scope: project
  summary: "Audit dependency manifests and lockfiles for exposure, maintenance risk, and upgrade strategy."
---

# Dependency Auditor

Use this skill when the user needs a dependency audit that balances security, licensing, and upgrade risk.

## Agent Compatibility

- Claude Code: supported
- Codex: supported

## Inputs

- Dependency manifests and lockfiles
- Runtime context when a package is only used in tests or local tooling
- Policy constraints such as approved licenses or frozen release branches

## Workflow

1. Identify the package manager and lockfile state for each ecosystem.
2. Separate direct dependencies from transitive risk when possible.
3. Flag vulnerabilities, unsupported packages, and stale pins.
4. Call out license conflicts or unclear license metadata.
5. Recommend the smallest safe upgrade plan with likely blast radius.

## Output Contract

- High-priority risks first
- Package name, version, and why it matters
- Suggested remediation path
- Notes on lockfile drift or missing lockfiles

## Guardrails

- Do not recommend broad upgrades without naming the likely compatibility risk.
- Distinguish runtime dependencies from developer-only tooling.
- Prefer concrete remediation steps over generic “upgrade everything” advice.
