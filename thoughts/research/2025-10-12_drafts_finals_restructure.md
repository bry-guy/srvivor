---
date: 2025-10-12T06:07:02Z
git_commit: 405f73c
branch: main
repository: git@github.com:bry-guy/srvivor.git
topic: "Restructure finals as final.txt drafts with season -1 test examples"
tags: [research, codebase, data-structure, migration, testing]
last_updated: 2025-10-12T06:07:02Z
---

## Ticket Synopsis
Reorganize the data structure so that "finals" are treated as just another draft named "final.txt" within each season's drafts directory. Create test examples for a fake season "-1" using existing design documentation. This simplifies the codebase by eliminating the separate finals/ directory structure.

## Summary
The codebase currently maintains separate `drafts/` and `finals/` directories, with finals stored as individual files per season. The research reveals this structure is hardcoded in the scoring command and tests. However, existing plans and specifications already detail this exact restructuring, including migration scripts and backward compatibility. The design documentation contains suitable examples for creating season "-1" test fixtures.

## Detailed Findings

### Current Directory Structure
- **Drafts**: `drafts/[season]/[drafter].txt` - Individual user prediction files
- **Finals**: `finals/[season].txt` - Final season results with `Drafter: Final` metadata
- **Test Fixtures**: Mirror production structure with `test_fixtures/drafts/` and `test_fixtures/finals/`

### Code Dependencies
- **Score Command**: `cmd/score.go:111` hardcodes `finalFilepath := fmt.Sprintf("./finals/%d.txt", season)`
- **Scoring Logic**: `internal/scorer/scorer.go` compares draft entries against final results
- **Test Suite**: `internal/scorer/scorer_test.go` uses `test_fixtures/finals/0.txt` and `test_fixtures/drafts/0.txt`

### Implementation Approach
The restructuring requires:
- Migration of existing finals files to new location
- Updates to scoring command to use new finals location
- Creation of season "-1" test fixtures
- Bash script for idempotent migration

### Design Documentation for Test Data
- `thoughts/design/design-scoring-points-available.md` contains Week 4 analysis examples
- `thoughts/design/design-scoring.md` has scoring algorithm examples
- These can be adapted into season "-1" draft files with name variations

### Bash Scripting Patterns
- `script/checkhealth.sh` - Simple health check with colored output functions
- `script/dev.sh` - Daemon management script
- Existing patterns show bash scripting conventions for the codebase

## Code References
- `cmd/score.go:111` - Hardcoded finals path construction
- `internal/scorer/scorer.go:43-89` - Main scoring function comparing drafts vs finals
- `internal/scorer/scorer_test.go:130-141` - Test fixture usage

## Architecture Insights
The current dual-directory structure creates unnecessary complexity in the scoring system. Consolidating finals as `final.txt` drafts simplifies the data model by treating finals as just another type of draft with special metadata. The existing plans show this was already identified as a needed architectural improvement.

## Historical Context (from thoughts/)
No prior research documents found for this specific restructuring topic.

## Related Research
- No prior research documents found for this specific topic
- Related work exists in the plans and tickets directories

## Open Questions
- What specific examples from the design docs should be used for season "-1" drafts?
- How should the auto-creation of empty final.txt files be implemented?
