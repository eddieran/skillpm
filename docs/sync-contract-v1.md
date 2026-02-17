# sync JSON Contract v1 (Beta)

This document defines the **machine-readable contract** for `skillpm sync --json`.

## Compatibility policy

- `schemaVersion` follows semantic style: `v1` for this contract.
- Within `v1`:
  - Existing fields documented here are **stable**.
  - New fields may be added in minor releases.
  - Existing field meaning/type will not change without a version bump.
- Consumers must ignore unknown fields.

## Top-level fields (stable)

- `schemaVersion` (string)
- `mode` (string): `dry-run` | `apply`
- `strictMode` (bool): whether `--strict` is enabled for this run
- `strictStatus` (string): `enabled` | `disabled`
- `strictFailureReason` (string): `strict-disabled` | `none` | `risk-present-failed` | `risk-present-skipped` | `risk-present-mixed`
- `outcome` (string): `noop` | `changed` | `changed-with-risk` | `blocked`
- `hasProgress` (bool)
- `hasRisk` (bool)
- `canProceed` (bool): `true` when no reinjection risk blocks next iteration
- `nextBatchReady` (bool): `true` when sync ran in apply mode and no reinjection risk remains
- `nextBatchBlocker` (string): `none` | `risk-present` | `dry-run-mode` | `unknown`
- `progressStatus` (string): `progress-made` | `no-progress`
- `riskStatus` (string): `clear` | `attention-needed`
- `riskLevel` (string): `none` | `medium` | `high`
- `summaryLine` (string): one-line summary for logs
- `nextAction` (string): short next-step hint
- `nextStepHint` (string): machine-readable next-step classifier for orchestration
- `executionPriority` (string): execution strategy hint
- `primaryAction` (string): primary actionable recommendation
- `recommendedCommand` (string): single command suggestion
- `recommendedCommands` (array[string]): prioritized command sequence
- `recommendedAgent` (string, optional)
- `noopReason` (string, optional)

## Breakdown and detail fields (stable)

- `actionCounts` (object): detailed counters for sources, upgrades, risks, etc.
- `riskCounts` (object): detailed counters for risk items.
- `topSamples` (object): sample items for sources, upgrades, risks.
- `actionBreakdown` (string): summary string of actions.
- `riskBreakdown` (string): summary string of risks.
- `riskHotspot` (string, optional)
- `progressHotspot` (string, optional)
- `riskAgents` (array[string]): list of agent names involved in risk.
- `riskAgentsTotal` (int)
- `riskInjectCommands` (array[string]): suggested injection commands for risk items.
- `updatedSources` (array[string])
- `upgradedSkills` (array[string])
- `reinjected` (array[string])
- `skippedReinjects` (array[string])
- `failedReinjects` (array[string])

## Exit code contract (`sync --strict`)

- `0`: successful sync and acceptable risk posture.
- `2`: strict risk failure (sync completed but policy rejects risk state).
- non-zero (except 2): execution/runtime/validation failure.

## Consumer guidance

- Prefer `outcome`, `hasRisk`, `riskLevel`, and `recommendedCommands` for automation.
- Do not parse human-readable console text for machine logic.
- Keep fallback behavior for missing optional fields.
