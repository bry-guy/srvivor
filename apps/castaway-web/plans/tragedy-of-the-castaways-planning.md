# Tragedy of the Castaways (weekly game)

Status: planning

Async, semi-cooperative commons game: tribes gain most by all sharing, but any one tribe gains by stealing.

## Rules
- Public pot starts at **9** tokens, shown on the site (and optionally a daily channel post).
- 7 daily votes. Each tribe privately votes **Share** or **Steal** (majority of members who voted; tie or no votes = Share). Players can change their vote until the deadline.
- Steal: the tribe takes 1 token from the pot into its stash (+1 per member). If all tribes share, the pot grows by 1.
- End of week: pot split three ways (rounded down); each player gets their tribe's share plus their tribe's stash.
- Pot hits 0: game ends early ("the island is barren"); stashes are kept.
- Proposed: final-day audit — a tribe that steals on day 7 forfeits its pot share (otherwise day 7 is always steal).

| Outcome | Points per player |
|---|---|
| All share all week | 16 / 3 = +5 |
| One tribe steals daily | thief +7, others about 0 |
| All steal | pot empty by day 3: +3 |
| Nobody plays | 9 / 3 = +3 |

## Visibility
- Public: the pot only, never which tribe stole (the size of the drop shows how many stole).
- Teammates see their tribe's tally after each deadline (e.g. "3 share, 1 steal"), not who voted what.
- The scores post reveals every tribe's daily choices.

## Voting and DMs
- Site page: pot, pot history, day and countdown, Share/Steal buttons with current vote.
- Players can DM Jeff "share" or "steal"; Jeff confirms with tribe, day and deadline.
- 8pm ET: Jeff DMs anyone who hasn't voted that day (current pot and deadline); "stop nudging" opts out.
- Deadline 11:59pm ET; new pot visible each morning.

## Build (about Inside Track size)
- Tables: game (pot, days, schedule) and daily votes; a daily resolver; site page; bot DM vote parsing and nudge schedule; game-results type; Probst template and season config.

## Open questions
- Deadline 11:59pm ET or earlier?
- Daily pot post in the channel, or site only?
- Final-day audit, or plain rules?
