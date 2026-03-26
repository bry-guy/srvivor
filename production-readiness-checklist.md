# Castaway Production Readiness Checklist

This checklist records repository-level production requirements across apps.
Detailed shared guidance also exists in `docs/production-readiness-checklist.md`.

## Shared readiness
- [x] Each app has current functional, non-functional, and production-readiness documentation — all apps have standardized docs
- [x] Shared secrets and configuration flows are documented — `docs/secrets-and-config.md` covers 1Password/fnox workflow
- [ ] Cross-app runbooks are documented for outages, restarts, and rollback
- [x] Release/versioning expectations are documented — `docs/versioning-and-releases.md`

## App readiness
- [ ] `apps/castaway-web` production requirements reviewed
- [ ] `apps/castaway-discord-bot` production requirements reviewed
- [ ] `apps/cli` stable/archive expectations reviewed

## Follow-up threads

- `docs/selfhost-k3s-deployment-blueprint.md`
- `docs/security-audit-web-discord-identity.md` — new: cross-app identity and authorization gap analysis
- `plans/selfhost-k3s-implementation-plan.md`
- `apps/castaway-web/plans/auth-and-authorization-planning.md`
- `apps/castaway-web/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/state-backend-and-operations-planning.md`
- `apps/castaway-discord-bot/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/postgres-state-backend-planning.md`
- `apps/cli/plans/archive-policy-planning.md`

## Current status

Current state: foundational implementation is in place — service auth, deployment manifests, migration job, TypeSpec contract, and PostgreSQL state backend are all implemented. Remaining work is authorization model design, operational runbooks, structured logging, and deployment hardening.
