# Castaway Non-Functional Requirements

This document records the repository-level non-functional requirements for Castaway.
Detailed shared guidance also exists in `docs/non-functional-requirements.md`.

## Security

- Secrets must not be committed.
- Production-facing services must have an explicit authentication and authorization model.
- Logs must avoid leaking secrets or sensitive payload data.

## Reliability

- The monorepo developer workflow must stay reproducible through shared tooling.
- Cross-app integrations must fail clearly when dependencies are unavailable.
- Historical scoring and seed workflows must remain repeatable.

## Operations

- Build, lint, and test workflows must remain documented and runnable from the repository.
- Shared release, rollback, and runbook expectations must be captured before production rollout.
- Shared documentation must stay current as app responsibilities evolve.

## Observability

- Production services must expose health checks and actionable logs.
- Alerts and runbooks are required before public production rollout.
