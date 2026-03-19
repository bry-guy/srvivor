# Production readiness checklist

Use this checklist before exposing Castaway services to production users.

## API security
- [ ] `castaway-web` bot/API authentication mechanism selected
- [ ] Auth credentials stored in managed secrets, not local files
- [ ] Authorization model documented for future write workflows
- [ ] Public network exposure reviewed

## Discord bot security
- [ ] Discord bot token rotation procedure documented
- [ ] Guild-level commands that change shared state require appropriate Discord permissions
- [ ] Logs verified to avoid leaking tokens, auth headers, or sensitive payloads

## Reliability and operations
- [ ] Health checks documented for `castaway-web` and `castaway-discord-bot`
- [ ] Dedicated migration job or pre-traffic hook documented for `castaway-web` production deploys
- [ ] Alerts defined for API downtime and bot startup failures
- [ ] Persistent bot state backup/restore plan documented
- [ ] Rollback procedure documented

## API contract discipline
- [ ] OpenAPI generation check enforced in CI
- [ ] Route parity test enforced in CI
- [ ] Public API docs updated for new routes and query parameters

## Local and staging validation
- [ ] Local development flow documented and verified
- [ ] Discord dev guild command sync verified
- [ ] Seeded local environment tested end-to-end against `castaway-web`

## Future write workflows
- [ ] Server-side authorization model designed
- [ ] Audit expectations defined for state-changing commands
- [ ] Admin-only command UX reviewed
