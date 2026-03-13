# castaway-discord-bot Production Readiness Checklist

## Security
- [ ] Discord token rotation procedure documented
- [ ] Guild-scoped commands verified to enforce required Discord permissions
- [ ] Secrets managed outside the repository and local plaintext files
- [ ] Logging reviewed for secret and payload leakage risks

## Reliability
- [ ] `castaway-web` outage behavior documented and tested
- [ ] State backup/restore expectations documented for the selected deployment model
- [ ] Restart/runbook documentation created
- [ ] Default-instance persistence validated under expected usage

## Observability
- [ ] Structured logging verified in the deployed environment
- [ ] Alerts defined for startup failures and repeated API dependency failures
- [ ] Operational dashboards or equivalent troubleshooting guidance documented

## Product readiness
- [ ] Discord dev guild registration and sync verified
- [ ] Bot invite and required scopes documented
- [ ] Slash command help/docs aligned with shipped behavior

## Follow-up thread

- `plans/state-backend-and-operations-planning.md`

## Operational status

Current state: suitable for local and single-instance development; production rollout still requires explicit operational runbooks and a production state strategy.
