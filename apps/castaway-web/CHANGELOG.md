# Changelog

All notable changes to `castaway-web` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/bry-guy/castaway/compare/castaway-web-v0.1.0...castaway-web-v0.2.0) (2026-10-23)


### Features

* add authenticated unscored Castawordle preview ([598ea45](https://github.com/bry-guy/castaway/commit/598ea452a8dd9e0991961a0278a7acd77ecb2082))
* add bot-friendly castaway-web filters ([320c181](https://github.com/bry-guy/castaway/commit/320c181046d55e5f82e9302886e8ac61aa4781a8))
* add castaway-web persistent API ([#5](https://github.com/bry-guy/castaway/issues/5)) ([35c0522](https://github.com/bry-guy/castaway/commit/35c052223ef5c7b3d0e8f178955e0abdaf6f9ee3))
* add managed progression checkpoints ([8c6d860](https://github.com/bry-guy/castaway/commit/8c6d860287984964d48fc38bc3e15904e5cf69ff))
* add Probst announcements delivered by the Discord bot ([d697612](https://github.com/bry-guy/castaway/commit/d6976124d777a9fd967631a127122773dbb4fc86))
* admin DM approval gate for posts and reply-to-fix drafts ([b91ab81](https://github.com/bry-guy/castaway/commit/b91ab811e3e2f5782ca3377bd2a788bbf5ae644d))
* admin-only participant pronouns for Probst copy, with castaway-pronouns skill ([bf78432](https://github.com/bry-guy/castaway/commit/bf78432b678167a51ff4e617df845a1518cb3b6c))
* **auth:** add discord-linked participant privacy ([8af1356](https://github.com/bry-guy/castaway/commit/8af1356427842a1282ce2a9f4d2b0d5897f3901e))
* **auth:** move discord admin authz to db and simplify bot auth ([9c50717](https://github.com/bry-guy/castaway/commit/9c5071701dc67f540a2b81cd856fdcc1fd8ebfd1))
* automation heartbeat; admins DMed when the scheduled job stalls ([0100b97](https://github.com/bry-guy/castaway/commit/0100b97f5baca45ff5dc977732c54eea47d49d0c))
* backfill season 50 bonus history ([0a675ce](https://github.com/bry-guy/castaway/commit/0a675ce48f641c3416d1cde10ab4173caf98fd65))
* backfill season 50 legacy data and gameplay docs ([38e7330](https://github.com/bry-guy/castaway/commit/38e7330a6fe19af993ceb67ca648ae35ea50af45))
* bot watches a draft thread and saves complete drafts ([a26be9b](https://github.com/bry-guy/castaway/commit/a26be9b21db275ccd98cbaf85c6e1626ddfbf944))
* **castaway-discord-bot:** add activity detail command ([4731307](https://github.com/bry-guy/castaway/commit/4731307f27be6bf94704ff43bd811397a207cfb6))
* **castaway-web:** add activities and occurrences handlers ([8b199fe](https://github.com/bry-guy/castaway/commit/8b199fef6a384f7403beb05caedb88b467498d14))
* **castaway-web:** add activity detail read APIs ([f99a984](https://github.com/bry-guy/castaway/commit/f99a984131ba5aa23308eaf27ac9e147e5e7e9f3))
* **castaway-web:** add bonus points persistence foundation ([c4f95fd](https://github.com/bry-guy/castaway/commit/c4f95fd32f4323cd279ff38b621130d2860f9406))
* **castaway-web:** add Season 51 episode schedule ([7ca8a74](https://github.com/bry-guy/castaway/commit/7ca8a742222700b05ad3d68bd10fcb9a97c10f16))
* **castaway-web:** add service auth and migration entrypoint ([8433ab2](https://github.com/bry-guy/castaway/commit/8433ab26ff2f8284463274be6b8a824afbe6fdca))
* **castaway-web:** add TypeSpec contract for activities, occurrences, and resolve APIs ([02f826c](https://github.com/bry-guy/castaway/commit/02f826ccd07508e78f681c52ec31c1790d848c3b))
* **castaway-web:** admin Inside Track corrections and probst track commands ([5338177](https://github.com/bry-guy/castaway/commit/5338177e444e45b4ca730ca4f8d0711fedc03a5e))
* **castaway-web:** admin Inside Track practice board via ?practice=1 while a game is open ([af9b734](https://github.com/bry-guy/castaway/commit/af9b734e071e604334723bf4c46226a8d14061d2))
* **castaway-web:** admin-only Inside Track at /games/inside-track, linked from games admin panel ([1e40405](https://github.com/bry-guy/castaway/commit/1e404053498b65200e973fc7848173558c069cd4))
* **castaway-web:** insert game results into edited scores drafts ([dd7b13f](https://github.com/bry-guy/castaway/commit/dd7b13fef277a8aa8c6b6c0d0ce440bdec520044))
* **castaway-web:** Inside Track 12 balls, deeper mat, shorter blue leg, throws count off the mat ([4943f65](https://github.com/bry-guy/castaway/commit/4943f65d9928ec36679de84f51628189259140ad))
* **castaway-web:** Inside Track beach, wood mat, torches and wooden balls; lighter ball; one sun ([d42b721](https://github.com/bry-guy/castaway/commit/d42b721494fbe77f7cc7efb56fabc6e20ea89fa6))
* **castaway-web:** Inside Track heavier ball, more friction ([81e3a3b](https://github.com/bry-guy/castaway/commit/81e3a3b6f54773f18d59272d17a321af22f8f136))
* **castaway-web:** Inside Track league points equal the in-game score ([d2cbfdc](https://github.com/bry-guy/castaway/commit/d2cbfdce1f5f089cf12a9d6d284d9dd7c7650732))
* **castaway-web:** Inside Track lip-sized cups, grab again on the mat, lighter bigger ball, no hint text ([a01baba](https://github.com/bry-guy/castaway/commit/a01baba63079a3f524fe0f4564b717d9d8d05f22))
* **castaway-web:** Inside Track practice balls, gentler side tracks ([9d4fd38](https://github.com/bry-guy/castaway/commit/9d4fd3889c970e81c1dbe43ad35e05d6e5d30831))
* **castaway-web:** Inside Track practice tosses on the mat, mat side wall, heavier ball ([6c57011](https://github.com/bry-guy/castaway/commit/6c570119e6d9c16e3fa9714a0fddd90d313a086a))
* **castaway-web:** Inside Track prototype page ([601feee](https://github.com/bry-guy/castaway/commit/601feee05f0e729daeb5ed6035a1db6af25c0353))
* **castaway-web:** Inside Track rack, cups and full-table physics with retro styling ([5440123](https://github.com/bry-guy/castaway/commit/5440123df1d12308642884b7660263c404621c8d))
* **castaway-web:** Inside Track score is cups plus balls left ([3bf14a7](https://github.com/bry-guy/castaway/commit/3bf14a7bd211ffdf6e661ef96fd6c9e4ad2d9325))
* **castaway-web:** Inside Track stamped score animation ([de0d6b7](https://github.com/bry-guy/castaway/commit/de0d6b7a97dcd457de57a18f64ff09e0959fa41f))
* **castaway-web:** Inside Track three-track board, smoother throws, island styling ([d470a14](https://github.com/bry-guy/castaway/commit/d470a144490245c8058acca48d4efac8909cb4b9))
* **castaway-web:** Island Scramble speed + accuracy scoring and Monday hint ([f976afa](https://github.com/bry-guy/castaway/commit/f976afafec0f514e14a6121d28654324e5a76186))
* **castaway-web:** playable Inside Track prototype at /inside-track ([5da4043](https://github.com/bry-guy/castaway/commit/5da40439aad41fe2b4ee839ebc59ae1c698988dd))
* **castaway-web:** report build id from healthz ([0d3f550](https://github.com/bry-guy/castaway/commit/0d3f55044aba7e66e405769758b7e29e23443060))
* **castaway-web:** seed season 50 bonus activities as first-class gameplay ([812f761](https://github.com/bry-guy/castaway/commit/812f7619e0aaab655a211a8bdc47a60b63eab260))
* **castaway-web:** set tribe colors through the API, allow pink ([f6e5f01](https://github.com/bry-guy/castaway/commit/f6e5f0181f42fefbb804b9390b435fc261b10aa2))
* **castaway:** add high signal observability ([9f746c6](https://github.com/bry-guy/castaway/commit/9f746c646eb281f4743909af15b5f30292650932))
* Castawordle scores itself at cutoff; Season 51 games run episode to episode ([02bded2](https://github.com/bry-guy/castaway/commit/02bded21075c4f7dd2344d7cb0ebb1ea7974aea3))
* **castawordle:** require Enter to submit a guess ([d256e9e](https://github.com/bry-guy/castaway/commit/d256e9e5a08db3ae3c4b39ed7931b1a616e5e518))
* declare postgres dependency with Score ([fe3c727](https://github.com/bry-guy/castaway/commit/fe3c72743d1e178b344f242cff13dd07a82dd8e5))
* **discord:** clarify score breakdown and instance context ([ea3ce6c](https://github.com/bry-guy/castaway/commit/ea3ce6cf6484e1687df207c4fe9c2adc774d6898))
* **discord:** clean up command ux and local integration runs ([5f5c96d](https://github.com/bry-guy/castaway/commit/5f5c96d2b1efd0e15ee7733d19dbc2e88fa489a7))
* **discord:** group history by episode ([17b0a46](https://github.com/bry-guy/castaway/commit/17b0a46825947cd9473a1cc9b3e323e35815c46c))
* DM-approvable episode imports; probst episode import ([22fe457](https://github.com/bry-guy/castaway/commit/22fe457bc49cbd7486a08684d51f74e675dcd69f))
* draft submissions as events with order bonus, balanced tribes, and Jeff posts ([6a0745c](https://github.com/bry-guy/castaway/commit/6a0745cf9a8a0c967130cc7619c60863e3ad54e7))
* edit held posts by DM reply; late approval posts right away; Season 51 scores at 8pm ([a2391dd](https://github.com/bry-guy/castaway/commit/a2391dd69f8a12864d62d954cd5f4919ca15df76))
* editable draft announcements, one-off bot messages, Wordle results file ([1970a3d](https://github.com/bry-guy/castaway/commit/1970a3d7e1ff9951928ba7c6f617eda381af00e3))
* games hub with season-visible results; Press the Button hints, tribe totals, streaks, first/last presser ([fc6753d](https://github.com/bry-guy/castaway/commit/fc6753d1bfade8d4a5514426b9136628cbfa1d84))
* Inside Track weekly game; Island Scramble before-hint scoring ([b57458d](https://github.com/bry-guy/castaway/commit/b57458d7cdf57206079ed7c6cd24005a42d9ecff))
* league tiebreakers, private Castawordle, Button admin tests, probst scores generate ([a7967bc](https://github.com/bry-guy/castaway/commit/a7967bc1ae34b2f7ed9a4bca641529ca50f51942))
* minimized two-guild Discord bot, Probst admin CLI, channel bindings, Wordle rounds ([ec380a1](https://github.com/bry-guy/castaway/commit/ec380a1ae50215cf25fd931ecad0bf914a64407a))
* missed-approval DM; late approvals post immediately for 7 days ([c97caad](https://github.com/bry-guy/castaway/commit/c97caade924a29c7d00a30462ca921772b5744ea))
* opt-in user-mention pings for announcements and messages (--notify) ([db4c6cc](https://github.com/bry-guy/castaway/commit/db4c6cce7cbd8ced91a2a5a601710e93ce89a7e4))
* Pick Your Champion (season merge, website + Discord reply picks, post-merge challenge scoring) ([649b153](https://github.com/bry-guy/castaway/commit/649b1538525c310aa4c52cbf3d68cfeec1127d39))
* Press the Button count and magnitude bonus ([7af9be6](https://github.com/bry-guy/castaway/commit/7af9be6962d940fe532976a6229148075653903f))
* Press the Button game, season apply/post with templated weekly scores ([d1c2872](https://github.com/bry-guy/castaway/commit/d1c28729eeaddd1e752a80b25beb8cc880c6680c))
* Press the Button hints for every scoring rule, picked per press on the server ([b1a89d6](https://github.com/bry-guy/castaway/commit/b1a89d68fb215c69dbbdbecdf8ceceba80224981))
* Press the Button opening prompt until first press ([239e522](https://github.com/bry-guy/castaway/commit/239e522e559c407b904722a044163b4c740f182a))
* Press the Button scores call out every rule; db-access skill ([49e6bc4](https://github.com/bry-guy/castaway/commit/49e6bc49a67eedc434e023907659a59ffd524ba5))
* **probst:** draft scores posts 12h before their approval DM ([0e0d168](https://github.com/bry-guy/castaway/commit/0e0d1685095884aeaffb8d9c8395863eee1fb27c))
* public listener with Discord login, session-only /api, probst login, access requests ([c6245d1](https://github.com/bry-guy/castaway/commit/c6245d11a2cf01cc810623e65b6577bad54f5809))
* rename contestants (PATCH /instances/{id}/contestants/{contestantID}, probst contestant rename) ([9457894](https://github.com/bry-guy/castaway/commit/9457894aa59c5fffdb9004802b14cd833ca61ee0))
* rename Spell It Out to Island Scramble; decouple probst tests from live season files ([6cc93ad](https://github.com/bry-guy/castaway/commit/6cc93ad80dd38539e418d87cd7e7765663a8fda7))
* reply 'overwrite' to replace a player's saved draft from an admin DM; probst announcement schedule --approval ([180c35b](https://github.com/bry-guy/castaway/commit/180c35b5b522f788ca432854bfaf9431f08dab0a))
* reschedule/unschedule pending announcements; move to Hurl 8 ([c96c79a](https://github.com/bry-guy/castaway/commit/c96c79a40b9b104771305f4e612cb0f02ed756b1))
* restrict test puzzles and prepare scheduled Castawordle games ([2da6731](https://github.com/bry-guy/castaway/commit/2da6731d7b3451e8733513dec761f91a413ddf33))
* scheduled probst check DMs admins when next week's game isn't ready (BrainLand first) ([fbd83e9](https://github.com/bry-guy/castaway/commit/fbd83e91e2b6aa7b98d8ec184de55c9f8f53c33e))
* scored Castawordle with episode windows, probst week, tribe dots ([5584af3](https://github.com/bry-guy/castaway/commit/5584af30a0c39806968062cf5ad0dad8d9b55206))
* Season 51 weekly loop (tribes, tribe challenges, Wordle) ([6cfa524](https://github.com/bry-guy/castaway/commit/6cfa5242f4f4a9fc1ce701e6744f76668a2d7540))
* season config synced to the database; scheduled reconcile runs the synced copy ([2b492ef](https://github.com/bry-guy/castaway/commit/2b492ef3f7e5a8cc2bb2b6deed37d6846453cfe4))
* season post templates (game results, reminders); no message text in code ([f1f84fe](https://github.com/bry-guy/castaway/commit/f1f84fef5da1cafb1a6eeaa4ac99af51880a1137))
* season reconcile — safe survivoR import and approval-gated scores drafts (BrainLand only) ([ca59cc0](https://github.com/bry-guy/castaway/commit/ca59cc0813428727081b994bb1a61a4296a76fc4))
* season sync --fix-episodes; season post --preview ([29716f4](https://github.com/bry-guy/castaway/commit/29716f474e7ca7e09985cb342b41cf7c89c1494a))
* Spell It Out word-scramble game with decoys, timed from Start, top three score ([2c2f54f](https://github.com/bry-guy/castaway/commit/2c2f54fb8211465e0b6f8d58b8405b21dd3fcf09))
* store tribe buff colors; draft posts and leaderboard use them (Savu purple, Toka yellow) ([f5be227](https://github.com/bry-guy/castaway/commit/f5be2271bc64d157715a69dcb007a050b51fcd08))
* survivoR episode sync, tribe buffs, mark-sent announcements, hide undrafted players ([9211685](https://github.com/bry-guy/castaway/commit/92116858980e57060c3401ebb8a1e5f8de473011))
* thread routine draft buffs under one Jeff post; 1st/2nd/last stay in channel ([df3e3ff](https://github.com/bry-guy/castaway/commit/df3e3fff43fb6e571e0a5f2c9e3ba22d6c31c242))
* tribe dots from stored colors; hold duplicate exits; test-change rule ([5bdf48a](https://github.com/bry-guy/castaway/commit/5bdf48ae8e05694033171344b73ce786eef16750))
* use SCOWL guesses and streamline score and error feedback ([212aad2](https://github.com/bry-guy/castaway/commit/212aad220aa3d49dc72688a84754292eabbb2002))
* **web:** draft short names, per-pick scoring, scored eliminations; hide empty Tribe column ([1a031ce](https://github.com/bry-guy/castaway/commit/1a031cea8668c1f23ef77113521fd0ef3a4ec306))
* **web:** Games hub, winner trophies, bonus column only when used, drop standings links ([c7ccba1](https://github.com/bry-guy/castaway/commit/c7ccba18e13b3f8a51029498403b5184dedfcf02))
* **web:** league season standings and hidden participant pronouns ([1bbf1eb](https://github.com/bry-guy/castaway/commit/1bbf1eb3a9bfa77c1874dfda326fa7f8fd868b1c))
* **web:** player profiles with Me page, clickable Scores names, and league history ([c9e453f](https://github.com/bry-guy/castaway/commit/c9e453f21617f30c31228612d988f140f460618d))
* **web:** players set their own pronouns on Me (visible only to themself) ([5a02ec8](https://github.com/bry-guy/castaway/commit/5a02ec8a6d9511ca17322fca3405349ca5280f50))
* **web:** server messages from templates ([18fa060](https://github.com/bry-guy/castaway/commit/18fa060653887ed24fb6472d402306ac5836b1f9))
* **web:** show per-pick points as +N with (+value − distance) on hover ([81895d5](https://github.com/bry-guy/castaway/commit/81895d5645bcbee756970fdb34dc67b3a017e918))


### Bug Fixes

* approval gate review — atomic revision claim, retryable DMs, no late sends, no gate bypass, participant-level fix guard ([172bb26](https://github.com/bry-guy/castaway/commit/172bb263f7fc73161fe38cc150dbc16bbbd1746c))
* auto-submit full Castawordle guesses and darken absent keys ([6678d15](https://github.com/bry-guy/castaway/commit/6678d15e8bcb1845e592284210a3594221c4c59a))
* **castaway-web:** auth status reports Discord login sessions accurately ([a5bd3f1](https://github.com/bry-guy/castaway/commit/a5bd3f1c0a23733c5ebf66fe53c85f5703763a48))
* **castaway-web:** count Inside Track cups without int-to-uint conversion (gosec G115) ([81f2a09](https://github.com/bry-guy/castaway/commit/81f2a09233a93967fe37e69570214647bf79a690))
* **castaway-web:** Inside Track throws when the finger crosses the start line ([feea86c](https://github.com/bry-guy/castaway/commit/feea86cef87956bb2494786b1aa5a2f7d7b85f66))
* **castaway-web:** Inside Track waits for a ball on a cup lip to drop before scoring ([209840f](https://github.com/bry-guy/castaway/commit/209840f2a66b424b1525b7196d4149b8f24575de))
* **castaway-web:** Inside Track white practice balls can't fill a scored cup ([82e3597](https://github.com/bry-guy/castaway/commit/82e35972e002f10201bd40372a2780fe3c683f75))
* **castaway-web:** PUBLIC_INSTANCE_ID must be the public UUID; set Season 51's ([8a3a72c](https://github.com/bry-guy/castaway/commit/8a3a72c1bb6c2ceceee63a323507e92594250997))
* **castaway-web:** reveal drafts once the season is underway ([9444da5](https://github.com/bry-guy/castaway/commit/9444da5a57d682e90af6a88b9319555a153ff88f))
* check button count lookup error; clearer AI flavor message ([3b27bf3](https://github.com/bry-guy/castaway/commit/3b27bf3d2b963fd43f183a6021e0f4f4f4be82c9))
* enforce managed score publication boundaries ([9022490](https://github.com/bry-guy/castaway/commit/90224901bf550a34d763bc992fa00d056832bb9d))
* include pinned Castawordle dictionary assets ([5edf4a8](https://github.com/bry-guy/castaway/commit/5edf4a84d59de7b5126df6f01fba88ec2b372476))
* infer contextual draft names and require weak-match confirmation ([1b27db8](https://github.com/bry-guy/castaway/commit/1b27db84a63221c9b6f1597e73512216f41eb12f))
* keep login return path for old /button links ([6766cba](https://github.com/bry-guy/castaway/commit/6766cba801b9f6472fdcb1fcd9cf2d5c8d7de264))
* make Stir the Pot close deterministic ([51ca015](https://github.com/bry-guy/castaway/commit/51ca015cb143d3f76daa5fc9c425cf4e4edd4087))
* repair Discord API contracts ([93d1ea6](https://github.com/bry-guy/castaway/commit/93d1ea60a713651197a5054bf2ceaae7468c3086))
* retain the weekly puzzle window after the finale ([72695bc](https://github.com/bry-guy/castaway/commit/72695bcfaa47b3c8c19c4888a7ca93966db69dae))
* Spell It Out drag and drop, 30 letters plus decoys, lighter game tile font ([dea5413](https://github.com/bry-guy/castaway/commit/dea54132a72689f80c814d4cca9255e6814822c5))
* Spell It Out script clash with admin script; browser check covers drag and solve ([bd99d2e](https://github.com/bry-guy/castaway/commit/bd99d2e4558bf8f69827942d4388219b71b59f4b))
* **web:** accept signed OAuth state when mobile sign-in returns in another browser ([8f36cc2](https://github.com/bry-guy/castaway/commit/8f36cc266734048c1682f70d890f5261ba81b9a3))
* **web:** confirm the Discord account before cookieless sign-in (login CSRF) ([5ff4550](https://github.com/bry-guy/castaway/commit/5ff4550ae56be51eb9063e89981dce2bcff1c8fe))
* **web:** keep wrapped season stats right-aligned ([ac024f6](https://github.com/bry-guy/castaway/commit/ac024f692b292f54c668a78dbd050362a4ea0389))
* **web:** return to Me and player pages after sign-in; gofmt probst week ([9b6d2f2](https://github.com/bry-guy/castaway/commit/9b6d2f277e08d58c1f487e7293b9a2c2cc446300))
* **web:** subtle pronouns hint with hover edit; tidy season list rows ([2cf4fcb](https://github.com/bry-guy/castaway/commit/2cf4fcb9fc722c1ac3ad98f82e3b15e5e5153d48))

## [Unreleased]
- Player profiles reveal drafts once any episode is scored, even if draft submissions were never closed (players saw "Drafts are revealed after drafts close" mid-season).
- Inside Track admin corrections: `GET /instances/:id/track-games/:episode/plays` (admin) lists every run with place, score, cups, balls used, finished and league points, before or after resolve. `PUT /instances/:id/track-games/:episode/plays/:participant` (admin) sets `cups_mask` (0–7), `balls_used` (0–12) and `finished`, validated like progress; it may move a run backwards, returns 409 once the game resolves, and writes only the play row.
- Scores posts drafted before their game is scored may carry a placeholder (`PUT /instances/:id/score-posts/:key` takes `awaiting_results`). Such a post is held at send time instead of sent; an unchanged-standings redraft with new text now replaces an unedited draft (and re-asks) instead of being ignored (migration 040).
- `PUT /instances/:id/score-posts/:key` takes `game_results` and `results_placeholder`. Once the game is scored, an admin-edited draft that still has its placeholder line gets the results in its place (`action` `results-inserted`), keeps the admin's text, and is re-asked; without the line it stays held and admins get a `scores-results-missing` alert (`action` `results-missing`).
- Tribe colors through the API: `PUT /instances/:id/tribes` takes an optional `color` per tribe; a tribe that doesn't exist yet needs one (so a typo can't create a tribe). `PATCH /instances/:id/tribes/:name` (admin) recolors an existing tribe without touching its membership history. `GET /instances/:id/tribes` returns each tribe's `color` and `emoji`. Pink (🩷, `#ff4fa3`) joins the colors (migration 039).
- Island Scramble scoring: +1 for solving, +1 for solving before the hint, fewest wrong checks +2 and second +1 (max 4; ties share); no speed points. Game results replace `time_points` with `hint_points`.
- Inside Track weekly game (`/games/inside-track`, migration 038): 3 practice balls then 12 scored balls into three foam-track cups; score is cups plus balls left once all three are in; one run per player, saved after every ball so a reload resumes. League points equal the score. Admin API `POST /instances/:id/track-games`, `POST /track-games/:id/resolve`; game results type `inside_track` with `score`, `cups`, `balls_left`. Admins get the practice board when no game is open.

- Island Scramble hint: from 8am Eastern on the fifth day after opening (Monday), the page shows one letter per word, picked from a hash of the game ID.

- Press the Button game results include each player's rule breakdown (`rank_rule`, `rank_points`, `volume_points`, `streak_points`).

- Draft buff posts, the draft-rewards thread, and admin alerts (rescored post, stale automation) come from embedded templates in `internal/httpapi/messages/`.

- `PUT /instances/:id/episodes` (admin) creates or moves a season's episodes to match a season file.

- Weekly scores posts include the week's game results (new `GET /instances/:id/game-results/:episode`), and automated posts without a score fingerprint (game-day reminders) use the same approval-gated `score-posts` upsert.

- Pick Your Champion: `probst season merge FILE N` ends Tribe scoring at merge Episode N and opens picks (website `/games/champion`, or reply to the bot's "Pick Your Champion" post) until Episode N+1 airs; then picks are fixed and shown to all. Post-merge challenges import from survivoR and award pickers of the winners (+3 immunity, +1 reward). The merge episode's own challenges are held for an admin.

- Failure alert: each successful scheduled `season reconcile --instance` run stamps a heartbeat; if none lands for 30 minutes, admins get one DM per stall.

- Season config lives in the database: `probst season sync FILE` saves the season file and scores template (revision-checked, refuses on episode mismatches); the scheduled `season reconcile --instance ID` runs the synced copy.

- A held post whose time passes unapproved gets one "⏰ Missed" DM; replying **yes** posts it immediately (late approvals now allowed for 7 days, was 3 hours).

- Episode imports that need judgment (medevac/quit, two people leaving) are DMed to admins as an exact proposal; replying **yes** records it (`POST /admin-alerts/:id/approve`, revision-bound). `probst episode import N --file F` replaces `season import`/`episode sync`.

- Spell It Out is now called **Island Scramble** (display name only; `spell_it_out`, URLs and data unchanged).

- Season automation (migrations 031–032): one-time admin alert DMs (`PUT /instances/:id/admin-alerts/:key`, delivered by the bot); `GET /instances/:id/scheduled-games` (each week's game and whether it's scored, no answers); `POST /instances/:id/episode-imports` applies an episode's boots and tribe challenge wins in one transaction, reusing matching hand-entered results and refusing conflicting ones; weekly standings snapshots (`/score-snapshots/:week`, frozen once that week's post is sent); `PUT /instances/:id/score-posts/:key` saves automation's scores post for approval tied to the standings it describes. Such a post can't be approved or sent once scores change; untouched drafts are redrafted, admin-edited ones keep the admin's text and are re-asked.

- Fix: Spell It Out did nothing for admins, because `scramble.js` and `game-admin.js` both declared a top-level `form` and the second script failed to load; both now run in their own block. The browser check now covers Spell It Out (30 letters + 10 decoys at 320px, drag to fill/swap/return, wrong check, solve) and fails on any page error.
- Spell It Out: drag tiles onto slots (swapping with what is there) or back to the tray, alongside tap-to-place; phrases may be up to 30 letters plus up to 10 decoys (decoys no longer count against the limit). Game tiles in Spell It Out and Castawordle use a regular-width bold system font instead of Impact.
- Spell It Out (migration 029): arrange letter tiles (the phrase plus random decoys) into a hidden phrase at `/games/scramble/:id`. Tiles stay hidden until the player presses Start, which starts their clock; checks only say right or wrong. At cutoff the three fastest solves earn +3/+2/+1 (fewer wrong checks breaks a time tie; exact ties share). Admins create games with `POST /api/instances/:id/scramble-games` (or Probst `season apply` with `type: spell_it_out, phrase_file, decoys`) and one-hour admin-only tests from `/games`. Phrases are A–Z and spaces, up to 30 tiles with decoys; words longer than 9 letters wrap.
- Press the Button hints now come from the server and hint at every scoring rule without explaining it: first press, landing on a shared count, taking first or second place, a first press at the bottom, 3–5 day streaks, and 100/1,000/10,000 presses, plus 10/50/250/500 as no-points noise.
- Games hub: every game lives under `/games` (episode list, `?season=` for past seasons), `/games/castawordle/:id` and `/games/button/:id`; old `/castawordle` and `/button` links redirect. Closed games show everyone's results to the season (Castawordle after close with the answer; Press the Button once scored), and profiles list a player's game results.
- Press the Button (migration 028): count readout, Savu/Toka press totals, hints at 10/50/100/250/500/1,000 presses (no points), daily streaks (3/4/5+ Eastern days: +1/+2/+3), first presser +1, last presser −1.
- Admin DM approvals (migration 027): `PUT .../announcements/{id}/approval` holds a post until an instance admin replies "yes" to the bot's DM; it then sends at exactly its time (never late, never other text; edits re-ask).
- Draft problems DM every instance admin; replying with `7. Thien An` lines resubmits the corrected draft as the player's (admins only; never over a saved draft).
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
