# Castawordle preview deployment

Status: done

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

The one-off announcement request was made on September 29 Pacific: after preview deployment, reschedule the existing September 30, 2026 scores post to 17:00 America/New_York, preserving its body and identity. Regular score posts should default to noon Eastern; Castawordle opening/cutoff times are unchanged. Follow-up inspection found no existing scores post/job, so no live schedule was modified and no replacement was created; see [the unresolved scheduling plan](../../probst/plans/weekly-score-announcements.md).

## Verification record

Local full monorepo CI, full PostgreSQL integration, and 320px/desktop browser checks passed. TypeSpec's existing dependency audit reports seven vulnerabilities (2 moderate, 4 high, 1 critical); no dependency changes or audit fixes were made. LSP probes were unavailable; lint/compiler/runtime checks passed. The feature overlay preserves the current live bot digest `sha256:eaab924b52a07e49a7150d44eadd28950c49640e4cec37bdf9954e64dfef530f`, matching main. Published image from code revision `5edf4a84d59de7b5126df6f01fba88ec2b372476` via workflow run `36672089420`; bot build and main digest updater were both skipped. Web digest: `sha256:92ca66db5a5c4a91c4e20d3b804b52667d42843d9b91188a57af254e9bc85b45`. Argo is explicitly pinned to deployment revision `c9ca814e4c3a636dfd29ea5351625c13fbe662ea` on the feature branch (not merged). Complete rendered-manifest comparison differed only in the web/migration image.

Live Argo reports Synced/Healthy/Succeeded and a successful PreSync migration hook. The new web, existing bot, and two tunnel pods are ready with zero restarts; actual web image matches the published digest and bot image is unchanged. Public root/list/detail requests redirect to Discord login; OAuth callback is unchanged; health/assets return 200 and unauthenticated play API returns 401. Existing Probst scores/player queries work, and Adam's corrected draft remains present. Mobile/desktop gameplay and cross-device persistence were checked against disposable PostgreSQL with fake OAuth; the owner should complete real Discord browser login on the deployed site. Instance admins can create unscored trial puzzles from `/castawordle`.
