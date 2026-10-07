-- One-off: reopen Season 51 Episode 2 Castawordle until Oct 7 7:59pm EDT, undoing its noon scoring
-- so it re-scores at the new cutoff. Run with: psql "$DB" -v commit=1 -f this-file (default rolls back).
\set ON_ERROR_STOP on
BEGIN;

-- The scored game (id 2, round occurrence 55) and Bryan's private replacement (id 3).
UPDATE castawordle_games SET cutoff_at = '2026-10-07 23:59:00+00'
 WHERE id IN (2, 3) AND cutoff_at = '2026-10-07 15:59:00+00';

-- Remove the 5 Castawordle awards (Marv +2, four Toka finishers +1) so re-scoring can award them afresh.
DELETE FROM bonus_point_ledger_entries WHERE activity_occurrence_id = 55;
-- Results are re-recorded from the plays at resolution.
DELETE FROM activity_occurrence_participants WHERE activity_occurrence_id = 55;

UPDATE activity_occurrences SET status = 'recorded', ends_at = NULL, effective_at = '2026-10-07 23:59:00+00'
 WHERE id = 55 AND status = 'resolved';
UPDATE wordle_rounds SET cutoff_at = '2026-10-07 23:59:00+00', closed_at = NULL, resolution_response = NULL
 WHERE activity_occurrence_id = 55;

-- New cadence: weekly games run episode to episode, closing 7:59pm ET before the next episode.
UPDATE button_games SET cutoff_at = '2026-10-14 23:59:00+00'
 WHERE episode_number = 3 AND cutoff_at = '2026-10-14 15:59:00+00' AND resolved_at IS NULL;
SELECT episode_number, opens_at, cutoff_at FROM button_games WHERE episode_number = 3;

SELECT g.id, g.cutoff_at, o.status, o.effective_at, w.cutoff_at AS round_cutoff, w.closed_at,
       (SELECT count(*) FROM bonus_point_ledger_entries WHERE activity_occurrence_id = 55) AS awards
  FROM castawordle_games g, activity_occurrences o JOIN wordle_rounds w ON w.activity_occurrence_id = o.id
 WHERE g.id IN (2, 3) AND o.id = 55;

\if :{?commit}
COMMIT;
\else
ROLLBACK;
\endif
