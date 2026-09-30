# Castawordle test access and scheduled puzzles

Status: in-progress

## Approved scope

Keep existing instance-admin checks. Games without an episode are admin-only tests, including existing previews; preserve their answers and progress. Admins choose an answer and an episode ahead of time. Linked players can see scheduled games and play only from 1pm Eastern on that episode date through noon on the following episode date. All games stay unscored.

## Implementation

Migration 021 adds nullable `episode_number`, a same-instance episode foreign key, and one-puzzle-per-instance/episode uniqueness. Filter lists and check detail/play/guess routes server-side. Derive clock times in `America/New_York`, with embedded timezone data; require both episode boundaries. Reject expired windows, caller-provided scheduled timestamps, and duplicates without changing existing answers. No cron, new roles, or legacy scoring-round creation.

## Verification and delivery

Targeted PostgreSQL and 320px/desktop browser checks passed: private-test denial, linked-player access, exact opening/cutoff, duplicate preparation, cross-instance denial, preserved progress, retry/concurrency behavior, DST/non-8pm airtimes, and no scoring writes. Browser form checks select an episode and prepare a future puzzle. Full monorepo CI and full PostgreSQL/browser integration also passed. TypeSpec retains seven existing dependency-audit findings; no dependency changes were made. LSP probes were unavailable; compiler, lint, and runtime checks passed. Final review precedes merge/push.

The owner explicitly authorized merging and pushing, replacing the original feature-only restriction. Preserve current main deployment image pins while merging. Argo remains pinned to the preview commit; merging is not activation. No production puzzle creation or scoring changes are part of this delivery.
