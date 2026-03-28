# test-writer

`test-writer` generates unit tests that fit the repository's conventions instead of producing isolated demo tests. It supports Go, Python, and TypeScript workflows and is tuned for behavior-first coverage.

## What It Does

- Reads the source file and nearby tests before generating coverage
- Preserves the local test framework and naming style
- Covers happy paths, edge cases, and failure behavior
- Explains what is still risky or hard to test

## Usage

### Claude Code

```text
Install: skillpm install clawhub/test-writer
Inject:  skillpm inject --agent claude clawhub/test-writer
Prompt:  "Use test-writer on internal/source/clawhub_provider.go and add focused unit tests."
```

### Codex

```text
Install: skillpm install clawhub/test-writer
Inject:  skillpm inject --agent codex clawhub/test-writer
Prompt:  "Use test-writer on internal/app/service.go. Cover sorting, filters, and failure paths."
```

## Good Targets

- Parsing and normalization code
- Service methods with clear inputs and outputs
- Regression fixes that need one locked-in behavior

## Test Cases

See [tests/cases.yaml](tests/cases.yaml) for repository-backed generation scenarios.
