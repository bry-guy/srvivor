# castaway-discord-bot Functional Requirements

`castaway-discord-bot` provides Discord-native access to Castaway draft data by querying `castaway-web`.

## Active player surface

- Register and dispatch only `/castaway score [player]`, `/castaway scores`, and `/castaway draft [player]`.
- Select players by native Discord user ID, defaulting to the invoker.
- Resolve the instance exclusively through API channel bindings; threads inherit their parent when unbound.
- Reject DMs and retired payloads; always reply ephemerally with public-only scoring and mention notifications disabled.
- Point setup errors to the configured, verified admin contact.
- Restrict registration and interaction handling to the explicitly configured guild allowlist; never mutate global registrations as a side effect.
- Admin setup belongs in [Probst](../probst/README.md); season operations are deferred.

## Deprecated legacy capabilities

The requirements below describe retained legacy handlers/state, not the registered or dispatched player surface.

## Inputs

- Discord slash command interactions
- `castaway-web` HTTP responses
- local persistent state for saved default instances
- environment-based runtime configuration and secrets

## Required capabilities

- register and serve the `/castaway` command surface
- return an individual participant score using the public weekly-score layout (rank, tribe badge, real Discord mention when linked, and public draft/bonus subtotal)
- return a leaderboard for the selected instance using that same public weekly-score layout
- return a participant draft for the selected instance
- list available instances
- save, show, and clear user-level default instances
- save, show, and clear guild-level default instances
- enforce Discord permission checks for guild-scoped default changes
- resolve the active instance from explicit input or saved defaults
- expose read workflows for activities, occurrences, and participant history
- support linked-player merge gameplay actions for:
  - Stir the Pot status and contributions
  - blind individual pony bids and current bid review
  - current pony ownership review
  - Loan Shark borrow / repay / status
  - public secret-bonus reveal announcements when hidden spends consume secret points
- require callers to be linked before they can submit self-service Stir the Pot contributions or auction bids
- support admin-on-behalf merge gameplay actions for named participants on Stir the Pot contributions and blind individual pony bids
- support admin Discord workflows for:
  - opening Stir the Pot for the next scheduled episode
  - showing the current Stir the Pot total for a named tribe
  - opening and closing individual pony auction lots, with lot starts bound to the next scheduled episode
  - recording individual pony immunity winners
- format responses safely within Discord message limits
- fail clearly when the API or local configuration is unavailable

## Outputs

- Discord messages for score, leaderboard, draft, instance, activity, and merge gameplay workflows
- persisted guild/user default state
- structured logs for runtime failures and startup state

## Current non-goals

- direct draft editing from Discord
- independent scoring logic outside `castaway-web`
- multi-replica production state coordination without a shared backend
