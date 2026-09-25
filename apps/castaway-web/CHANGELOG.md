# Changelog

All notable changes to `castaway-web` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/bry-guy/srvivor/compare/castaway-web-v0.1.0...castaway-web-v0.2.0) (2026-09-25)


### Features

* add bot-friendly castaway-web filters ([320c181](https://github.com/bry-guy/srvivor/commit/320c181046d55e5f82e9302886e8ac61aa4781a8))
* add castaway-web persistent API ([#5](https://github.com/bry-guy/srvivor/issues/5)) ([35c0522](https://github.com/bry-guy/srvivor/commit/35c052223ef5c7b3d0e8f178955e0abdaf6f9ee3))
* add managed progression checkpoints ([8c6d860](https://github.com/bry-guy/srvivor/commit/8c6d860287984964d48fc38bc3e15904e5cf69ff))
* add Probst announcements delivered by the Discord bot ([d697612](https://github.com/bry-guy/srvivor/commit/d6976124d777a9fd967631a127122773dbb4fc86))
* **auth:** add discord-linked participant privacy ([8af1356](https://github.com/bry-guy/srvivor/commit/8af1356427842a1282ce2a9f4d2b0d5897f3901e))
* **auth:** move discord admin authz to db and simplify bot auth ([9c50717](https://github.com/bry-guy/srvivor/commit/9c5071701dc67f540a2b81cd856fdcc1fd8ebfd1))
* backfill season 50 bonus history ([0a675ce](https://github.com/bry-guy/srvivor/commit/0a675ce48f641c3416d1cde10ab4173caf98fd65))
* backfill season 50 legacy data and gameplay docs ([38e7330](https://github.com/bry-guy/srvivor/commit/38e7330a6fe19af993ceb67ca648ae35ea50af45))
* **castaway-discord-bot:** add activity detail command ([4731307](https://github.com/bry-guy/srvivor/commit/4731307f27be6bf94704ff43bd811397a207cfb6))
* **castaway-web:** add activities and occurrences handlers ([8b199fe](https://github.com/bry-guy/srvivor/commit/8b199fef6a384f7403beb05caedb88b467498d14))
* **castaway-web:** add activity detail read APIs ([f99a984](https://github.com/bry-guy/srvivor/commit/f99a984131ba5aa23308eaf27ac9e147e5e7e9f3))
* **castaway-web:** add bonus points persistence foundation ([c4f95fd](https://github.com/bry-guy/srvivor/commit/c4f95fd32f4323cd279ff38b621130d2860f9406))
* **castaway-web:** add Season 51 episode schedule ([7ca8a74](https://github.com/bry-guy/srvivor/commit/7ca8a742222700b05ad3d68bd10fcb9a97c10f16))
* **castaway-web:** add service auth and migration entrypoint ([8433ab2](https://github.com/bry-guy/srvivor/commit/8433ab26ff2f8284463274be6b8a824afbe6fdca))
* **castaway-web:** add TypeSpec contract for activities, occurrences, and resolve APIs ([02f826c](https://github.com/bry-guy/srvivor/commit/02f826ccd07508e78f681c52ec31c1790d848c3b))
* **castaway-web:** seed season 50 bonus activities as first-class gameplay ([812f761](https://github.com/bry-guy/srvivor/commit/812f7619e0aaab655a211a8bdc47a60b63eab260))
* **castaway:** add high signal observability ([9f746c6](https://github.com/bry-guy/srvivor/commit/9f746c646eb281f4743909af15b5f30292650932))
* **discord:** clarify score breakdown and instance context ([ea3ce6c](https://github.com/bry-guy/srvivor/commit/ea3ce6cf6484e1687df207c4fe9c2adc774d6898))
* **discord:** clean up command ux and local integration runs ([5f5c96d](https://github.com/bry-guy/srvivor/commit/5f5c96d2b1efd0e15ee7733d19dbc2e88fa489a7))
* **discord:** group history by episode ([17b0a46](https://github.com/bry-guy/srvivor/commit/17b0a46825947cd9473a1cc9b3e323e35815c46c))
* editable draft announcements, one-off bot messages, Wordle results file ([1970a3d](https://github.com/bry-guy/srvivor/commit/1970a3d7e1ff9951928ba7c6f617eda381af00e3))
* minimized two-guild Discord bot, Probst admin CLI, channel bindings, Wordle rounds ([ec380a1](https://github.com/bry-guy/srvivor/commit/ec380a1ae50215cf25fd931ecad0bf914a64407a))
* reschedule/unschedule pending announcements; move to Hurl 8 ([c96c79a](https://github.com/bry-guy/srvivor/commit/c96c79a40b9b104771305f4e612cb0f02ed756b1))
* Season 51 weekly loop (tribes, tribe challenges, Wordle) ([6cfa524](https://github.com/bry-guy/srvivor/commit/6cfa5242f4f4a9fc1ce701e6744f76668a2d7540))


### Bug Fixes

* enforce managed score publication boundaries ([9022490](https://github.com/bry-guy/srvivor/commit/90224901bf550a34d763bc992fa00d056832bb9d))
* make Stir the Pot close deterministic ([51ca015](https://github.com/bry-guy/srvivor/commit/51ca015cb143d3f76daa5fc9c425cf4e4edd4087))
* repair Discord API contracts ([93d1ea6](https://github.com/bry-guy/srvivor/commit/93d1ea60a713651197a5054bf2ceaae7468c3086))

## [Unreleased]

### Added
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
