# Castawordle preview deployment

Status: in-progress

## Approved scope

Temporarily host feature branch `feat/castawordle-preview` at `https://castaway.bry-guy.net`. Retain the existing Discord OAuth callback. Every content page requires a Discord session; health/OAuth/static assets remain reachable. Trial play persists in new tables but never creates scoring occurrences, participant inputs, or bonus awards.

The requested two-level preview hostname is not covered by free Cloudflare Universal SSL, so use the owner's production-hostname fallback. No new paid service or infrastructure is required.

## Pre-deployment baseline

Observed 2026-09-29 Pacific:

- Argo application: `argocd/castaway-home-k3s`
- GitOps source: `main`, path `deploy/environments/home-k3s`
- Synced revision: `80cc16c72b2f6aa45461e2ffa23ac32b72b4bdac`
- Web image: `ghcr.io/bry-guy/castaway-web@sha256:2e2f2b375ffed70f5a5881794695a8976719ec8520436cfa8769e81583accd4a`
- Bot image/configuration and cloudflared resources must remain unchanged.

## Procedure

1. Pass documented CI, disposable PostgreSQL integration, and optional browser checks. Commit the feature code without staging `AGENTS.md`.
2. Publish only the web image through workflow dispatch on the feature branch. `update-home-k3s` is guarded to run only on `main`, preventing branch publishing from pushing deployment changes into main.
3. Pin the published web digest in the feature branch's home-k3s overlay. Compare its complete rendered resources with the baseline; only the web/migration image should differ.
4. Commit/push the digest update. Explicitly set the live Argo application's source to that tested deployment commit; do not merge into main. Argo runs additive migration 020 before serving the feature image.
5. Verify Synced/Healthy status, actual image digest, migration, authenticated-page redirects, existing Probst API compatibility, and availability of unscored trial games.

## Rollback

Set `spec.source.targetRevision` on `argocd/castaway-home-k3s` back to `main`, then request/observe Argo synchronization. For an exact baseline rollback, use revision `80cc16c72b2f6aa45461e2ffa23ac32b72b4bdac` instead. The old application ignores the additive game/play tables, so do not drop them or delete saved trials during rollback. Keep managed credentials and DNS/tunnel configuration unchanged.

The one-off announcement request was made on September 29 Pacific: after preview deployment, reschedule the existing September 30, 2026 scores post to 17:00 America/New_York, preserving its body and identity. Regular score posts should default to noon Eastern; Castawordle opening/cutoff times are unchanged.

## Verification record

Local full monorepo CI, full PostgreSQL integration, and 320px/desktop browser checks passed. TypeSpec's existing dependency audit reports seven vulnerabilities (2 moderate, 4 high, 1 critical); no dependency changes or audit fixes were made. LSP probes were unavailable; lint/compiler/runtime checks passed. The feature overlay preserves the current live bot digest `sha256:eaab924b52a07e49a7150d44eadd28950c49640e4cec37bdf9954e64dfef530f`, matching main. Image publication and deployment remain pending.
