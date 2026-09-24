# Season 51 announcements via Probst

Status: planning

## Current state and boundaries

- The last recorded BrainLand check found Season 51 instance `605699c6-1f21-4741-8e0f-4864d5e62018`, a `#general` channel binding, and one imported draft. This is test data on production infrastructure, not an isolated dev database. Live state has not been rechecked for this plan.
- The operator reports no separate Podracing Season 51 instance exists yet and selects Podracing `#survivor` for production announcements. Verify both facts read-only before creating or binding anything; do not use completed Season 50 as the new season's target.
- `DefaultEpisodeScheduleForSeason` in castaway-web supplies a real schedule only for Season 50; Season 51 receives a January 2000 placeholder. No authoritative Season 51 episode times can be inferred from the API yet.
- Probst has no announcement send command. The deployed bot can send as its bot account, but neither an announcement queue nor a live scheduler exists. The YAML/Hurl season runner is not a scheduler.
- This document authorizes no message, schedule, production write, credential retrieval, or deployment. The recap/kickoff examples must not be silently sent or rescheduled from their historical wording.

## Checkpoints

1. **Confirm destinations and dates.** Read the current instance and channel bindings through already-configured read-only access. Confirm whether Podracing Season 51 exists and whether `#survivor` is the intended channel. Obtain the next few actual times in `America/New_York` (if approved) and decide where the authoritative episode schedule will be maintained; never treat the placeholder as an episode date. Do not seek credentials without permission.
2. **Manual-send vertical slice.** Probst previews an operator-authored UTF-8 Markdown file verbatim with instance, bound guild/channel, and send-now time. Explicit `--yes` submits a stable request key to a service-authenticated, instance-admin-authorized API; the API persists an immutable announcement before delivery. The existing bot sends it as its own configured identity and records the returned Discord message ID. Validate Discord length, reject a mismatched binding, preserve formatting, disable notification pings by default, and require explicit named-user opt-in for pings. No human-user token or direct unrecorded Probst send.
3. **Durable scheduling.** The same record accepts an explicit timezone-aware future timestamp; the existing bot runtime claims due work atomically and survives restarts. Persist destination, content, due time in UTC, payload identity, status, and Discord message ID. Provide minimal Probst list/status/cancel commands; cancel only before delivery. Reject changed-payload key reuse and a binding that changed between enqueue and delivery. Respect Discord rate limits; an uncertain network result needs reconciliation rather than a blind retry or an exactly-once claim. Test contention, restart, retry, cancellation, rebind, and ambiguous delivery with fake Discord and disposable PostgreSQL.
4. **BrainLand verification, then Podracing activation.** Test the manual and scheduled paths on disposable infrastructure first. Send a clearly labeled BrainLand test message only after the exact content, destination, and timing are approved; verify the account is @JeffProbst and read back the message ID. Confirm Podracing's separate Season 51 mapping before explicitly approving any production announcements. No automatic migration of draft/test messages to Podracing.
5. **Season-relative times after the schedule is real.** Once an authoritative Season 51 schedule is written and checked, allow an episode time plus offset to resolve to a concrete UTC due time when enqueued. Later edits to episode dates must not silently move queued announcements: cancel and recreate after preview and approval.

## Decisions and remaining inputs

- Use a separate Podracing Season 51 instance (not yet created, per operator) and Podracing `#survivor` as the intended announcement channel. Confirm its exact channel ID and binding before any write.
- Use `America/New_York` for episode and announcement times; store resolved due times in UTC.
- Still needed: the next few confirmed episode dates and desired announcement send times. Do not reuse relative wording such as “tonight” from the old examples.
- Decide whether the example user mentions should notify selected IDs; default to rendering them without notifications.
