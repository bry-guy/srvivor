# Castawordle and responsive website

Status: in-progress

## Goal and owner decisions

Keep the current website almost as simple as raw HTML, while making its typography, light/dark presentation, and mobile layout pleasant. Add a playable Castawordle whose results feed Castaway scoring.

Selected direction:

- Last Torch: familiar Wordle gameplay, with six torches extinguishing on incorrect accepted guesses.
- Survivor-inspired puzzle lettering and tiles, rather than ordinary flat letter boxes.
- Operator-defined word lengths, including seven and eight letters; do not hardcode five columns.
- First-pass implementation and feature-branch hosting are approved. All content pages require Discord login; OAuth entry/callback, non-sensitive assets, and health checks remain reachable.
- Preview is unscored: save play progress, but do not create scoring occurrences, participant scoring inputs, or bonus awards. Scoring integration below is deferred.
- Each game has its own public UUID and belongs to a Castaway instance. `/castawordle/:id` is its stable player URL.
- Follow-up approved: tests (including existing preview games) are admin-only; admins prepare an answer for an episode ahead of time. Scheduled puzzles are visible to linked players and open from 1pm Eastern on that episode date through noon on the next episode date. Merge/push is now authorized; scored gameplay remains deferred.
- Temporarily host the branch at `castaway.bry-guy.net`, preserving the existing Discord callback. The requested `castaway.preview.bry-guy.net` is outside free Cloudflare Universal SSL coverage; the owner approved production replacement as the simple fallback.

## Recommended architecture

Keep the existing `castaway-web` Go application, PostgreSQL, Discord login, public hostname, and deployment. Use server-rendered `html/template`, embedded same-origin CSS/JavaScript, and one small `<castawordle-game>` web component. No new frontend framework, generic game engine, service, event bus, build pipeline, or paid service. HTMX is not necessary for these pages.

### Routing

- `/` stays the scores/leaderboard page.
- `/castawordle` lists games for the configured Castaway instance, linked beside Scores in a small header.
- `/castawordle/:gameID` is one game's server-rendered page.
- `GET /api/castawordle/:gameID/play` returns public puzzle rules and the authenticated player's saved play.
- `POST /api/castawordle/:gameID/play/guesses` submits one guess and returns updated state.
- Instance admins create unscored games through `POST /api/instances/:instanceID/castawordle` or the game-list page's admin form. Omitted/null `episode_number` creates an admin-only test; selecting an episode creates a scheduled puzzle using that instance's stored episode dates. One puzzle per instance/episode; no caller-defined scheduled timestamps or background scheduler.

Use the existing session middleware, plus narrow player-route allowlisting and round/instance authorization inside each handler. Resolve the participant from the session's Discord identity in the round's instance; do not accept a participant ID, actor header, or reported score from the browser. Restrict v1 play to the configured current season. An instance admin without a player link may configure a puzzle but cannot fabricate a player play.

Existing round management and manual result-entry routes remain privileged. Reuse transaction-aware application functions, not internal HTTP calls to privileged handlers. Do not broaden the entire existing Wordle API to players.

### Minimal site presentation

- Use a readable native system-font stack for body text, comfortable line height, restrained spacing, and a modest content-width limit.
- Default to the device's light/dark preference. Provide a small Auto / Light / Dark control, with the explicit preference stored locally and shared across pages.
- Use CSS variables and native `color-scheme`; include accessible contrast and visible keyboard focus.
- Keep the leaderboard table semantic. On narrow screens, allow scrolling inside its wrapper rather than making the whole page overflow.
- Serve external same-origin CSS/JS. The existing CSP (`default-src 'self'`) should not be weakened for inline scripts/styles, CDN libraries, or remote fonts.
- If needed for the tile lettering, self-host one small, clearly licensed display-font asset. Body text stays ordinary and readable; no third-party font requests.

### Last Torch game presentation

Keep the recognizable grid, colored feedback, Enter/Backspace controls, and on-screen QWERTY keyboard. Add original block/stencil-like lettering and a subtle carved/painted tile treatment reminiscent of Survivor challenge puzzles. Do not copy show artwork or NYT assets.

Six torches sit above the board. An incorrect accepted guess extinguishes one; invalid dictionary words consume no guess and no torch. A correct final guess leaves the final torch lit. The guess limit stays six initially, regardless of length.

The board is six rows by the configured answer length. Derive its column count from returned puzzle rules. Eight columns must fit a 320px-wide phone without horizontal scrolling. Scale tile size and gaps with available width; keep touch keyboard targets usable. Support physical keyboards, mobile keyboards, and assistive technology. Keyboard handling must not steal input from other controls.

