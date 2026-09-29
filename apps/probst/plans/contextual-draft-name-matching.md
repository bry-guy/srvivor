# Contextual draft-name matching

Status: done

## Goal

Recognize supported variations of the show's contestant names in complete drafts, without inventing permanent nicknames or filling arbitrary missing names. Keep the web watched-thread parser and Probst importer coherent.

## Verified examples

- Keeling's unedited message `1554599095459651617`, posted `2026-09-29T21:01:47.412Z`: `2 brody` is an existing clear fuzzy match for Brady Booker; `17 an` needs context to identify Thien An Nguyen.
- Mooney's unedited message `1554628735611969549`, posted `2026-09-29T22:59:34.175Z`: `2.  Rhodey Rob` and `14.  Shondra` are unresolved under the existing matcher. The live draft was separately corrected through a targeted installed-Probst import; the original fixture is untouched.

Both messages are from draft thread `1552521406137372833`. Offline fixture copies live in Probst's `testdata/` and the web API's `internal/httpapi/testdata/`. Tests use the full name `Thien An Nguyen` without the artificial `An` nickname.

## Bounded approach

1. Keep existing exact and clear full-roster fuzzy matches; thresholds and margins are unchanged.
2. On a complete draft with distinct accepted picks and valid numbering, resolve a single remaining partial name only when it matches a whole word of the sole unused contestant's name. Report this as `inferred`.
3. Retain supported unnumbered partial-name lines for that contextual check, without counting unresolved words as confident draft matches.
4. User chose conservative matching: Mooney's weak typo and trailing-name evidence produce suggestions only, requiring an explicitly corrected submission. No hardcoded aliases, automatic suffix matching, lowered acceptance thresholds, blind reduced-roster fuzzy matching, or general assignment solver. Suggestion-only candidates use the existing full-roster similarity scores (minimum 0.5, margin 0.1) or a trailing exact alias; they never set a contestant or draft order.
5. Preserve rejection of arbitrary text, duplicates, malformed numbering, incomplete drafts, and unresolved collisions.

## Verification

The original-message HTTP/database integration cases assert all 21 ranks, preservation of a pending second-submission claim and its +1 bonus, and replay idempotency for draft picks, bonus ledger, tribe membership, and announcement creation. Mooney's unedited message must produce review diagnostics and no picks or rewards before a corrected revision. Unit tests exercise both parser copies and reject weak suggestions, malformed drafts, and ambiguous aliases even when only one contestant is unused.

With user permission, the existing Probst broken-draft import regression now uses `Unknown Contestant` rather than `An` as its invalid input; its no-submission assertions remain intact. User also approved checked assertions for pre-existing web regression lint errors, without changing their behavioral expectations.

Validation passed: Probst CI (vet, unit tests, build) and its run/help smoke check; web CI (lint, unit tests, build, OpenAPI parity, local Hurl server regression); and the full disposable PostgreSQL integration suite, including both original messages. Active Go LSP checks found no language diagnostics. Auxiliary GORM rules falsely flag string formatting in these parsers, which have no database imports or calls.

Deployment remains a separate, unapproved step. No production parser/name changes have been applied. The existing TypeSpec dependency audit warnings and pre-launch auth documentation drift are outside this matching change.
