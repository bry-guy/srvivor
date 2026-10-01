# Castawordle scored release

Status: in-progress

## Approved contract

Prepare a player-visible scored Episode 2 puzzle in the existing Podracing Season 51 instance, with an explicit September 30, 2026, 8pm EDT opening and October 7, 2026, 11:59am EDT cutoff. Opening is now in the past; do not promise the missed launch deadline. Keep the operator-selected answer in private untracked input, not this document.

At closing, only completed submitted games count. Never-started or incomplete games earn zero Wordle points, including no Wordle tribe award. Use existing individual/tribe scoring for completed results without new special all-failed rules. Preserve generic historical Wordle behavior and unscored/private preview games. Awards must be derived from saved server results, occur only after cutoff, and be retry-safe.

Provide validated declarative puzzle preparation and explicit post-cutoff resolution, plus website tribe dots. Confirm the existing Season 51 tribe-challenge path implements +2 immunity/+1 reward beginning Episode 2; do not configure both old and new payout paths.

Latest delivery instruction supersedes scheduling: send the approved editable Markdown as a one-off Discord message only after the updated website and scored puzzle are verified live. No announcement scheduling or past-due queued announcement. Preserve user edits; show/reapprove any change to the opening wording. No launch message has been sent.

## Rollback baseline

Read-only preflight: Argo is pinned to `bc1ad1bf815b4d975b08f7e4d120210e0e78e8ff`, Synced/Healthy/Succeeded. Web digest `sha256:c88e738e56e9d0d87e955fcd5756022c2adf6010dcef2d2b59d0fcb28679bd4e`; bot digest `sha256:eaab924b52a07e49a7150d44eadd28950c49640e4cec37bdf9954e64dfef530f`. Both ready. Preserve bot/tunnel and credentials. Rollback uses the pinned revision and retains additive schema and progress; do not drop data.

## Verification gates

Pending: combined patch review, CI, disposable PostgreSQL tests for exact windows, privacy/access, completed-result awards, incomplete-result zero awards, no early scoring, replay/duplicate prevention; image publication, migration and live digest/readback, protected pages, puzzle metadata and CLI compatibility; final approved one-off message delivery with recorded Discord message ID. Keep source changes separate from user-authored announcement and private inputs in Git.
