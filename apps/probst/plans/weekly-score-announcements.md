# Weekly scores announcement scheduling

Status: planning

## Requested timing

- Instance: Podracing Season 51 (`ee0a0884-37ad-47bb-820b-cf111bec3a39`).
- First scores post: September 30, 2026, 17:00 America/New_York (`2026-09-30T21:00:00Z`).
- Normal scores posts: Wednesdays at 12:00 America/New_York, retaining Eastern daylight-saving behavior.
- Do not change Castawordle's separately approved opening/cutoff window.
- Reschedule an existing post/job without changing its identity or copy; do not duplicate it.

## Current blocker

After the Castawordle feature preview was deployed, read-only inspection found no existing scores announcement or weekly scores scheduler:

- Podracing's announcement API lists nine sent entries: kickoff/lookback/reminder and draft acknowledgments; no pending or draft scores entry.
- No Castaway Kubernetes CronJob or scores GitHub workflow exists.
- No matching local user cron/LaunchAgent or project scheduled-agent store exists.
- `probst recap` renders copy and saves a local score snapshot; it does not establish a recurring schedule.
- Earlier source records confirm the requested timing and a local Wordle announcement draft, not creation of a scores post/job.

Consequently no live schedule was changed and no replacement announcement was created. The previous assumption that a scores post was already scheduled is unsupported. To finish, identify any externally managed job not covered by these checks, or explicitly establish the missing scheduling workflow and approved announcement copy. Keep existing nonempty external announcement text untouched.

## Available operator commands

Once an existing scores announcement's identity is known:

```sh
probst announcement schedule EXISTING_NAME --at "2026-09-30 17:00" --instance podracing
```

For October 7 and later, use noon Eastern. `announcement schedule` modifies the saved row rather than creating a new one. Read back its identity, body, and scheduled timestamp; verify all other announcements and the Wordle window remain unchanged. A recurring job would still be a separate capability requiring an explicit implementation decision.
