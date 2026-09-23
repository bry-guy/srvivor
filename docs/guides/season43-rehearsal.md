# Season 43 iterative rehearsal

## Run locally

From the repository root:

```bash
env -u DATABASE_URL -u CASTAWAY_TEST_DATABASE_URL -u CASTAWAY_REHEARSAL_LIVE \
  MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:rehearsal-season43
```

This creates and removes its own PostgreSQL container and test database, runs a local API with a test-controlled clock, and drives gameplay through Hurl. No Discord access is needed. Runtime is roughly two seconds for the test plus container startup/build time. The full disposable integration suite also includes this test.

Edit `apps/castaway-web/internal/httpapi/testdata/season43/fixture.json` to iterate on invented drafts and Wordle inputs; update the checked expected scores deliberately. Historical source fields are pinned separately from invented gameplay. See the adjacent fixture README and license for attribution and field mapping.

## Explicit live Discord run

Only after local assertions pass and BrainLand message posting is authorized:

```bash
FNOX_PROFILE=castaway-discord-bot fnox exec -- \
  env -u DATABASE_URL -u CASTAWAY_TEST_DATABASE_URL \
  CASTAWAY_REHEARSAL_LIVE=1 \
  CASTAWAY_REHEARSAL_REPORT=/tmp/castaway-season43-new-run.jsonl \
  MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:rehearsal-season43
```

Choose a new report filename each run; existing files are not overwritten. The test validates the fixed BrainLand guild and uses its `#general` channel (`1078197143501819918`). The bot lacks channel-creation permission. Destination changes require a deliberate code change; it cannot silently use the production announcement channel.

The admin harness enters six results each week, applies that episode's historical outcomes, resolves Wordle, verifies leaderboard totals, and sends the API's actual score rows. It posts an opening every six seconds, scores three seconds later, and one final summary: at most 27 messages. All mentions are disabled. The logical game clock covers September–December 2022 while the delivery clock covers about 80 seconds.

Posts are not automatically retried. A failed/uncertain send or a pre-send deadline missed by over two seconds stops the run. Check the JSONL report and channel before restarting; restarting creates a fresh season and another set of messages. Cleanup removes only the owned local server/database/container; Discord messages and the delivery report remain.

## Observed live run: 2026-09-22

- All 13 episodes and 18 placements replayed successfully.
- Every episode asserted exact draft, bonus and total scores for all six players; future outcomes stayed absent until their episode.
- All 13 resolution retries preserved exact award counts; 42 public +1 ledger entries, no secret or retired-mechanic activity in this fixture.
- 27/27 Discord messages verified by read-back; zero user, role or everyone mentions.
- Planned-to-Discord timestamp lag: median 128 ms, maximum 615 ms.
- [First opening](https://discord.com/channels/1078197143501819915/1078197143501819918/1551979570297569476) · [Completion](https://discord.com/channels/1078197143501819915/1078197143501819918/1551979897444896909)
- Exact delivery IDs and timestamps: [`season43-2026-09-22.jsonl`](../rehearsals/season43-2026-09-22.jsonl).

| Player | Draft | Wordle | Total |
| --- | ---: | ---: | ---: |
| Ada | 88 | 7 | 95 |
| Faye | 87 | 7 | 94 |
| Drew | 86 | 7 | 93 |
| Ben | 83 | 7 | 90 |
| Eli | 78 | 7 | 85 |
| Cora | 72 | 7 | 79 |

## What this does not establish

This is an accelerated legacy-instance rehearsal, not persistent season hosting or a durable weekly scheduler. It does not exercise command registration, player self-submission, production deployment, or restart recovery for announcements. The runner's fake admin identity is local only; Discord delivery uses the authorized bot, not impersonation of a person.

Public-only and retired-mechanic enforcement for a real upcoming-season instance remains unimplemented. The fixture simply uses no other mechanics. Managed progression still blocks Wordle; its separate YAML/Hurl example is unchanged. Before launch, choose the actual timetable/timezone, accept the Wordle tie/incomplete-submission rules, provide admin/tribe/schedule setup, implement season-specific retirement guards and durable delivery, and verify operational rollback/access.
