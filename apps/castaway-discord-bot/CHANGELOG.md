# Changelog

All notable changes to `castaway-discord-bot` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/bry-guy/srvivor/compare/castaway-discord-bot-v0.1.0...castaway-discord-bot-v0.2.0) (2026-04-16)


### Features

* add castaway discord bot app ([1d6be12](https://github.com/bry-guy/srvivor/commit/1d6be129a0b80f48197f3a001c86d74b660aeb49))
* **auth:** add discord-linked participant privacy ([8af1356](https://github.com/bry-guy/srvivor/commit/8af1356427842a1282ce2a9f4d2b0d5897f3901e))
* **auth:** move discord admin authz to db and simplify bot auth ([9c50717](https://github.com/bry-guy/srvivor/commit/9c5071701dc67f540a2b81cd856fdcc1fd8ebfd1))
* backfill season 50 bonus history ([0a675ce](https://github.com/bry-guy/srvivor/commit/0a675ce48f641c3416d1cde10ab4173caf98fd65))
* **castaway-discord-bot:** add activities and occurrences commands ([def0cbc](https://github.com/bry-guy/srvivor/commit/def0cbc50bc30dfc56750679bc154654127e7538))
* **castaway-discord-bot:** add activity detail command ([4731307](https://github.com/bry-guy/srvivor/commit/4731307f27be6bf94704ff43bd811397a207cfb6))
* **castaway-discord-bot:** add client methods and formatters for activities and occurrences ([24c7b89](https://github.com/bry-guy/srvivor/commit/24c7b89ef029feadfa6dd76812b31be539bb0b8e))
* **castaway-discord-bot:** add occurrence detail and history views ([9535c50](https://github.com/bry-guy/srvivor/commit/9535c5072ea167f1cdf936db3030885019ee6b77))
* **castaway-discord-bot:** humanize manual adjustment history ([25ec78b](https://github.com/bry-guy/srvivor/commit/25ec78b69c861a15e51c9a1b5922f471e8ce3ba9))
* **castaway-discord-bot:** show bonus point breakdowns ([5aa6224](https://github.com/bry-guy/srvivor/commit/5aa622488280b1513ba8f27a1e20c122a2c0cb6a))
* **discord:** alphabetize commands, rename clear→unset, cap history at current episode, consistent titles ([2fed817](https://github.com/bry-guy/srvivor/commit/2fed817c8c3d798544824692d9e99d3ef064247d))
* **discord:** clarify score breakdown and instance context ([ea3ce6c](https://github.com/bry-guy/srvivor/commit/ea3ce6cf6484e1687df207c4fe9c2adc774d6898))
* **discord:** clean up command ux and local integration runs ([5f5c96d](https://github.com/bry-guy/srvivor/commit/5f5c96d2b1efd0e15ee7733d19dbc2e88fa489a7))
* **discord:** consistent bullet formatting across all commands ([f2a3ae9](https://github.com/bry-guy/srvivor/commit/f2a3ae9e5b22b6e12140d7c70d44372cb3a01bf3))
* **discord:** group history by episode ([17b0a46](https://github.com/bry-guy/srvivor/commit/17b0a46825947cd9473a1cc9b3e323e35815c46c))
* **discord:** simplify command options and history formatting ([042a230](https://github.com/bry-guy/srvivor/commit/042a2306a098c1f117dcdeaf8c9822473ff9da81))
* support bot api auth and postgres state backend ([81721ea](https://github.com/bry-guy/srvivor/commit/81721eaf4b8ae330d520da77686386c8a13ac47a))


### Bug Fixes

* **castaway-discord-bot:** accept numeric history involvement ids ([be4b543](https://github.com/bry-guy/srvivor/commit/be4b54337c755d74d6ac431a3d2d5735698ac04a))
* **castaway-discord-bot:** parse nested participant activity history ([c455e69](https://github.com/bry-guy/srvivor/commit/c455e697b64290c690508daaff67ad4a6144d734))

## [Unreleased]

### Changed
- Render `/castaway score` and `/castaway scores` with total points plus draft and visible bonus breakdowns from the leaderboard API.

## [0.1.0] - 2026-03-06

### Added
- Standalone Discord bot app with slash commands for scores, drafts, and instance context.
- Local stack integration through Docker Compose and root `mise run start` / `stop`.
- fnox + 1Password integration for Discord secrets.
