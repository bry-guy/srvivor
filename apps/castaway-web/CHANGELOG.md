# Changelog

All notable changes to `castaway-web` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/bry-guy/srvivor/compare/castaway-web-v0.1.0...castaway-web-v0.2.0) (2026-04-16)


### Features

* add bot-friendly castaway-web filters ([320c181](https://github.com/bry-guy/srvivor/commit/320c181046d55e5f82e9302886e8ac61aa4781a8))
* add castaway-web persistent API ([#5](https://github.com/bry-guy/srvivor/issues/5)) ([35c0522](https://github.com/bry-guy/srvivor/commit/35c052223ef5c7b3d0e8f178955e0abdaf6f9ee3))
* **auth:** add discord-linked participant privacy ([8af1356](https://github.com/bry-guy/srvivor/commit/8af1356427842a1282ce2a9f4d2b0d5897f3901e))
* **auth:** move discord admin authz to db and simplify bot auth ([9c50717](https://github.com/bry-guy/srvivor/commit/9c5071701dc67f540a2b81cd856fdcc1fd8ebfd1))
* backfill season 50 bonus history ([0a675ce](https://github.com/bry-guy/srvivor/commit/0a675ce48f641c3416d1cde10ab4173caf98fd65))
* backfill season 50 legacy data and gameplay docs ([38e7330](https://github.com/bry-guy/srvivor/commit/38e7330a6fe19af993ceb67ca648ae35ea50af45))
* **castaway-discord-bot:** add activity detail command ([4731307](https://github.com/bry-guy/srvivor/commit/4731307f27be6bf94704ff43bd811397a207cfb6))
* **castaway-web:** add activities and occurrences handlers ([8b199fe](https://github.com/bry-guy/srvivor/commit/8b199fef6a384f7403beb05caedb88b467498d14))
* **castaway-web:** add activity detail read APIs ([f99a984](https://github.com/bry-guy/srvivor/commit/f99a984131ba5aa23308eaf27ac9e147e5e7e9f3))
* **castaway-web:** add bonus points persistence foundation ([c4f95fd](https://github.com/bry-guy/srvivor/commit/c4f95fd32f4323cd279ff38b621130d2860f9406))
* **castaway-web:** add service auth and migration entrypoint ([8433ab2](https://github.com/bry-guy/srvivor/commit/8433ab26ff2f8284463274be6b8a824afbe6fdca))
* **castaway-web:** add TypeSpec contract for activities, occurrences, and resolve APIs ([02f826c](https://github.com/bry-guy/srvivor/commit/02f826ccd07508e78f681c52ec31c1790d848c3b))
* **castaway-web:** seed season 50 bonus activities as first-class gameplay ([812f761](https://github.com/bry-guy/srvivor/commit/812f7619e0aaab655a211a8bdc47a60b63eab260))
* **discord:** clarify score breakdown and instance context ([ea3ce6c](https://github.com/bry-guy/srvivor/commit/ea3ce6cf6484e1687df207c4fe9c2adc774d6898))
* **discord:** clean up command ux and local integration runs ([5f5c96d](https://github.com/bry-guy/srvivor/commit/5f5c96d2b1efd0e15ee7733d19dbc2e88fa489a7))
* **discord:** group history by episode ([17b0a46](https://github.com/bry-guy/srvivor/commit/17b0a46825947cd9473a1cc9b3e323e35815c46c))

## [0.1.0] - 2026-03-06

### Added
- Persistent Castaway web API with instances, participants, drafts, outcomes, and leaderboard endpoints.
- Bot-friendly instance, participant, and leaderboard filters.
- OpenAPI generation and route parity checks to prevent API drift.
