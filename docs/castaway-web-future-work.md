# castaway-web future work

## Deferred: bonus points system

This is intentionally postponed while core persistence + draft gameplay stabilize.

Detailed planning now lives in:
- `docs/castaway-manual-gameplay-logs.md`
- `docs/castaway-bonus-points-plan.md`

The current recommendation is a manual-first, event-driven bonus ledger model instead of starting with a fully generic rules engine.

Not included in current implementation:
- Bonus event persistence
- Group/tribe modeling for bonus gameplay
- Event participation/provenance tracking
- Bonus scoring integration into leaderboard
- Discord bot display updates for draft vs bonus vs total
