# castaway-web Production Readiness Checklist

## Security
- [ ] Authentication approach selected and documented
- [ ] Authorization model documented for any write-capable workflows
- [ ] Secrets provided through managed secret storage
- [ ] Public exposure reviewed for TLS, ingress, and network policy

## Reliability
- [ ] Database backup and restore procedure documented
- [ ] Migration rollout and rollback procedure documented
- [ ] Health checks wired into the deployment environment
- [ ] Seed/dev-only workflows clearly separated from production operations

## Observability
- [ ] Structured logging verified in the deployed environment
- [ ] Alerting defined for API downtime and startup failures
- [ ] Runbook documented for database outages and application restarts

## Contract discipline
- [ ] OpenAPI generation check enforced in CI
- [ ] Route parity check enforced in CI
- [ ] Public API docs updated for externally visible changes

## Follow-up thread

- `plans/auth-and-authorization-planning.md`

## Operational status

Current state: suitable for local development; production rollout still requires auth, runbooks, and deployment hardening.
