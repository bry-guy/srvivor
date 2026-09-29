# Original-message fixture

`keeling-original-draft.txt` is a byte-for-byte copy of `apps/probst/testdata/keeling-original-draft.txt`. That directory's README records provenance: Discord message `1554599095459651617`, posted `2026-09-29T21:01:47.412Z` in thread `1552521406137372833`, with no edit timestamp.

The integration test posts this body through the watched-thread endpoint using fake Discord identities and a disposable Season 51 roster/database. It verifies every saved rank, second-submission credit, and replay idempotency, without an `An` nickname or live Discord access.
