# srvivor Monorepo

`srvivor` is a suite of tools for Survivor TV show fantasy drafts, starting with a CLI scoring app.

## Commands

### Score Command

Calculate and display the total score for Survivor drafts for a particular season.

```bash
srvivor score [-f --file [filepath] | -d --drafters [drafters]] -s --season [season] [--validate]
```

Options:
- `-f, --file`: Input file containing the draft
- `-d, --drafters`: Drafter name(s) to lookup the draft
- `-s, --season`: Season number of the Survivor game (required)
- `--validate`: Validate all contestant names against roster before scoring

Examples:
```bash
srvivor score -d bryan -s 44
srvivor score -d "*" -s 45
srvivor score -f ./drafts/44/bryan.txt -s 44
srvivor score -d bryan -s 49 --validate
```

### Fix Drafts Command

Fix draft files by normalizing contestant names against the canonical roster.

```bash
srvivor fix-drafts -s [season] -d [drafters] [--dry-run] [--threshold float]
```

Options:
- `-s, --season`: Season number (required)
- `-d, --drafters`: Drafter name(s) or "*" for all
- `--dry-run`: Preview changes without modifying files
- `--threshold`: Minimum confidence threshold for fuzzy matching (default: 0.70)

Examples:
```bash
srvivor fix-drafts -s 49 -d amanda
srvivor fix-drafts -s 49 -d "*" --dry-run
srvivor fix-drafts -s 49 -d bryan --threshold 0.80
```

## Roster Management

Season rosters are stored in `packages/shared/go/rosters/[season].json` and contain canonical contestant information for name matching.

### Roster Format

```json
{
  "season": 49,
  "contestants": [
    {
      "canonical_name": "Sophie S",
      "first_name": "Sophie",
      "last_name": "Stevens",
      "nickname": ""
    }
  ]
}
```

## Name Matching

The application intelligently matches input names against the canonical roster using:

1. Exact canonical name match
2. Nickname match
3. First/last name component matches
4. Fuzzy string similarity matching

## Develop

Pre-requisites:
1. Install Mise: https://mise.jdx.dev/
2. `mise install`: install required tools
3. `go work sync`: sync Go workspace dependencies

Your main developer loop is `mise run test && mise run build && mise run run`.

Test your changes via `mise run test`.
