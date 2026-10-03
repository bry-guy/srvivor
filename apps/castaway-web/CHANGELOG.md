# Changelog

All notable changes to `castaway-web` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/bry-guy/srvivor/compare/castaway-web-v0.1.0...castaway-web-v0.2.0) (2026-10-03)


### Features

* add authenticated unscored Castawordle preview ([598ea45](https://github.com/bry-guy/srvivor/commit/598ea452a8dd9e0991961a0278a7acd77ecb2082))
* add bot-friendly castaway-web filters ([320c181](https://github.com/bry-guy/srvivor/commit/320c181046d55e5f82e9302886e8ac61aa4781a8))
* add castaway-web persistent API ([#5](https://github.com/bry-guy/srvivor/issues/5)) ([35c0522](https://github.com/bry-guy/srvivor/commit/35c052223ef5c7b3d0e8f178955e0abdaf6f9ee3))
* add managed progression checkpoints ([8c6d860](https://github.com/bry-guy/srvivor/commit/8c6d860287984964d48fc38bc3e15904e5cf69ff))
* add Probst announcements delivered by the Discord bot ([d697612](https://github.com/bry-guy/srvivor/commit/d6976124d777a9fd967631a127122773dbb4fc86))
* admin-only participant pronouns for Probst copy, with castaway-pronouns skill ([bf78432](https://github.com/bry-guy/srvivor/commit/bf78432b678167a51ff4e617df845a1518cb3b6c))
* **auth:** add discord-linked participant privacy ([8af1356](https://github.com/bry-guy/srvivor/commit/8af1356427842a1282ce2a9f4d2b0d5897f3901e))
* **auth:** move discord admin authz to db and simplify bot auth ([9c50717](https://github.com/bry-guy/srvivor/commit/9c5071701dc67f540a2b81cd856fdcc1fd8ebfd1))
* backfill season 50 bonus history ([0a675ce](https://github.com/bry-guy/srvivor/commit/0a675ce48f641c3416d1cde10ab4173caf98fd65))
* backfill season 50 legacy data and gameplay docs ([38e7330](https://github.com/bry-guy/srvivor/commit/38e7330a6fe19af993ceb67ca648ae35ea50af45))
* bot watches a draft thread and saves complete drafts ([a26be9b](https://github.com/bry-guy/srvivor/commit/a26be9b21db275ccd98cbaf85c6e1626ddfbf944))
* **castaway-discord-bot:** add activity detail command ([4731307](https://github.com/bry-guy/srvivor/commit/4731307f27be6bf94704ff43bd811397a207cfb6))
* **castaway-web:** add activities and occurrences handlers ([8b199fe](https://github.com/bry-guy/srvivor/commit/8b199fef6a384f7403beb05caedb88b467498d14))
* **castaway-web:** add activity detail read APIs ([f99a984](https://github.com/bry-guy/srvivor/commit/f99a984131ba5aa23308eaf27ac9e147e5e7e9f3))
* **castaway-web:** add bonus points persistence foundation ([c4f95fd](https://github.com/bry-guy/srvivor/commit/c4f95fd32f4323cd279ff38b621130d2860f9406))
* **castaway-web:** add Season 51 episode schedule ([7ca8a74](https://github.com/bry-guy/srvivor/commit/7ca8a742222700b05ad3d68bd10fcb9a97c10f16))
* **castaway-web:** add service auth and migration entrypoint ([8433ab2](https://github.com/bry-guy/srvivor/commit/8433ab26ff2f8284463274be6b8a824afbe6fdca))
* **castaway-web:** add TypeSpec contract for activities, occurrences, and resolve APIs ([02f826c](https://github.com/bry-guy/srvivor/commit/02f826ccd07508e78f681c52ec31c1790d848c3b))
* **castaway-web:** seed season 50 bonus activities as first-class gameplay ([812f761](https://github.com/bry-guy/srvivor/commit/812f7619e0aaab655a211a8bdc47a60b63eab260))
* **castaway:** add high signal observability ([9f746c6](https://github.com/bry-guy/srvivor/commit/9f746c646eb281f4743909af15b5f30292650932))
* **castawordle:** require Enter to submit a guess ([d256e9e](https://github.com/bry-guy/srvivor/commit/d256e9e5a08db3ae3c4b39ed7931b1a616e5e518))
* **discord:** clarify score breakdown and instance context ([ea3ce6c](https://github.com/bry-guy/srvivor/commit/ea3ce6cf6484e1687df207c4fe9c2adc774d6898))
* **discord:** clean up command ux and local integration runs ([5f5c96d](https://github.com/bry-guy/srvivor/commit/5f5c96d2b1efd0e15ee7733d19dbc2e88fa489a7))
* **discord:** group history by episode ([17b0a46](https://github.com/bry-guy/srvivor/commit/17b0a46825947cd9473a1cc9b3e323e35815c46c))
* draft submissions as events with order bonus, balanced tribes, and Jeff posts ([6a0745c](https://github.com/bry-guy/srvivor/commit/6a0745cf9a8a0c967130cc7619c60863e3ad54e7))
* editable draft announcements, one-off bot messages, Wordle results file ([1970a3d](https://github.com/bry-guy/srvivor/commit/1970a3d7e1ff9951928ba7c6f617eda381af00e3))
* league tiebreakers, private Castawordle, Button admin tests, probst scores generate ([a7967bc](https://github.com/bry-guy/srvivor/commit/a7967bc1ae34b2f7ed9a4bca641529ca50f51942))
* minimized two-guild Discord bot, Probst admin CLI, channel bindings, Wordle rounds ([ec380a1](https://github.com/bry-guy/srvivor/commit/ec380a1ae50215cf25fd931ecad0bf914a64407a))
* opt-in user-mention pings for announcements and messages (--notify) ([db4c6cc](https://github.com/bry-guy/srvivor/commit/db4c6cce7cbd8ced91a2a5a601710e93ce89a7e4))
* Press the Button count and magnitude bonus ([7af9be6](https://github.com/bry-guy/srvivor/commit/7af9be6962d940fe532976a6229148075653903f))
* Press the Button game, season apply/post with templated weekly scores ([d1c2872](https://github.com/bry-guy/srvivor/commit/d1c28729eeaddd1e752a80b25beb8cc880c6680c))
* public listener with Discord login, session-only /api, probst login, access requests ([c6245d1](https://github.com/bry-guy/srvivor/commit/c6245d11a2cf01cc810623e65b6577bad54f5809))
* rename contestants (PATCH /instances/{id}/contestants/{contestantID}, probst contestant rename) ([9457894](https://github.com/bry-guy/srvivor/commit/9457894aa59c5fffdb9004802b14cd833ca61ee0))
* reschedule/unschedule pending announcements; move to Hurl 8 ([c96c79a](https://github.com/bry-guy/srvivor/commit/c96c79a40b9b104771305f4e612cb0f02ed756b1))
* restrict test puzzles and prepare scheduled Castawordle games ([2da6731](https://github.com/bry-guy/srvivor/commit/2da6731d7b3451e8733513dec761f91a413ddf33))
* scored Castawordle with episode windows, probst week, tribe dots ([5584af3](https://github.com/bry-guy/srvivor/commit/5584af30a0c39806968062cf5ad0dad8d9b55206))
* Season 51 weekly loop (tribes, tribe challenges, Wordle) ([6cfa524](https://github.com/bry-guy/srvivor/commit/6cfa5242f4f4a9fc1ce701e6744f76668a2d7540))
* survivoR episode sync, tribe buffs, mark-sent announcements, hide undrafted players ([9211685](https://github.com/bry-guy/srvivor/commit/92116858980e57060c3401ebb8a1e5f8de473011))
* thread routine draft buffs under one Jeff post; 1st/2nd/last stay in channel ([df3e3ff](https://github.com/bry-guy/srvivor/commit/df3e3fff43fb6e571e0a5f2c9e3ba22d6c31c242))
* use SCOWL guesses and streamline score and error feedback ([212aad2](https://github.com/bry-guy/srvivor/commit/212aad220aa3d49dc72688a84754292eabbb2002))
* **web:** draft short names, per-pick scoring, scored eliminations; hide empty Tribe column ([1a031ce](https://github.com/bry-guy/srvivor/commit/1a031cea8668c1f23ef77113521fd0ef3a4ec306))
* **web:** Games hub, winner trophies, bonus column only when used, drop standings links ([c7ccba1](https://github.com/bry-guy/srvivor/commit/c7ccba18e13b3f8a51029498403b5184dedfcf02))
* **web:** league season standings and hidden participant pronouns ([1bbf1eb](https://github.com/bry-guy/srvivor/commit/1bbf1eb3a9bfa77c1874dfda326fa7f8fd868b1c))
* **web:** player profiles with Me page, clickable Scores names, and league history ([c9e453f](https://github.com/bry-guy/srvivor/commit/c9e453f21617f30c31228612d988f140f460618d))
* **web:** players set their own pronouns on Me (visible only to themself) ([5a02ec8](https://github.com/bry-guy/srvivor/commit/5a02ec8a6d9511ca17322fca3405349ca5280f50))
* **web:** show per-pick points as +N with (+value − distance) on hover ([81895d5](https://github.com/bry-guy/srvivor/commit/81895d5645bcbee756970fdb34dc67b3a017e918))


### Bug Fixes

* auto-submit full Castawordle guesses and darken absent keys ([6678d15](https://github.com/bry-guy/srvivor/commit/6678d15e8bcb1845e592284210a3594221c4c59a))
* **castaway-web:** auth status reports Discord login sessions accurately ([a5bd3f1](https://github.com/bry-guy/srvivor/commit/a5bd3f1c0a23733c5ebf66fe53c85f5703763a48))
* **castaway-web:** PUBLIC_INSTANCE_ID must be the public UUID; set Season 51's ([8a3a72c](https://github.com/bry-guy/srvivor/commit/8a3a72c1bb6c2ceceee63a323507e92594250997))
* check button count lookup error; clearer AI flavor message ([3b27bf3](https://github.com/bry-guy/srvivor/commit/3b27bf3d2b963fd43f183a6021e0f4f4f4be82c9))
* enforce managed score publication boundaries ([9022490](https://github.com/bry-guy/srvivor/commit/90224901bf550a34d763bc992fa00d056832bb9d))
* include pinned Castawordle dictionary assets ([5edf4a8](https://github.com/bry-guy/srvivor/commit/5edf4a84d59de7b5126df6f01fba88ec2b372476))
* infer contextual draft names and require weak-match confirmation ([1b27db8](https://github.com/bry-guy/srvivor/commit/1b27db84a63221c9b6f1597e73512216f41eb12f))
* make Stir the Pot close deterministic ([51ca015](https://github.com/bry-guy/srvivor/commit/51ca015cb143d3f76daa5fc9c425cf4e4edd4087))
* repair Discord API contracts ([93d1ea6](https://github.com/bry-guy/srvivor/commit/93d1ea60a713651197a5054bf2ceaae7468c3086))
* retain the weekly puzzle window after the finale ([72695bc](https://github.com/bry-guy/srvivor/commit/72695bcfaa47b3c8c19c4888a7ca93966db69dae))
* **web:** accept signed OAuth state when mobile sign-in returns in another browser ([8f36cc2](https://github.com/bry-guy/srvivor/commit/8f36cc266734048c1682f70d890f5261ba81b9a3))
* **web:** confirm the Discord account before cookieless sign-in (login CSRF) ([5ff4550](https://github.com/bry-guy/srvivor/commit/5ff4550ae56be51eb9063e89981dce2bcff1c8fe))
* **web:** keep wrapped season stats right-aligned ([ac024f6](https://github.com/bry-guy/srvivor/commit/ac024f692b292f54c668a78dbd050362a4ea0389))
* **web:** return to Me and player pages after sign-in; gofmt probst week ([9b6d2f2](https://github.com/bry-guy/srvivor/commit/9b6d2f277e08d58c1f487e7293b9a2c2cc446300))
* **web:** subtle pronouns hint with hover edit; tidy season list rows ([2cf4fcb](https://github.com/bry-guy/srvivor/commit/2cf4fcb9fc722c1ac3ad98f82e3b15e5e5153d48))

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
