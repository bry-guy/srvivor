# castaway-web future work

## Follow-on work: bonus gameplay operations

The core bonus-points model is now implemented.

Implemented today:
- bonus event persistence
- group/tribe modeling for bonus gameplay
- occurrence/result provenance tables
- bonus scoring integration into leaderboard totals
- Discord bot display updates for draft vs bonus vs total

Remaining follow-on work:
- activity/occurrence write APIs for operators and clients
- Discord bot read workflows for activities and occurrences
- richer participant-facing auth for viewing secret bonus data
- continued cleanup of historical seed provenance where manual corrections still stand in for first-class gameplay events
