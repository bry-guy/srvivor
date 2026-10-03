---
name: castaway-scores-flavor
description: Use when `probst scores generate` asks for the flavor lines of a weekly Castaway scores post. Returns JSON only; Probst renders numbers, mentions, standings and links itself.
---

# Castaway scores post: flavor lines

`probst scores generate` sends a JSON brief: season, week, a `facts` object, and this week's template lines (`fallback`) as a style reference. Reply with **only** a JSON object:

```json
{"intro": "...", "leader": "...", "gainer": "...", "slider": "..."}
```

## Voice
- Jeff Probst hosting Tribal Council: warm, punchy, a little theatrical. One sentence per line, two at most.
- Praise the leader. Hype the biggest gainer. Gently rib the biggest slider: playful, never mean, nothing about the person beyond their score.
- Vary wording from the fallback lines; don't repeat a fallback verbatim.
- Survivor flavor is welcome (torches, idols, blindsides, the tribe has spoken).
- Never say "pony." For in-game tribe scoring, say "your Tribe."

## Hard rules (Probst rejects a line that breaks them and uses the template line instead)
- `leader` must contain every name in `facts.leader_names` and the number `facts.leader_total`.
- `gainer` must contain every name in `facts.gainer_names` and the number `facts.gain`.
- `slider` must contain every name in `facts.slider_names` and the number `facts.slide`.
- If `facts.nobody_moved` is true, return `""` for `gainer` and `slider`.
- Use names exactly as given. No @mentions, links, emoji, standings, or other numbers. Probst adds those.
- Refer to players by name; no pronouns needed.
- Don't invent events, eliminations or results.

The output is a draft for the host to edit. Saving or scheduling still goes through `castaway-announcements` approval.
