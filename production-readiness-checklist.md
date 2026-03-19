# Castaway Production Readiness Checklist

This checklist records repository-level production requirements across apps.
Detailed shared guidance also exists in `docs/production-readiness-checklist.md`.

## Shared readiness
- [ ] Each app has current functional, non-functional, and production-readiness documentation
- [ ] Shared secrets and configuration flows are documented
- [ ] Cross-app runbooks are documented for outages, restarts, and rollback
- [ ] Release/versioning expectations are documented

## App readiness
- [ ] `apps/castaway-web` production requirements reviewed
- [ ] `apps/castaway-discord-bot` production requirements reviewed
- [ ] `apps/cli` stable/archive expectations reviewed

## Follow-up threads

- `docs/selfhost-k3s-deployment-blueprint.md`
- `plans/selfhost-k3s-implementation-plan.md`
- `apps/castaway-web/plans/auth-and-authorization-planning.md`
- `apps/castaway-web/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/state-backend-and-operations-planning.md`
- `apps/castaway-discord-bot/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/postgres-state-backend-planning.md`
- `apps/cli/plans/archive-policy-planning.md`

## Current status

Current state: local development readiness is in place; production readiness remains an app-by-app hardening effort.
