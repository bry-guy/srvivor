---
type: feature
priority: medium
created: 2025-01-11T12:00:00Z
status: reviewed
tags: [data-structure, migration, testing]
keywords: [drafts, finals, final.txt, season, migration, bash script, test fixtures, Drafter: Final]
patterns: [directory structure, file migration, data reorganization, bash scripting]
---

# FEATURE-001: Restructure finals as final.txt drafts with season -1 test examples

## Description
Reorganize the data structure so that "finals" are treated as just another draft named "final.txt" within each season's drafts directory. Create test examples for a fake season "-1" using existing design documentation. This simplifies the codebase by eliminating the separate finals/ directory structure.

## Context
Currently, drafts are stored as individual files per user per season in drafts/[season]/[user].txt, while finals are stored separately in finals/[season].txt. This dual structure complicates code that needs to work with both types of data. The change will make finals consistent with the draft structure, making development easier.

## Requirements
Consolidate finals into the drafts directory structure as final.txt files, create test fixtures for season "-1", and update related code and documentation.

### Functional Requirements
- Move all finals/[season].txt files to drafts/[season]/final.txt
- Update Go code to reference the new final.txt location
- Create test fixtures for season "-1" with multiple user drafts and final.txt
- Auto-create empty final.txt (1-18 positions) if missing for a season
- Create idempotent bash migration script
- Update documentation files
- Restructure test fixtures to match new structure

### Non-Functional Requirements
- No changes to CLI user experience
- Migration script outputs to stdio
- Leave old finals/ directory after migration (no backward compatibility needed)
- No special error handling requirements

## Current State
- Drafts: drafts/[season]/[user].txt files with user predictions
- Finals: finals/[season].txt files with actual season results
- Drafter metadata uses "Final" for finals
- Test fixtures have separate drafts/ and finals/ directories

## Desired State
- All drafts including finals: drafts/[season]/[name].txt
- Finals named "final.txt" with Drafter: "Final"
- Season "-1" exists with test examples from design docs
- No finals/ directory
- Test fixtures restructured
- Code updated to work with new structure

## Research Context
Information specifically for research agents to understand the codebase and plan implementation.

### Keywords to Search
- drafts - Directory structure and file handling
- finals - Current finals implementation and references
- final.txt - New naming convention
- season - Season handling logic
- migration - Data migration patterns
- bash script - Scripting examples in codebase
- test fixtures - Test data organization
- Drafter: Final - Metadata handling for finals

### Patterns to Investigate
- directory structure - How directories are organized and traversed
- file migration - Patterns for moving/renaming files
- data reorganization - How data structures are changed
- bash scripting - Existing scripts in the codebase
- metadata handling - How Drafter field is processed
- test fixture organization - How test data is structured

### Key Decisions Made
- No backward compatibility - finals/ directory can be left but ignored
- Drafter name "Final" preserved for contract compatibility
- Auto-create empty final.txt with 1-18 positions if missing
- Migration via idempotent bash script
- Use design docs in thoughts/design/ for season "-1" examples
- No validation checks or special error handling
- Restructure test fixtures to match production structure

## Success Criteria
How to verify the ticket is complete and working correctly.

### Automated Verification
- [ ] Go tests pass with new structure
- [ ] Migration script runs without errors
- [ ] CLI commands work unchanged for users
- [ ] Test fixtures load correctly

### Manual Verification
- [ ] finals/ directory no longer exists or is ignored
- [ ] Each season has drafts/[season]/final.txt
- [ ] Season "-1" has multiple user drafts and final.txt
- [ ] Drafter: Final metadata preserved
- [ ] Documentation updated
- [ ] Test fixtures restructured

## Related Information
- Existing design docs in thoughts/design/ contain examples for season "-1"
- Current finals use Drafter: Final metadata
- Migration should handle all existing seasons (44-49)

## Notes
- Migration script should be idempotent (safe to run multiple times)
- Empty final.txt should have positions 1-18 with empty player names
- No changes to user-facing CLI behavior
- Old finals/ directory can remain but should be ignored by code