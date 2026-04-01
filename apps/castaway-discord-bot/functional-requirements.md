# castaway-discord-bot Functional Requirements

`castaway-discord-bot` provides Discord-native access to Castaway draft data by querying `castaway-web`.

## Inputs

- Discord slash command interactions
- `castaway-web` HTTP responses
- local persistent state for saved default instances
- environment-based runtime configuration and secrets

## Required capabilities

- register and serve the `/castaway` command surface
- return an individual participant score
- return a leaderboard for the selected instance
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
- support admin Discord workflows for:
  - opening Stir the Pot
  - opening and closing individual pony auction lots
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
