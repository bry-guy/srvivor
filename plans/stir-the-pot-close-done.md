# Stir the Pot Close Flow

Status: `done`
Owner: castaway-web + castaway-discord-bot
Last updated: 2026-04-01

## Goal

Add an admin-only close flow for Stir the Pot so admins can stop contributions and JeffProbst can announce each tribe's final result.

## Implemented

- Added admin close endpoint:
  - `POST /instances/:instanceID/stir-the-pot/close`
- Added admin bot command:
  - `/castaway pot close [instance]`
- Closing a pot now:
  - prevents further contributions
  - preserves the round for later tribal-pony bonus resolution
  - returns tribe-by-tribe totals and earned pony bonus values
- JeffProbst now posts one public result message per tribe after close.
- Updated the hidden final threshold in code from `11` to `10`.
- Player/admin open-round formatting now hides the final threshold as `?→+4`.

## UX

- While open, users see:
  - `2→+1, 5→+2, 8→+3, ?→+4`
- On close, admins get an ephemeral confirmation and Jeff posts tribe results publicly.
