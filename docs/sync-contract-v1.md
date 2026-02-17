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
- `outcome` (string): `noop` | `changed` | `changed-with-risk` | `blocked`
- `hasProgress` (bool)
- `hasRisk` (bool)
- `canProceed` (bool): `true` when no reinjection risk blocks next iteration
- `progressStatus` (string): `progress-made` | `no-progress`
- `riskStatus` (string): `clear` | `attention-needed`
- `riskLevel` (string): `none` | `medium` | `high`
- `summaryLine` (string): one-line summary for logs
- `nextAction` (string): short next-step hint
- `executionPriority` (string): execution strategy hint
- `primaryAction` (string): primary actionable recommendation
- `recommendedCommand` (string): single command suggestion
- `recommendedCommands` (array[string]): prioritized command sequence
- `recommendedAgent` (string, optional)
- `noopReason` (string, optional)

## Breakdown and detail fields (stable)

- `actionBreakdown` (object)
- `riskBreakdown` (object)
- `riskHotspot` (string, optional)
- `progressHotspot` (string, optional)
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
