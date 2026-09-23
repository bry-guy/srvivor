# Dependable weekly season execution

Status: `in-progress`

## Goal and boundaries

Deliver a predictable weekly game using existing scoring and point storage. Preserve the uncommitted YAML/Hurl scenario work as test infrastructure, not a live-season controller. No production operations are authorized by this plan.

The launch timetable is unset. This season uses public bonus points only: no secret earning, balances, spending, or conversion in the season's gameplay. Stir the Pot, auctions, and loans are retired for this season. Preserve historical instances and their ledger records; do not delete or convert historical secret points. Discord command reduction and AI-assisted thread ingestion are deferred until a separate detailed plan; no Discord history access or registration changes are included here.

## Checkpoint 1: manually operated Wordle round

Use legacy instances provisionally because existing activity operations are supported there. Managed instances currently reject activity writes; do not remove those guards or change existing instances' modes.

- Add narrow service-authenticated, instance-admin API operations to create a round with explicit opening and cutoff times, enter/replace a participant result before cutoff, close entry, and resolve.
- Reuse the existing `tribe_wordle` resolver and ledger. Its current behavior averages up to three submitted guesses per tribe, allows tied winning tribes, and awards one public point to each active member of winning tribes. These inherited rules require acceptance before launch, particularly fewer-than-three submissions.
- Validate participant/group instance ownership and applicable tribe membership.
- Lock the round before checking state, accepting inputs, closing, or resolving. Resolve awards and state atomically. Repeated resolution returns the persisted result without additional awards.
- Prevent generic activity/occurrence routes from bypassing lifecycle ownership of these rounds, without changing unrelated legacy activities.
- Use strict cutoff as the baseline; allow edits before cutoff. Post-cutoff entry and post-resolution correction are deferred unless existing support makes a bounded exception practical and it is explicitly selected.
- Keep closure and resolution separate: closed entry is not proof that admin-entered results are complete.

Verification uses disposable PostgreSQL only: authorization, cross-instance rejection, injected-clock boundaries, submission/closure races, duplicate resolution, rollback, generic-route bypass, exact public awards, and unchanged secret balances. Existing regression tests are preserved.

Implementation checkpoint: the manual API and additive migration 013 are present locally, uncommitted. The new Wordle lifecycle regression and existing app/gameplay/HTTP suites pass through the disposable integration task. Database-free root CI also passes, including generated API consistency and Hurl checks. Only the two generated OpenAPI files are staged for the index-based consistency check. The held-lock test uses a timed wait rather than verified database lock-wait observation; do not treat it as deterministic contention evidence. Scheduling, public-only season enforcement, deployment, and rule acceptance remain pending.

## Checkpoint 2: schedule the same operations

The inventory found no existing runtime scheduler. Add only the durable scheduling needed for round opening announcements, cutoff closure, and result announcements after explicit resolution. Persist round/job identity and delivery state; resume safely after restart. Manual admin triggers use the same operations. Retrying a Discord announcement must not repeat scoring; do not promise exactly-once external delivery.

Keep scheduling inactive until the launch date, named timezone, weekly opening/cutoff/results times, and delayed-job policy are confirmed. Do not add a general workflow framework.

## Checkpoint 3: public-only season enforcement and rehearsal

Prepare a bounded implementation proposal before changing enforcement. Disable Stir the Pot, auctions, loans, and secret-point earning/spending for the upcoming season instance. Check specialized writes and generic activity/occurrence resolution so hidden commands cannot bypass the policy. Remove secret-balance presentation from this season's player-facing responses and score views; retain public bonus awards and actor authorization. Reuse existing instance configuration if suitable rather than building a general feature-flag framework.

Historical instances, scoring, secret ledger entries, and regression coverage remain intact. Do not drop secret-related schema, convert balances, or globally alter resolver behavior. If the policy will apply to an existing instance, obtain an explicit decision about its existing secret balances before mutation. Production changes and Discord registration changes need separate authorization.

Rehearse an accelerated week using the existing disposable test infrastructure, including admin entry, restart, duplicate/manual triggers, delayed jobs, failed announcement delivery, public bonus totals, rejected retired mechanics, and historical-instance isolation. Produce a short manual recovery runbook.

## Completed accelerated rehearsal

Season 43 now has a pinned survivoR fixture, six invented drafts, and thirteen synthetic weekly Wordle rounds driven through Hurl against a disposable legacy API. Local checks cover each week's exact scores, historical outcome visibility, duplicate resolution, public-only ledger entries, and absence of retired mechanics in the fixture. An explicitly authorized BrainLand run posted and verified 27 messages, with median/max delivery lag 128/615 ms. See [instructions and evidence](../docs/guides/season43-rehearsal.md).

This completes the iterative rehearsal, not Checkpoint 2's durable scheduler or Checkpoint 3's season-wide enforcement. No production deployment or historical database was touched.

## Release and deferred work

Deployment is a separate authorization gate requiring verified access, backup, migration and rollback readiness. Local test success is not production readiness.

After the weekly loop is proven, separately plan the player-facing Discord shortlist (primarily score and draft) and potential admin/AI ingestion from threads through the API or a CLI. No new CLI or player participation commands are required for the current checkpoints.
