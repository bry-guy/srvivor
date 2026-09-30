# Castawordle test access and scheduled puzzles

Status: done

## Approved scope

Keep existing instance-admin checks. Games without an episode are admin-only tests, including existing previews; preserve their answers and progress. Admins choose an answer and an episode ahead of time. Linked players can see scheduled games and play only from 1pm Eastern on that episode date through noon on the following episode date. All games stay unscored.

## Implementation

Migration 021 adds nullable `episode_number`, a same-instance episode foreign key, and one-puzzle-per-instance/episode uniqueness. Filter lists and check detail/play/guess routes server-side. Derive clock times in `America/New_York`, with embedded timezone data. Use the next episode's date when available; for the final registered episode, close at noon seven Eastern calendar days later. Reject intermediate schedule gaps; do not invent an episode. Reject expired windows, caller-provided scheduled timestamps, and duplicates without changing existing answers. No cron, new roles, or legacy scoring-round creation.

## Verification and delivery

Targeted PostgreSQL and 320px/desktop browser checks passed: private-test denial, linked-player access, exact opening/cutoff, duplicate preparation, cross-instance denial, preserved progress, retry/concurrency behavior, DST/non-8pm airtimes, and no scoring writes. Browser form checks select an episode and prepare a future puzzle. Full monorepo CI and full PostgreSQL/browser integration also passed. TypeSpec retains seven existing dependency-audit findings; no dependency changes were made. LSP probes were unavailable; compiler, lint, and runtime checks passed. Independent review identified missing finale support; the approved next-Wednesday window now covers the finale, with DST and selector/API regression checks. Full CI and PostgreSQL/browser integration passed again after the finale fix. Implementation and the separately authorized production rollout are complete; see [deployment evidence](castawordle-scheduled-deployment.md).

The owner explicitly authorized merging and pushing, replacing the original feature-only restriction. Merged current main locally without conflicts, preserving its web/bot deployment image pins and the main-only image-updater guard. After separate deployment approval, Argo was pinned to verified release revision `bc1ad1b`; migration and live smoke checks passed. No production puzzle creation or scoring changes are part of this delivery.
