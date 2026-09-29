# Original draft fixtures

These files contain the exact bodies of the players' Season 51 messages, including whitespace. They were retrieved using Probst's existing authenticated Discord client; temporary capture helpers are not part of the test suite.

| Fixture | Message | Posted |
| --- | --- | --- |
| `keeling-original-draft.txt` | `1554599095459651617` | `2026-09-29T21:01:47.412Z` |
| `mooney-original-draft.txt` | `1554628735611969549` | `2026-09-29T22:59:34.175Z` |

Both are from thread `1552521406137372833`; Discord reported no edit timestamp for either message.

The same bytes are copied to `apps/castaway-web/internal/httpapi/testdata/` for offline HTTP/database integration tests. Tests use `Thien An Nguyen` without the artificial quoted `An` nickname. They do not contact Discord or need operator credentials. Mooney's manually corrected live draft is not used as a fixture.
