# Player administration and minimized Discord surface

Status: in-progress

## Scope

Probst is an HTTP client using trusted-service delegation. The API owns channel bindings, transaction-bound admin checks, restricted first-admin bootstrap, and conflict-safe player links. Discord exposes only score, scores, and draft with native user selection, ephemeral responses, and public-only rendering.

## Evidence

- Probst unit checks and the real binary against disposable PostgreSQL passed.
- Full disposable API suite passed with historical Wordle/scenario behavior intact.
- Root CI passed with database URLs unset; disposable integration passed separately.
- The bounded advisor review approved proceeding to BrainLand verification only; this is not delivery or deployment approval.
- Minimized Discord dispatch and guild-only registration tests pass. BrainLand command registration/read-back and genuine player-command checks succeeded when the competing Kubernetes bot was temporarily paused.

## Remaining gates

- BrainLand initially contained only the existing `/castaway` command; no unrelated application commands were present.
- After user-confirmed runtime isolation, the bot registered only in BrainLand guild `1078197143501819915`; API read-back verified exactly `score`, `scores`, and `draft`, with optional native USER selectors for `score` and `draft`.
- BrainLand `#general` is bound to a disposable local Season 901 instance with two synthetic participants, two outcomes, and one two-pick draft. Its API uses a fresh local PostgreSQL container and loopback-only service.
- The user confirmed the singular `/castaway` root. Before isolation, the old Kubernetes bot emitted the ambiguous-instance error; the local bot logged `40060`/`10062` before handling those interactions. With Kubernetes paused, the user retried and the local `/castaway scores` handler logged success.
- The competing pod was `castaway-discord-bot` in namespace `castaway`, image `ghcr.io/bry-guy/castaway-discord-bot@sha256:86fdcd1c03f960f14560902db9ef3ba7bcd4fe74cb2b2e66f0358bdd9c7b5b8e`, running commit `1ec3223` from 2026-09-16. Its handler lacks a guild dispatch guard and resolves instance defaults, so it answers BrainLand interactions meant for the channel-bound bot.
- For the approved retest, auto-reconciliation for `castaway-home-k3s` was paused, the bot replica scaled 1→0, and the local scores command succeeded. The local bot was stopped before restoration; the K8s replica and exact Argo policy (`prune=true`, `selfHeal=true`) were restored and verified 1/1, Synced, Healthy.
- Post-restore Discord read-back still shows only BrainLand `score`, `scores`, `draft`; global command count is zero. No permanent source or deployment fix was made. The legacy K8s bot can still compete after restoration.
- The user chose to replace the old command surface everywhere. The known guild registrations are BrainLand `1078197143501819915` with `score`, `scores`, and `draft`, and Podracing `521073437779689474` with the 18 legacy subcommands; global command read-back is empty. The deployed bot is configured for Podracing.
- The new bot registers and filters for one configured guild. Serving both known guilds requires an explicit guild allowlist, guild-scoped registration in each, and dispatch tests; global registration remains disabled.
- The deployed web image is `ghcr.io/bry-guy/castaway-web@sha256:735baba3e32c00a07417a7026496d2c15916d361fc0ff7c1a7e021f198e9c740`; its source commit and production migration level remain unverified.
- The current `feat/integration-ci` worktree is not a release candidate: it is seven commits past deployed bot commit `1ec3223`, with 27 committed files changed and 64 staged, unstaged, or untracked paths. It contains unrelated progression, Wordle, scenario, and rehearsal work.

## Candidate rollout

- Cut a reviewed release from `main`; do not push the current worktree. Main pushes automatically publish changed web/bot images, update GitOps digests, and cause Argo's PreSync migration job to run.
- The current migration runner applies every sorted unapplied SQL file. This worktree contains `012_managed_progression.sql`, `013_wordle_rounds.sql`, and `014_discord_channel_bindings.sql`; the channel-admin handlers currently use the progression lock query, so this is not safely reducible to migration 014 alone. Production's applied migration level is unknown. A minimal release needs channel-binding/admin API code decoupled from progression/Wordle and a migration sequence coordinated with the unmerged progression branch.
- Before any release: verify backup freshness and isolated restore, run a sanitized read-only migration preflight, confirm the production instance for each command channel, and preserve existing participant Discord links. Do not seed, import, or run the migration executable to discover state.
- Roll out API/schema first, configure the authenticated admin bootstrap if needed, bind confirmed channels and link players through Probst, then replace the old bot with one runtime using an explicit guild allowlist for the intended guilds. Register/read back only the three player commands per guild and run genuine user checks.
- Rollback must stop the new bot before restoring an old runtime, and remove any guild registration the old runtime cannot safely serve; never leave two responders active.

## Follow-up

Season-operation commands are a separate requested fast-follow. Scheduling, AI ingestion, OAuth/RBAC, generic gameplay CRUD, historical data rewrites, and secret-balance presentation remain deferred.
