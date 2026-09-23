# Season 43 rehearsal fixture

Historical source: [doehm/survivoR](https://github.com/doehm/survivoR), commit [`a6329b609b00021f904312664cc5fefe9241914a`](https://github.com/doehm/survivoR/commit/a6329b609b00021f904312664cc5fefe9241914a). The JSON fixture records SHA-256 hashes of the three source exports. MIT attribution and license are in `LICENSE.survivoR.md`.

## Mapping and provenance

Only US season 43 rows are included:

| Upstream | Fixture / Castaway |
| --- | --- |
| `castaways.castaway_id` | Stable upstream fixture ID; mapped to local contestant UUID by full name during setup |
| `castaways.full_name` | Contestant name |
| `castaways.place` | Outcome position (1 = winner, 18 = first exit) |
| `castaways.episode` | Episode in which that placement becomes public |
| `episodes.episode` | Instance episode number |
| `episodes.episode_date` | Historical calendar date |
| `boot_order` | Independent export cross-check of ID, episode and exit order |

Verified 18 unique contestants, placements 1–18, 13 episodes, `place = 19 - order`, two Episode 9 eliminations, and finale placements Gabler/Cassidy/Owen = 1/2/3. Finalists become known in Episode 13, not earlier. Source calendar dates do not assert broadcast time: noon UTC and a one-hour Wordle window are synthetic test timing.

The source is a pinned reference dataset, not an online runtime dependency. No R installation, runtime download, global contestant-ID migration, or schema rewrite is required. Future seasons with returns, tied placements, or different episode conventions require a separate mapping review.

## Invented game inputs

Six fake players form two fixed tribes: Ember (Ada, Ben, Cora) and Tide (Drew, Eli, Faye). Their complete drafts are shuffled permutations generated once with Python `random.Random(43)` in fixture contestant order. They are not historical player drafts. All Wordle guesses are synthetic: Ember wins odd episodes, Tide wins even episodes, and Episode 13 ties. Each player ends with seven public bonus points.

Checked expected draft totals are stored per episode, not computed by the production scorer during the test. The rule used to derive them is `max(0, 19 - actual_place - abs(draft_rank - actual_place))` for each contestant whose exit/final position has become known. Final draft totals: Ada 88, Ben 83, Cora 72, Drew 86, Eli 78, Faye 87. No points-available estimate is used as an oracle.

The test creates its instance, roster, activity, drafts, outcomes and Wordle writes through HTTP/Hurl. Instance-admin identity, tribes, memberships and the explicit historical schedule use test fixture helpers because corresponding legacy setup APIs are absent. The injected clock is test-only. Each run starts fresh; it does not resume a previous database or Discord run.
