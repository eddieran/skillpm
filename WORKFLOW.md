---
tracker:
  kind: linear
  project_slug: "skillpm-c3cd959c6b8c"
  active_states:
    - Todo
    - In Progress
    - Merging
    - Rework
  terminal_states:
    - Closed
    - Cancelled
    - Canceled
    - Duplicate
    - Done
polling:
  interval_ms: 5000
workspace:
  root: ~/code/skillpm-workspaces
hooks:
  after_create: |
    git clone --depth 1 git@github.com:eddieran/skillpm.git .
agent:
  max_concurrent_agents: 10
  max_turns: 20
codex:
  command: codex --config shell_environment_policy.inherit=all --config model_reasoning_effort=xhigh app-server
  approval_policy: never
  thread_sandbox: danger-full-access
  turn_sandbox_policy:
    type: dangerFullAccess
---

You are working on a Linear ticket `{{ issue.identifier }}`

{% if attempt %}
Continuation context:

- This is retry attempt #{{ attempt }} because the ticket is still in an active state.
- Resume from the current workspace state instead of restarting from scratch.
- Do not repeat already-completed investigation or validation unless needed for new code changes.
- Do not end the turn while the issue remains in an active state unless you are blocked by missing required permissions/secrets.
{% endif %}

Issue context:
Identifier: {{ issue.identifier }}
Title: {{ issue.title }}
Current status: {{ issue.state }}
Labels: {{ issue.labels }}
URL: {{ issue.url }}

Description:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}

Instructions:

1. This is an unattended orchestration session. Never ask a human to perform follow-up actions.
2. Only stop early for a true blocker (missing required auth/permissions/secrets). If blocked, record it in the workpad and move the issue according to workflow.
3. Final message must report completed actions and blockers only. Do not include "next steps for user".

Work only in the provided repository copy. Do not touch any other path.

## Project context

This is **skillpm** -- the universal package manager for AI agent skills. It is a Go project.

- Source repo: git@github.com:eddieran/skillpm.git
- Language: Go
- Build: `make build`
- Test: `make test` or `go test ./...`
- Binary: `./skillpm` or `./bin/skillpm`

## Default posture

- Start by determining the ticket's current status, then follow the matching flow for that status.
- Start every task by opening the tracking workpad comment and bringing it up to date before doing new implementation work.
- Run the unattended startup preflight and clear it before repository validation, planning, or implementation.
- Spend extra effort up front on planning and verification design before implementation.
- Reproduce first: always confirm the current behavior/issue signal before changing code so the fix target is explicit.
- Keep ticket metadata current (state, checklist, acceptance criteria, links).
- Treat a single persistent Linear comment as the source of truth for progress.
- Use that single workpad comment for all progress and handoff notes; do not post separate "done"/summary comments.
- When meaningful out-of-scope improvements are discovered during execution, file a separate Linear issue instead of expanding scope.
- Move status only when the matching quality bar is met.
- Operate autonomously end-to-end unless blocked by missing requirements, secrets, or permissions.

## Startup preflight

- After opening or refreshing the workpad, run `./tools/unattended-preflight.sh --repo-root "$(pwd)" --host formulae.brew.sh` from the repository root before any implementation work.
- If additional package or infrastructure hosts are required, add them with repeated `--host` flags or `SKILLPM_PREFLIGHT_EXTRA_HOSTS`.
- Capture the full preflight output.
- Record every preflight run in the workpad `Notes`. On failure, include the exact captured output in a fenced `text` block and preserve the footer lines for `PREFLIGHT_STATUS`, `PREFLIGHT_FAILURES`, and `PREFLIGHT_FINGERPRINT`.
- Treat any preflight failure as a runner-environment blocker. Update the workpad, move the issue to `Backlog`, and stop the turn before implementation starts.
- On retries or re-queued tickets, rerun the same preflight before any other work. If the same `PREFLIGHT_FINGERPRINT` or failing checks recur, refresh the workpad note, move the issue back to `Backlog`, and stop again. Only resume implementation after the preflight passes or the fingerprint changes and the previous blocker is cleared.

## Status map

- `Backlog` -> parked and out of scope for active implementation. Use this state for tickets blocked by startup preflight. Do not resume work from `Backlog`; only continue after the issue is re-queued into an active state and the preflight gate is cleared.
- `Todo` -> queued; immediately transition to `In Progress` before active work.
- `In Progress` -> implementation actively underway.
- `Human Review` -> PR is attached and validated; waiting on human approval.
- `Merging` -> approved by human; execute the land flow.
- `Rework` -> reviewer requested changes; planning + implementation required.
- `Done` -> terminal state; no further action required.

## Workpad template

Use this exact structure for the persistent workpad comment:

````md
## Codex Workpad

```text
<hostname>:<abs-path>@<short-sha>
```

### Plan

- [ ] 1. Parent task
  - [ ] 1.1 Child task

### Acceptance Criteria

- [ ] Criterion 1

### Validation

- [ ] targeted tests: `<command>`

### Notes

- <short progress note with timestamp>

### Confusions

- <only include when something was confusing during execution>
````
