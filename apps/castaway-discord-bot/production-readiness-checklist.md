# castaway-discord-bot Production Readiness Checklist

## Security
- [ ] Discord token rotation procedure documented
- [x] Guild-scoped commands verified to enforce required Discord permissions — `hasGuildManagePermission` check in `handlers.go`
- [ ] Secrets managed outside the repository and local plaintext files
- [ ] Logging reviewed for secret and payload leakage risks

## Reliability
- [ ] `castaway-web` outage behavior documented and tested
- [ ] State backup/restore expectations documented for the selected deployment model
- [x] BoltDB-to-PostgreSQL state migration path documented and validated — `postgres_store.go` implemented with `ensureSchema`, both backends available via `Store` interface and `Open()` factory
- [ ] Restart/runbook documentation created
- [x] Default-instance persistence validated under expected usage — guild and user defaults stored with proper unique constraints

## Observability
- [ ] Structured logging verified in the deployed environment
- [ ] Alerts defined for startup failures and repeated API dependency failures
- [ ] Operational dashboards or equivalent troubleshooting guidance documented

## Product readiness
- [ ] Discord dev guild registration and sync verified
- [ ] Bot invite and required scopes documented
- [ ] Slash command help/docs aligned with shipped behavior

## Follow-up threads

- `plans/state-backend-and-operations-planning.md`
- `plans/service-to-service-authentication-planning.md`
- `plans/postgres-state-backend-planning.md`

## Operational status

Current state: service auth header wired into API client; PostgreSQL state backend implemented alongside BoltDB; guild permission enforcement in place. Remaining gaps: token rotation runbooks, structured logging, outage behavior documentation, and state backup/restore procedures.
