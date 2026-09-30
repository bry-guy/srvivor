# Scheduled Castawordle deployment

Status: done

## Authorized rollout

Deploy verified source `56f52e537165dc6c6ed36376b44333827c93b64b` using pinned GitOps revision `bc1ad1bf815b4d975b08f7e4d120210e0e78e8ff`. GitHub CI and image publication succeeded. Web/migration image: `ghcr.io/bry-guy/castaway-web@sha256:c88e738e56e9d0d87e955fcd5756022c2adf6010dcef2d2b59d0fcb28679bd4e`.

Only the web image differs from the running preview's deployment manifests. Preserve the bot digest `sha256:eaab924b52a07e49a7150d44eadd28950c49640e4cec37bdf9954e64dfef530f`, tunnel, credentials, answers, and saved progress. Games remain unscored. Do not create production puzzles or submit guesses for smoke tests.

## Rollback baseline

Before rollout, Argo was Synced/Healthy/Succeeded at `eaa2a851b1e82f18e6fd113ccda8dc314ca8da5e`; web digest `sha256:46e8c06ffd23e6b3a849e232644f6e3864ecb7cfaa7e8443be5a10f7fb78ed1f`.

To roll back, restore `argocd/castaway-home-k3s` source targetRevision to that exact preview commit and synchronize. Keep additive migration 021 and saved games/plays; do not drop columns/tables or rewrite scoring. The verified new revision is pinned rather than changing ongoing tracking to main. Restoring the previous image also restores its less restrictive trial visibility; use rollback only with that privacy regression understood.

## Verification

Argo reports Synced/Healthy/Succeeded at the pinned revision, with a successful PreSync migration hook. The web deployment uses the published digest; web, existing bot, and both tunnel pods are ready with zero restarts. Bot digest and tunnel are unchanged.

Live checks: `/healthz` 200; `/` and `/castawordle` redirect to Discord login; `/assets/game-admin.js?v=2` 200 with episode selection support; unauthenticated gameplay API 401. Read-only Probst scores succeeded using Season 51 UUID `ee0a0884-37ad-47bb-820b-cf111bec3a39`.

Admin-only visibility, scheduled/finale windows, progress, mobile/desktop gameplay, and zero-scoring effects were verified in disposable PostgreSQL/browser tests before rollout, not by submitting production guesses. No production puzzles, answers, saved plays, or scores were edited during deployment.
