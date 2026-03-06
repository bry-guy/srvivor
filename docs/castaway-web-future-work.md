# castaway-web future work

## Deferred: bonus points system

This is intentionally postponed while core persistence + draft gameplay stabilize.

Planned model (high level):
- `bonus_rules`
- `bonus_assignments`
- `season_events`
- `bonus_ledger`

Goals:
- Support tribe-based and contestant-based bonus targets
- Keep an auditable points ledger
- Allow automatic and manual point adjustments

Not included in current implementation:
- Bonus rule CRUD
- Event ingestion for immunity/journey outcomes
- Bonus scoring integration into leaderboard