Feedback is not color-only: expose correct/present/absent states in accessible text and provide a high-contrast palette. Respect reduced-motion preferences; animations are optional and must not delay accepted moves.

Before visual implementation, obtain an owner-provided screenshot/reference of the intended Survivor puzzle lettering. The reference is not needed to settle the backend architecture.

## Puzzle rules and content

Recommended initial length range: 4–8 ASCII letters, default five. Length is derived from the operator-selected answer, not entered as a second potentially contradictory value. This range is a proposed initial bound, not a requirement already approved by the owner.

Every player in a round gets the same answer, length, dictionary version, and six-guess limit. Players do not choose an easier length for a scored round. Freeze the answer/rules when the round opens; do not change a live puzzle under saved plays.

Use an operator-curated answer with a broader licensed dictionary of valid same-length guesses. Ordinary English guesses remain valid; Survivor flavor need not force an obscure trivia-only answer list. Validate answer membership and length at configuration time. Pin and document the dictionary's source, license, normalization, and version before implementation. No runtime vocabulary service or copied proprietary answer list.

Keep the existing episode window: opens Wednesday 1pm America/New_York and closes at noon on the next episode's air day. Dates follow the existing episode schedule; derive explicit local clock times, not airtime offsets. Timezone data is embedded in the Go binary for DST-safe deployment. Use the following episode's date for cutoff; the final registered episode closes at noon seven Eastern calendar days later. Reject intermediate schedule gaps. Admin-only tests are supported; public practice, daily games, archives, hard mode, hints, and additional guess-limit settings remain out of scope.

Longer words are not automatically harder: letter information, repetition, vocabulary size, and candidate families all matter. Keep six guesses first and measure actual results by length before proposing a rule change.

## Persistence: two small additions

First pass uses independent unscored games, not scoring occurrences. The future bridge can attach a game to existing Wordle round/award machinery after its rules are approved.

1. A private `castawordle_games` record belonging to a Castaway instance: public UUID, name, answer, pinned dictionary version, opening and cutoff timestamps, and nullable episode number. NULL identifies admin-only tests; scheduled rows have a same-instance episode foreign key and unique instance/episode constraint. Keep the answer out of generic activity/occurrence metadata, which other APIs may expose.
2. A `castawordle_plays` record unique on game/player: a bounded JSON guess history, persisted in-progress/solved/exhausted status, and relevant timestamps. Expiration is derived from the game's cutoff without a scheduler.

Six guesses do not need a separate event-sourcing system or guess table. Recompute tile feedback from stored guesses and the private answer when resuming. Save accepted guesses on the server; browser storage is only for appearance preferences and optional unfinished typing, not game progress or scores.

Preserve the existing public round identifier convention: APIs use the activity-occurrence UUID, while migrations link to the correct internal Wordle round key. Do not conflate those IDs.

## Server-authoritative move flow

1. Authenticate and resolve the player's link to the round's instance.
2. Validate the requested round, window, puzzle, and word.
3. Serialize play creation/update in a database transaction, using a consistent lock order with existing close/resolve paths.
4. Check an idempotency key or expected guess position. A retry returns the already accepted response; a conflicting move requests resynchronization. Double taps, multiple tabs, and another device must not consume extra guesses.
5. Enforce same-length dictionary membership and the six-guess limit. Invalid words consume nothing; duplicate request delivery is not another attempt.
6. Compute feedback with the standard two-pass duplicate-letter algorithm: greens first, then allocate yellows from remaining answer-letter counts.
7. Persist the accepted guess and resulting state, and commit before confirming it to the client.

The browser sends guesses, not a solved flag or final count. A page reload or second device resumes the same authoritative play. New play/guess requests after cutoff are rejected, with a clear closed-round response rather than a false successful submission. Distinguish transport errors from invalid guesses so an offline phone does not lose a turn.

Never return the answer in HTML, JavaScript, CSS, dictionaries designated as solution lists, or an unfinished player's API response. A dictionary may naturally contain the word; it must not identify the chosen answer. At completion, the server may reveal the answer to that player. Do not expose other players' guess histories or answer-bearing share text during the live window. This is a low-stakes league, not a claim to prevent people sharing spoilers outside the app.

## Deferred: results and existing scoring

Automatically persist a terminal game result when solved or exhausted. A success is the actual accepted-guess count (1–6); the existing failure convention is 7. Keep explicit game status as well as the numeric adapter value, so failure is not confused with a seven-guess success when word length is seven.

At round closure, finalize eligible results into the existing occurrence-participant inputs, selecting each player's tribe at the existing cutoff instant. Use an atomic, idempotent bridge in the close/resolve transaction rather than assuming the current tribe at an earlier play-completion time is still correct. Missing player/tribe eligibility must remain visible to the operator; do not silently award or discard it.

