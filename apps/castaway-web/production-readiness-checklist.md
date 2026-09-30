# castaway-web Production Readiness Checklist

## Security
- [x] Authentication approach selected and documented — bearer-token service auth, see `plans/service-to-service-authentication-planning.md`
- [x] Bot-to-API bearer-token authentication enforced on all production routes except `/healthz` — implemented in `internal/httpapi/auth.go`, tested in `auth_test.go`
- [ ] Authorization model documented for any write-capable workflows — see `docs/security-audit-web-discord-identity.md` for identified gaps
- [ ] Secrets provided through managed secret storage
- [ ] Public exposure reviewed for TLS, ingress, and network policy

## Castawordle preview
- [x] Content-page session gates and own-player/cross-instance API authorization covered offline.
- [x] Private answers, duplicate-letter feedback, cutoff, concurrent/retried moves, and zero scoring effects covered in disposable PostgreSQL.
- [x] 320px/desktop, themes, touch/physical input, fake OAuth return, lost-response recovery, and resume checked with Playwright.
- [x] Feature image deployed and public page gates/OAuth entry/CLI compatibility verified; owner completes real Discord browser sign-in.
- [ ] Scored-game lifecycle and abandoned/all-failed policies approved and implemented; this preview is unscored.

## Reliability
- [x] Offline original-message draft tests verify no rewards for unconfirmed names, pending submission credit, all ranks, and replay idempotency.
- [ ] Contextual parser changes deployed and observed on a new live draft.
- [ ] Database backup and restore procedure documented
- [x] Dedicated migration job or pre-traffic hook wired into the deployment environment — `cmd/migrate/main.go` + `deploy/base/castaway-web/migration-job.yaml` as Argo CD PreSync hook
- [x] Production web pods configured with `AUTO_MIGRATE=false` — enforced in k8s environment overlay
- [ ] Migration rollout and rollback procedure documented
- [x] Health checks wired into the deployment environment — `/healthz` endpoint registered outside auth middleware
- [ ] Seed/dev-only workflows clearly separated from production operations

## Observability
- [ ] Structured logging verified in the deployed environment
- [ ] Alerting defined for API downtime and startup failures
- [ ] Runbook documented for database outages and application restarts

## Contract discipline
- [x] OpenAPI generation check enforced in CI — TypeSpec in `typespec/main.tsp`, generated `openapi.yaml`
- [x] Route parity check enforced in CI — `internal/httpapi/openapi_routes_test.go`
- [ ] Public API docs updated for externally visible changes

## Follow-up threads

- `plans/auth-and-authorization-planning.md`
- `plans/service-to-service-authentication-planning.md`

## Operational status

Current state: service-to-service auth is implemented and tested; deployment manifests and migration job are wired. Remaining gaps: authorization model, runbooks, structured logging, and operational hardening.
