---
name: castaway-pronouns
description: Use when writing Probst/Castaway Discord message copy (scores posts, recaps, announcements, `probst message`) that refers to a player in the third person. Pronouns are for Probst message copy only — never display them to anyone but the player themself.
---

# Castaway pronouns: Probst message copy only

Players have a stored `pronouns` value: `he/him`, `she/her`, or `they/them` (unset = unknown).

**Where it may be used:** only in text Probst posts to Discord (scores posts, recaps, announcements, one-off messages). The only other place pronouns appear is the player's own **Me** page, where they can change them; never show one player's pronouns to another (profiles, leaderboards, tables, player-facing logs, or anywhere else).

**How to read them:** `probst player list --instance INSTANCE --json` (admin-only) → each participant's `pronouns`. Pronouns belong to the person: the same player has the same value in every season, and players may change theirs at any time, so always read the current value.

**Rules:**
1. Look the player up every time; never infer pronouns from a name, nickname, or past copy.
2. Use exactly the stored pronoun. `they/them` takes plural verbs ("they are", "they've").
3. If unset, write around it with the player's name or a mention instead of guessing.
4. Never print a player's pronouns in the message itself (no "Marv (she/her)").
5. Copy still goes through the `castaway-announcements` approval flow before posting.
