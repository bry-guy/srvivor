---
description: Fixes the current build via fixing linting, compilation, test failures).
---
**TARGET APP:** $ARGUMENTS

## Goal

Fix the current build. If $ARGUMENTS are passed, look for that app specifically within the repo and run commands directly against that.

## Contraints

* ALWAYS refer to AGENTS.md before executing this workflow.
* NEVER run custom commands (e.g. `golangci-lint`) instead of using pre-specified tooling (e.g., `make`, `mise`).
* NEVER modify regression tests.
* ASK PERMISSION to move or rename files.

## Workflow

1. Fix lint issues.
2. Fix compilation issues.
3. Fix test failures.
