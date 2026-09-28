---
name: castaway-announcements
description: Use before any Probst/Castaway Discord post — `probst message`, `announcement save|send|edit|schedule`, `buff --yes`, or any bot/API write that puts text in Discord. Requires the user to approve final copy first.
---

# Castaway announcements: finalize copy first

Before persisting or posting any Discord text for Castaway (Podracing or BrainLand):

1. Show the user the **exact final text** (with mentions, emoji, links), the destination (instance alias / channel), send time (America/New_York), and whether `--notify` pings.
2. Wait for explicit approval of that exact copy. Any edit → show the full revised text again and re-ask.
3. Only then run the command with `--yes`. Dry runs (no `--yes`) are fine anytime.
4. After sending/saving, report the name, status, and message ID (`probst announcement list`).

Never test in Podracing; BrainLand only. Never invent dates or results in copy.
