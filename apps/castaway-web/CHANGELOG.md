# Changelog

All notable changes to `castaway-web` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Press the Button shows a small "count: N" under the button, and players earn +1 per order of magnitude past 10 presses (100: +1, 1,000: +2, 10,000+: +3), on top of the ranking rules.
- Standings tiebreakers everywhere: total, then draft points (without bonus), then earliest first draft submission.
- Private Castawordle (`POST /castawordle/{id}/private`, migration 026): one player's own puzzle for a scored week. Only they see it, it hides the original from them, and its result replaces theirs in the original's round.
- Press the Button admin test games: "Test Press the Button" on Games makes a one-hour admin-only game at `/button/{id}` that never scores.
- Press the Button (/button, migration 025): one big button, presses counted per player, scored at cutoff (most +2, 2nd most -1, least +1, shared counts +group size up to 3); resolves itself each minute. Leaderboard accepts `?at=`; outcomes include `updated_at`.
- Profile drafts use short names (survivoR `castaway`, migration 024) and show each scored pick as (+value − distance) with an ⓘ explainer; current-season eliminations are italic once scored. Past standings hide Tribe when a season had none.
- Nav "Castawordle" is now "Games" (/games hub); past standings show a 🏆 for winners and hide Bonus when a season had none; profile season header links to standings.
- Players can view and change their own pronouns on Me; pronouns are never shown to other players.
- Seasons page lists league seasons with winners; past seasons have standings pages, and past league players have profiles. Participants gain a hidden pronouns field (migration 023).
- Profiles: Me page and clickable Scores names show a season score and draft plus league history (secret points hidden; current drafts revealed after drafts close). Castawordle guesses submit with Enter. Cookieless mobile sign-in asks to confirm the Discord account.
- Make test puzzles admin-only, preserving existing answers/progress. Admins can prepare one player-visible puzzle per instance episode; derive the 1pm–next-episode-noon Eastern window from the instance schedule, with DST support and duplicate protection (migration 021). Gameplay remains unscored.

- Replace the pronunciation dictionary with SCOWL size 70 US/UK words and inflections; preserve historical saved-guess replay. Invalid-word errors read `invalid word, try again` and clear after edits. Remove the Scores page's Draft column without changing scoring.

- Automatically check complete Castawordle guesses on touch/physical keyboards; rejected words remain editable and cost no turn. Darken ruled-out keyboard letters in both themes.

- Discord-authenticated content pages, responsive system typography, and device-following/saved light/dark themes.
- Unscored Last Torch Castawordle at `/castawordle/{gameID}`: instance-owned 4–8-letter puzzles, private server answers, six guesses, duplicate-letter feedback, transactional retries, and persisted own-player progress (migration 020).
- Admin trial creation and PostgreSQL/browser coverage for authorization, mobile layout, theme persistence, concurrent moves, lost responses, resume, and absence of scoring side effects.

- Watched drafts can infer a single whole name word from the sole unused contestant in a complete, valid draft. Weak suggestions require a corrected submission; automatic fuzzy thresholds and ambiguity checks remain unchanged.
- Test the captured Keeling and Mooney original messages offline, including all ranks, no rewards for unconfirmed guesses, pending submission credit, and idempotent replay.

- Draft buffs after 1st/2nd go to a shared "Draft rewards" thread; announcements can target a bot-opened thread (migration 019).

- `PATCH /instances/{id}/contestants/{contestantID}` renames a contestant (admin only).

- Optional public listener (`PUBLIC_PORT`): website, Discord login, and session-only `/api`; access requests; `POST /instances/{id}/admins`.

### Added
- Watched draft threads: `PUT .../draft-submissions/thread`, `GET /draft-threads`, `POST /draft-threads/{id}/messages`, and `POST .../draft-submissions/{participantID}/reject`. A player's first draft post claims their submission order even if it has problems; migration 018 records handled message versions.
- Draft-submission events: `POST /instances/{id}/draft-submissions` (open) and `/close`; first drafts record order, +2/+1 bonus, a balanced random tribe, and a queued announcement atomically.
- Leaderboard rows include `has_draft`; `POST .../announcements/{id}/sent` records hand-posted announcements.
- Announcements accept `notify_users` (migration 017) so `<@user>` mentions can ping; `@everyone`/roles never do.
- Draft announcements (migration 016): create with `draft: true`, edit text (`PUT .../body`), schedule or send now (`PUT .../schedule`), unschedule back to draft (`DELETE .../schedule`), or delete, until the bot claims it. The bot sends the latest text.
- Season 51 episode schedule (CBS Wednesdays 8pm ET), admin tribe assignment over time (`/instances/{id}/tribes`), tribe immunity/reward awards (`/tribe-challenges`, +2/+1), and opt-in Wordle scoring: +2 best individual, +1 to the tribe with the best submitter average.
- API-owned Discord channel bindings, restricted first-admin bootstrap, and transactional player-link administration for Probst.
- Repeatable Season 43 Hurl rehearsal using pinned survivoR data, fake drafts and public Wordle awards, with explicit opt-in accelerated BrainLand delivery.
- Admin-operated Wordle rounds for legacy instances, with explicit submission windows, public bonus awards, atomic resolution, and retry-safe creation/closure/resolution.

## [0.1.0] - 2026-03-06

### Added
- Persistent Castaway web API with instances, participants, drafts, outcomes, and leaderboard endpoints.
- Bot-friendly instance, participant, and leaderboard filters.
- OpenAPI generation and route parity checks to prevent API drift.
