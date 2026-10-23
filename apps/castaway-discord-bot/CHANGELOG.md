# Changelog

All notable changes to `castaway-discord-bot` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/bry-guy/castaway/compare/castaway-discord-bot-v0.1.0...castaway-discord-bot-v0.2.0) (2026-10-23)


### Features

* add castaway discord bot app ([1d6be12](https://github.com/bry-guy/castaway/commit/1d6be129a0b80f48197f3a001c86d74b660aeb49))
* add Probst announcements delivered by the Discord bot ([d697612](https://github.com/bry-guy/castaway/commit/d6976124d777a9fd967631a127122773dbb4fc86))
* admin DM approval gate for posts and reply-to-fix drafts ([b91ab81](https://github.com/bry-guy/castaway/commit/b91ab811e3e2f5782ca3377bd2a788bbf5ae644d))
* **auth:** add discord-linked participant privacy ([8af1356](https://github.com/bry-guy/castaway/commit/8af1356427842a1282ce2a9f4d2b0d5897f3901e))
* **auth:** move discord admin authz to db and simplify bot auth ([9c50717](https://github.com/bry-guy/castaway/commit/9c5071701dc67f540a2b81cd856fdcc1fd8ebfd1))
* backfill season 50 bonus history ([0a675ce](https://github.com/bry-guy/castaway/commit/0a675ce48f641c3416d1cde10ab4173caf98fd65))
* bot watches a draft thread and saves complete drafts ([a26be9b](https://github.com/bry-guy/castaway/commit/a26be9b21db275ccd98cbaf85c6e1626ddfbf944))
* **bot:** all Discord messages from templates ([6ae75b8](https://github.com/bry-guy/castaway/commit/6ae75b890c3142b0221c1c8545450eb465cd1534))
* **castaway-discord-bot:** add activities and occurrences commands ([def0cbc](https://github.com/bry-guy/castaway/commit/def0cbc50bc30dfc56750679bc154654127e7538))
* **castaway-discord-bot:** add activity detail command ([4731307](https://github.com/bry-guy/castaway/commit/4731307f27be6bf94704ff43bd811397a207cfb6))
* **castaway-discord-bot:** add client methods and formatters for activities and occurrences ([24c7b89](https://github.com/bry-guy/castaway/commit/24c7b89ef029feadfa6dd76812b31be539bb0b8e))
* **castaway-discord-bot:** add occurrence detail and history views ([9535c50](https://github.com/bry-guy/castaway/commit/9535c5072ea167f1cdf936db3030885019ee6b77))
* **castaway-discord-bot:** humanize manual adjustment history ([25ec78b](https://github.com/bry-guy/castaway/commit/25ec78b69c861a15e51c9a1b5922f471e8ce3ba9))
* **castaway-discord-bot:** show bonus point breakdowns ([5aa6224](https://github.com/bry-guy/castaway/commit/5aa622488280b1513ba8f27a1e20c122a2c0cb6a))
* **castaway:** add high signal observability ([9f746c6](https://github.com/bry-guy/castaway/commit/9f746c646eb281f4743909af15b5f30292650932))
* declare postgres dependency with Score ([fe3c727](https://github.com/bry-guy/castaway/commit/fe3c72743d1e178b344f242cff13dd07a82dd8e5))
* **discord:** alphabetize commands, rename clear→unset, cap history at current episode, consistent titles ([2fed817](https://github.com/bry-guy/castaway/commit/2fed817c8c3d798544824692d9e99d3ef064247d))
* **discord:** clarify score breakdown and instance context ([ea3ce6c](https://github.com/bry-guy/castaway/commit/ea3ce6cf6484e1687df207c4fe9c2adc774d6898))
* **discord:** clean up command ux and local integration runs ([5f5c96d](https://github.com/bry-guy/castaway/commit/5f5c96d2b1efd0e15ee7733d19dbc2e88fa489a7))
* **discord:** consistent bullet formatting across all commands ([f2a3ae9](https://github.com/bry-guy/castaway/commit/f2a3ae9e5b22b6e12140d7c70d44372cb3a01bf3))
* **discord:** group history by episode ([17b0a46](https://github.com/bry-guy/castaway/commit/17b0a46825947cd9473a1cc9b3e323e35815c46c))
* **discord:** simplify command options and history formatting ([042a230](https://github.com/bry-guy/castaway/commit/042a2306a098c1f117dcdeaf8c9822473ff9da81))
* DM-approvable episode imports; probst episode import ([22fe457](https://github.com/bry-guy/castaway/commit/22fe457bc49cbd7486a08684d51f74e675dcd69f))
* edit held posts by DM reply; late approval posts right away; Season 51 scores at 8pm ([a2391dd](https://github.com/bry-guy/castaway/commit/a2391dd69f8a12864d62d954cd5f4919ca15df76))
* minimized two-guild Discord bot, Probst admin CLI, channel bindings, Wordle rounds ([ec380a1](https://github.com/bry-guy/castaway/commit/ec380a1ae50215cf25fd931ecad0bf914a64407a))
* missed-approval DM; late approvals post immediately for 7 days ([c97caad](https://github.com/bry-guy/castaway/commit/c97caade924a29c7d00a30462ca921772b5744ea))
* opt-in user-mention pings for announcements and messages (--notify) ([db4c6cc](https://github.com/bry-guy/castaway/commit/db4c6cce7cbd8ced91a2a5a601710e93ce89a7e4))
* Pick Your Champion (season merge, website + Discord reply picks, post-merge challenge scoring) ([649b153](https://github.com/bry-guy/castaway/commit/649b1538525c310aa4c52cbf3d68cfeec1127d39))
* **probst:** draft scores posts 12h before their approval DM ([0e0d168](https://github.com/bry-guy/castaway/commit/0e0d1685095884aeaffb8d9c8395863eee1fb27c))
* public listener with Discord login, session-only /api, probst login, access requests ([c6245d1](https://github.com/bry-guy/castaway/commit/c6245d11a2cf01cc810623e65b6577bad54f5809))
* reply 'overwrite' to replace a player's saved draft from an admin DM; probst announcement schedule --approval ([180c35b](https://github.com/bry-guy/castaway/commit/180c35b5b522f788ca432854bfaf9431f08dab0a))
* scheduled probst check DMs admins when next week's game isn't ready (BrainLand first) ([fbd83e9](https://github.com/bry-guy/castaway/commit/fbd83e91e2b6aa7b98d8ec184de55c9f8f53c33e))
* support bot api auth and postgres state backend ([81721ea](https://github.com/bry-guy/castaway/commit/81721eaf4b8ae330d520da77686386c8a13ac47a))
* survivoR episode sync, tribe buffs, mark-sent announcements, hide undrafted players ([9211685](https://github.com/bry-guy/castaway/commit/92116858980e57060c3401ebb8a1e5f8de473011))
* thread routine draft buffs under one Jeff post; 1st/2nd/last stay in channel ([df3e3ff](https://github.com/bry-guy/castaway/commit/df3e3fff43fb6e571e0a5f2c9e3ba22d6c31c242))


### Bug Fixes

* approval gate review — atomic revision claim, retryable DMs, no late sends, no gate bypass, participant-level fix guard ([172bb26](https://github.com/bry-guy/castaway/commit/172bb263f7fc73161fe38cc150dbc16bbbd1746c))
* **castaway-discord-bot:** accept numeric history involvement ids ([be4b543](https://github.com/bry-guy/castaway/commit/be4b54337c755d74d6ac431a3d2d5735698ac04a))
* **castaway-discord-bot:** parse nested participant activity history ([c455e69](https://github.com/bry-guy/castaway/commit/c455e697b64290c690508daaff67ad4a6144d734))
* **castaway-discord-bot:** suppress link previews on announcements and admin DMs ([eb80f65](https://github.com/bry-guy/castaway/commit/eb80f650571c8f8cb23fc2d3e0e9a6d86a479738))
* group bot imports correctly for CI ([cbcdc3f](https://github.com/bry-guy/castaway/commit/cbcdc3f52f65ed971c20094c7d628a134de20af4))
* repair Discord API contracts ([93d1ea6](https://github.com/bry-guy/castaway/commit/93d1ea60a713651197a5054bf2ceaae7468c3086))

## [Unreleased]

- Approval DMs state the send time in Eastern, up front and bold (`posts Wed Oct 21 8:00pm ET`), on the first ask, re-asks, and "missed" DMs; "approved" replies use the same format.

- Announcements and admin DMs post with link previews suppressed.

- Every user-facing Discord message (DMs, replies, channel posts, score/leaderboard formatting, player-visible errors) now comes from embedded templates in `internal/messages/templates/`; output is unchanged.

- Pick Your Champion: `probst season merge FILE N` ends Tribe scoring at merge Episode N and opens picks (website `/games/champion`, or reply to the bot's "Pick Your Champion" post) until Episode N+1 airs; then picks are fixed and shown to all. Post-merge challenges import from survivoR and award pickers of the winners (+3 immunity, +1 reward). The merge episode's own challenges are held for an admin.

- A held post whose time passes unapproved gets one "⏰ Missed" DM; replying **yes** posts it immediately (late approvals now allowed for 7 days, was 3 hours).

- Episode imports that need judgment (medevac/quit, two people leaving) are DMed to admins as an exact proposal; replying **yes** records it (`POST /admin-alerts/:id/approve`, revision-bound). `probst episode import N --file F` replaces `season import`/`episode sync`.

- Posts threaded announcements, opening the thread from a starter message on first use and reopening it if deleted.

- `/castaway scores` and `score` hide players who have not drafted.
### Changed
- Restrict `/castaway` to `score [player]`, `scores`, and `draft [player]`, using API channel bindings, ephemeral public-only replies, and explicit BrainLand/Podracing guild registration.
- Restrict registration and dispatch to the configured multi-guild allowlist without clearing globals or falling back to global registration.
- Render `/castaway score` and `/castaway scores` with total points plus draft and visible bonus breakdowns from the leaderboard API.

## [0.1.0] - 2026-03-06

### Added
- Reads watched draft threads (needs the Message Content intent), forwards posts to the API, and DMs the admin contact about each problem draft.
- Standalone Discord bot app with slash commands for scores, drafts, and instance context.
- Local stack integration through Docker Compose and root `mise run start` / `stop`.
- fnox + 1Password integration for Discord secrets.
