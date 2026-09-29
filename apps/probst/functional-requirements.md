# Functional requirements

- Inspect delegated service identity, instances, players, drafts, and public scores.
- Bind/unbind Discord channels and link/unlink player identities with explicit IDs.
- Bootstrap the first instance admin only through the API's configured bootstrap identity.
- Require `--yes` for channel replacement, unbinding, and unlinking.
- Support readable output and `--json`; propagate HTTP failures as nonzero exits.
- Load operator settings from an optional owner-only local JSON file, with flags taking precedence over environment values and environment values over the file.
- Create legacy instances with contestants, list contestants, and add players with optional Discord links.
- Import drafts from a Discord thread or text file: ignore chat, keep each author's latest draft before a cutoff, report unlinked authors, missing drafts, and unmatched or ambiguous names, and write only complete drafts when confirmed with `--yes`.
- Infer a single unmatched whole name word only in a complete, valid, duplicate-free draft with one unused contestant; expose the inferred match and keep full-roster fuzzy thresholds unchanged.
- Report weak candidate suggestions as requiring confirmation; neither suggestions nor ambiguous matches can be submitted with `--yes` without a corrected draft.
- Defer generic gameplay CRUD.
- Run the weekly loop from the season schedule: set tribes per episode, record immunity/reward winners, and open, import (from a Discord thread), hand-enter, and resolve the Wordle; every write previews unless `--yes`.
- Print the weekly scores post (`recap`): spoilered boots, biggest gainer/loser since the last recap, and the leaderboard.
