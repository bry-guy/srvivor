---
name: castaway-db-access
description: Use whenever you're about to query or write the Castaway production database directly (psql, a temporary Go test with a live DATABASE_URL, pgx scripts) instead of going through the API or Probst. Ask Bryan first whether to build or extend an API for that query.
---

# Castaway: ask before reading the database directly

Direct database access bypasses auth, admin checks, and the server's own logic, and the next person can't repeat it.

Before any direct query against production (or BrainLand), stop and ask Bryan:

> "I need <what> and no API exposes it. Should we build/extend an API for this (e.g. `<proposed endpoint or Probst command>`), or do you want a one-off read-only query?"

- Propose the concrete endpoint or Probst command, and which existing loader it would reuse.
- Default to building the API: if the data is needed once, it's usually needed again (example: live Press the Button standings before close, which should be an admin endpoint).
- If Bryan approves a one-off read, use only the 1Password credential (never Kubernetes secrets), a `READ ONLY` transaction, and remove any temporary files afterwards. Say what was read.
- Writes always go through the API or Probst, never direct SQL.
