# Production readiness checklist

Use this checklist before exposing Castaway services to production users.

## API security
- [x] `castaway-web` bot/API authentication mechanism selected — bearer-token service auth implemented in `httpapi/auth.go`
- [ ] Auth credentials stored in managed secrets, not local files
- [ ] Authorization model documented for future write workflows — see `docs/security-audit-web-discord-identity.md` for gaps
- [ ] Public network exposure reviewed

## Discord bot security
- [ ] Discord bot token rotation procedure documented
- [x] Guild-level commands that change shared state require appropriate Discord permissions — `hasGuildManagePermission` enforced in handlers
- [ ] Logs verified to avoid leaking tokens, auth headers, or sensitive payloads

## Reliability and operations
- [x] Health checks documented for `castaway-web` and `castaway-discord-bot` — `/healthz` endpoint implemented and tested
- [x] Dedicated migration job or pre-traffic hook documented for `castaway-web` production deploys — `cmd/migrate/main.go` + k8s Job manifest
- [ ] External PostgreSQL ownership and backup/restore responsibility documented for the selfhost target
- [ ] Alerts defined for API downtime and bot startup failures
- [ ] External PostgreSQL backup/restore ownership and drill cadence documented
- [ ] Persistent bot state backup/restore plan documented
- [ ] Rollback procedure documented

## API contract discipline
- [x] OpenAPI generation check enforced in CI — TypeSpec definitions in `typespec/main.tsp` with generated OpenAPI
- [x] Route parity test enforced in CI — `openapi_routes_test.go` validates router/spec alignment
- [ ] Public API docs updated for new routes and query parameters

## Local and staging validation
- [x] Local development flow documented and verified — mise tasks, fnox secrets, local dev documented
- [ ] Discord dev guild command sync verified
- [ ] Seeded local environment tested end-to-end against `castaway-web`

## Future write workflows
- [ ] Server-side authorization model designed
- [ ] Audit expectations defined for state-changing commands
- [ ] Admin-only command UX reviewed