Reuse the existing +2 best-individual and +1 best-tribe-average award machinery, including ties and the bonus ledger's replay protection. Results become visible in the app before award resolution; bonus points are awarded through normal round resolution, not by directly editing leaderboard totals.

For a browser-backed round, reject manual self-reported result writes/imports that would replace verified game results. Existing externally played/manual Wordle rounds remain supported without a puzzle record. Repeat completion, close, or resolve requests must not duplicate participant results or bonuses.

No new scheduler is necessary for v1: operator-driven close/resolve continues to work, with browser plays supplying the results automatically.

### Scoring policies to approve before implementation

- **Started but abandoned:** recommend that a play with at least one accepted guess counts as a failure at cutoff. Merely viewing the page creates no scoring participation. Never-started players remain excluded from submitter averages. This recommendation needs owner approval.
- **Everyone fails:** `internal/gameplay/season51.go` currently awards +2 to the lowest recorded count even when that count is the failure value 7. Recommend that a Castawordle round with no successful players award no individual or tribe winner bonus. This is a product/scoring decision, not an incidental parser change; approve it explicitly and preserve existing manual-round behavior unless separately changed.
- **Initial bounds:** confirm 4–8 letters with six guesses, or specify different supported bounds. Seven/eight-letter answers are required; the lower bound is proposed.

## Deferred: scored operator workflow

Extend the existing Probst Wordle opening workflow rather than adding a second game administration system. Proposed example:

```sh
probst wordle open --episode 2 --instance INSTANCE --puzzle-file puzzle.json
```

The untracked local file contains the chosen answer. The server derives length, validates dictionary membership, and creates the private puzzle with the round in the same transaction. Do not put live answers into committed fixtures, public generated schemas/examples, or printed diagnostic output. Existing Wordle opening without a puzzle file remains the external/manual-results path.

The browser game finds the season's active configured round through the server-rendered page; it does not need a season picker, game hub, or public puzzle editor. A web editor can be added later if CLI editing becomes a real inconvenience.

## Success-rate evidence and our own measurements

A NYT analysis published December 17, 2023, covering 515 million Wordle games reported failure rates of 1.7% for SLATE starters and 3.6% for ADIEU starters: corresponding success rates of 98.3% and 96.4% in those cohorts. The hardest puzzle by solve rate, JOKER, was solved by 71% in that analysis.

These are historical opening-word/puzzle-specific figures, not a universal overall solve rate or a forecast for seven/eight-letter Castawordle.

Sources:

- [Syndicated full text, Seven things we learned analyzing 515 million Wordles](https://edition.pagesuite.com/tribune/article_popover.aspx?guid=44a66681-0a7c-403b-90cf-8889d11b3be6)
- [Author's article index, identifying the NYT article and publication date](https://aatishb.com/articles/)

Use persisted plays for simple operator statistics by length: started, finished, solved, exhausted, and abandoned-at-cutoff; solve rate among started plays; completion rate; and mean guesses among successful plays. Define denominators and show the small sample size. No analytics SaaS, dashboard platform, or attempts to infer difficulty solely from letter count.

## Implementation sequence and acceptance checks

1. **Site polish:** shared templates/assets, body typography, theme preference, narrow-screen leaderboard. No gameplay changes.
2. **Rules and storage:** licensed dictionary, 4–8-letter validation, duplicate-letter scoring, puzzle/play migrations and repository queries. No live game until the contract is tested.
3. **Authenticated gameplay:** embedded component, own-player endpoints, transactional moves/resume/idempotency, operator puzzle configuration, scoring adapter and approved cutoff policies.
4. **Validation and documentation:** run documented app/monorepo CI and disposable integration workflows; keep TypeSpec/OpenAPI, app READMEs, requirements, changelogs, and readiness checklists aligned. Do not modify existing regression expectations without permission.

Required tests:

- Five-, seven-, and eight-letter puzzles, repeated letters, dictionary/length rejection, sixth-guess success and exhaustion.
- No answer leakage through public pages/API or generic occurrence metadata.
- Own-player and cross-instance authorization, anonymous/unlinked rejection, CSRF checks.
- Retried moves, concurrent tabs/devices, persistence across reload/server restart, cutoff races, and immutable opened puzzles.
- No scoring participation from page views or invalid-only guesses; approved abandoned/all-failed behavior; cutoff-time tribe membership; prevention of manual overwrite.
- Completion/close/resolve replay never duplicates results or bonus awards.
- Touch and physical keyboards, 320px phones through desktop, light/dark/system modes, focus visibility, high-contrast feedback, and reduced motion.

Deployment and production smoke testing require a separate approved step. No extra hosting, paid services, or machine-global tooling is required by this architecture.
