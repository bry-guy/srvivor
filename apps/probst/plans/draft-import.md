# Draft import and Season 51 setup

Status: in-progress (BrainLand file and thread imports verified; Discord command confirmation and Podracing mapping pending)

## Goal

Players post drafts as free text in a Discord thread that also contains chat. The admin loads
them into castaway-web through Probst, reviewing every parse before anything is written.
Season setup (instance, contestants, players, channel binding) is done through the API via Probst.

## Existing pieces (no new API)

- `POST /instances` (legacy mode) with `contestants`; `GET /instances/:id/contestants`.
- `POST /instances/:id/participants`, `PUT .../participants/:id/discord-link`.
- `POST /instances/:id/admins/bootstrap` (needs `BOOTSTRAP_ADMIN_DISCORD_USER_ID`).
- `PUT /discord/guilds/:g/channels/:c` binding (instance admin).
- `PUT /instances/:id/drafts/:participant` — ordered `contestant_ids`, rejects anything that is
  not every contestant exactly once; replace semantics make re-runs safe.

## Changes

1. Config: set `BOOTSTRAP_ADMIN_DISCORD_USER_ID=235246238382030849` in the home-k3s web ConfigMap
   so `probst instance bootstrap-admin` works for legacy instances.
2. Probst setup commands:
   - `probst instance create --name N --season S --contestants-file F` (one name per line)
   - `probst contestant list`
   - `probst player add NAME [--discord-user ID]` (create + optional link)
3. Draft parser (package in Probst, no external deps):
   - Contestant aliases derived from the stored name `First "Nick" Last`: full, nickname,
     first (all but last word), last, first word.
   - Line cleanup: strip code fences, markdown, emoji, mentions, bracketed/parenthetical notes.
   - Line matching: best alias over the cleaned line and its leading 1–3 word windows; exact alias
     wins; otherwise Levenshtein similarity >= 0.75 and a clear margin over the runner-up, else
     the line is ambiguous or unmatched (never guessed).
   - A message is a draft candidate only if at least 15 lines match contestants; chat is ignored.
   - Order: numbered lines by number (must be 1..N unique), otherwise message order.
   - Verdict per draft: READY (all N once), or NEEDS REVIEW with reasons
     (missing, duplicate, unmatched, ambiguous, bad numbering).
4. `probst draft import --file F --participant ID` — dry run by default, `--yes` submits if READY.
5. `probst draft import THREAD_URL` — reads the thread with `CASTAWAY_DISCORD_BOT_TOKEN`:
   - all messages (paginated), latest candidate draft per author wins, optional `--before` cutoff;
     edits after the cutoff are flagged;
   - author mapped through the Discord link; unlinked authors are reported, not guessed;
   - review table per author (READY / NEEDS REVIEW / NO DRAFT), `--verbose` shows every line match;
   - `--yes` submits READY drafts only; unchanged drafts are skipped.

## Validation

- Unit tests: messy fixtures (chat, emoji, nicknames, misspellings, "An" vs "Ana", numbering gaps,
  split/partial drafts, code blocks), thread selection and cutoff logic with a fake Discord server.
- Live: create a BrainLand Season 51 test instance via Probst, bind BrainLand #general, import a
  messy file draft, verify with `/castaway draft`; then import from a BrainLand thread mixing chat
  and drafts, verify via `probst draft show` and `/castaway draft`.
- Podracing Season 51 instance is created the same way once the player list is confirmed.

## BrainLand verification

- Created legacy Season 51 instance `605699c6-1f21-4741-8e0f-4864d5e62018` with 21 contestants; bootstrapped admin `235246238382030849`, linked Brain, and bound BrainLand #general (`1078197143501819918`).
- Parsed the messy file draft and submitted it; a repeat import returned `UNCHANGED`.
- Read real thread `1552530452596789319`: one linked Brain draft, 21 unique exact matches, no ambiguous picks. Submitted the reviewed order, verified all 21 ordered contestant IDs from `probst draft show`, and confirmed a repeated import returned `UNCHANGED`.
- Genuine `/castaway draft` and `/castaway scores` interactions still need user confirmation; no Discord-command result is claimed here. Podracing setup awaits its confirmed player list, Discord IDs, and exact channel-to-instance mapping.

## Out of scope

Draft deadline enforcement in the API, player self-submission, LLM parsing, combining drafts split
across several messages (reported as NEEDS REVIEW; fall back to `--file`).
