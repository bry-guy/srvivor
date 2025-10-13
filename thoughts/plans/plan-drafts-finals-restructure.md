# Drafts Finals Restructure Implementation Plan

## Overview

Restructure finals as final.txt drafts within each season's drafts directory, create test fixtures for season "-1" using Week 4 design examples, and update the scoring system to work with the new structure while maintaining backward compatibility.

## Current State Analysis

- **Drafts**: Stored as `drafts/[season]/[drafter].txt` with individual user predictions
- **Finals**: Stored separately as `finals/[season].txt` with `Drafter: Final` metadata
- **Scoring**: Hardcoded to load finals from `./finals/{season}.txt` in `cmd/score.go:111`
- **Tests**: Use `test_fixtures/drafts/` and `test_fixtures/finals/` mirroring production structure
- **Design Docs**: Contain Week 4 analysis examples with names: Tom, Dick, Harry, Cosmo, Elaine, Larry, Moe, Curly

### Key Discoveries:
- Scoring logic treats drafts and finals identically except for validation rules
- Current structure creates unnecessary complexity in file path management
- Design documentation provides suitable test data for season "-1"
- Existing tests expect specific fixture locations that need updating

## Desired End State

After implementation:
- All finals stored as `drafts/[season]/final.txt` with `Drafter: Final`
- Season "-1" exists with multiple draft variations and final.txt using Week 4 examples
- Scoring command automatically looks for finals in new location first, falls back to old
- Auto-creation of empty final.txt (1-18 positions) when scoring seasons without finals
- Test fixtures restructured to match production
- Backward compatibility maintained during transition

### Key Decisions Made
- Use Week 4 design examples (Tom, Dick, Harry, Cosmo, Elaine, Larry, Moe, Curly) for season "-1"
- Auto-create empty final.txt only during scoring when missing, with warning message
- Preserve old finals/ directory during transition (manual deletion later)
- Maintain Drafter: Final metadata for contract compatibility
- No validation checks or special error handling beyond auto-creation

## What We're NOT Doing

- Removing old finals/ directory (preserved for backward compatibility)
- Changing CLI user experience or command syntax
- Adding validation or error handling beyond auto-creation
- Modifying scoring algorithm or draft format
- Creating roster system or name matching (separate future feature)

## Implementation Approach

Incremental migration with backward compatibility:
1. Create new directory structure and test data
2. Update scoring to prefer new location with fallback
3. Add auto-creation for missing finals
4. Update tests to use new structure
5. Provide migration documentation

## Phase 1: Data Migration

### Overview
Create the new directory structure, migrate existing finals to new location, and create season "-1" test fixtures using Week 4 design examples.

### Changes Required:

#### 1. Create Season -1 Test Fixtures
**File**: `drafts/-1/`
**Changes**: Create directory and draft files using Week 4 examples

```bash
# Create directory
mkdir -p drafts/-1

# Create final.txt with Week 4 eliminations
# Drafter: Final
# Date: 2023-01-01
# Season: -1
# ---
# 1. 
# 2. 
# 3. 
# 4. 
# 5. Larry
# 6. Dick
# 7. Harry
# 8. Moe

# Create multiple draft variations
# Draft 1: drafts/-1/bryan.txt (perfect order)
# Draft 2: drafts/-1/riley.txt (scrambled order)
# Draft 3: drafts/-1/katie.txt (different variations)
```

#### 2. Migrate Existing Finals
**File**: `drafts/[44-49]/final.txt`
**Changes**: Copy existing finals to new location

```bash
# For each season 44-49
cp finals/44.txt drafts/44/final.txt
cp finals/45.txt drafts/45/final.txt
# ... etc
```

### Success Criteria:

#### Automated Verification:
- [x] Season -1 directory exists with final.txt and multiple drafts
- [x] All existing finals copied to new location
- [x] File contents match originals (diff verification)

#### Manual Verification:
- [x] Season -1 drafts contain Week 4 contestant names with variations
- [x] Final.txt has correct eliminations: Larry(5), Dick(6), Harry(7), Moe(8)
- [x] All seasons 44-48 have final.txt in drafts directory

---

## Phase 2: Code Updates

### Overview
Update the scoring command to look for finals in the new location first, with fallback to old location for backward compatibility.

### Changes Required:

#### 1. Update Finals Path Logic
**File**: `cmd/score.go`
**Changes**: Modify finalFilepath construction to check new location first

```go
// Before (line 111):
finalFilepath := fmt.Sprintf("./finals/%d.txt", season)

// After:
finalFilepath := fmt.Sprintf("./drafts/%d/final.txt", season)
if _, err := os.Stat(finalFilepath); os.IsNotExist(err) {
    // Fallback to old location with warning
    oldPath := fmt.Sprintf("./finals/%d.txt", season)
    if _, err := os.Stat(oldPath); err == nil {
        slog.Warn("Using deprecated finals location", "old_path", oldPath, "new_path", finalFilepath)
        finalFilepath = oldPath
    } else {
        // Auto-create empty final.txt
        slog.Warn("No finals found, creating empty final.txt", "path", finalFilepath)
        createEmptyFinal(finalFilepath, season)
    }
}
```

#### 2. Add Empty Final Creation Function
**File**: `cmd/score.go`
**Changes**: Add helper function to create empty final.txt

```go
func createEmptyFinal(filepath string, season int) error {
    file, err := os.Create(filepath)
    if err != nil {
        return err
    }
    defer file.Close()

    // Write metadata
    fmt.Fprintf(file, "Drafter: Final\n")
    fmt.Fprintf(file, "Date: %s\n", time.Now().Format("2006-01-02"))
    fmt.Fprintf(file, "Season: %d\n", season)
    fmt.Fprintf(file, "---\n")

    // Write 1-18 empty positions
    for i := 1; i <= 18; i++ {
        fmt.Fprintf(file, "%d. \n", i)
    }

    return nil
}
```

### Success Criteria:

#### Automated Verification:
- [x] Go tests pass with new finals location
- [x] Scoring works with season -1 test data
- [x] Backward compatibility maintained (old finals still work)

#### Manual Verification:
- [x] Scoring season -1 produces expected results from Week 4 analysis
- [x] Scoring existing seasons still works unchanged
- [x] Auto-creation warning appears when finals missing
- [x] Empty final.txt created with 1-18 positions

---

## Phase 3: Auto-Creation

### Overview
Implement automatic creation of empty final.txt files when scoring seasons that don't have finals, with appropriate warning messages.

### Changes Required:

#### 1. Integrate Auto-Creation Logic
**File**: `cmd/score.go`
**Changes**: Add auto-creation when finals not found in either location

```go
finalFilepath := fmt.Sprintf("./drafts/%d/final.txt", season)
if _, err := os.Stat(finalFilepath); os.IsNotExist(err) {
    oldPath := fmt.Sprintf("./finals/%d.txt", season)
    if _, err := os.Stat(oldPath); err == nil {
        slog.Warn("Using deprecated finals location", "old_path", oldPath)
        finalFilepath = oldPath
    } else {
        // Auto-create empty final
        slog.Warn("No finals found for season, creating empty final.txt", "season", season, "path", finalFilepath)
        if err := createEmptyFinal(finalFilepath, season); err != nil {
            slog.Error("Failed to create empty final", "error", err)
            os.Exit(1)
        }
    }
}
```

### Success Criteria:

#### Automated Verification:
- [x] Tests pass when scoring seasons without finals
- [x] Empty final.txt created with correct format

#### Manual Verification:
- [x] Warning message appears in output when auto-creating finals
- [x] Created file has 1-18 positions with empty player names
- [x] Subsequent scoring of same season uses created file

---

## Phase 4: Test Updates

### Overview
Update test fixtures and test code to match the new directory structure.

### Changes Required:

#### 1. Update Test Fixtures
**File**: `test_fixtures/`
**Changes**: Restructure to match production

```bash
# Create new structure
mkdir -p test_fixtures/drafts/0
mv test_fixtures/drafts/0.txt test_fixtures/drafts/0/final.txt
mv test_fixtures/drafts/1.txt test_fixtures/drafts/0/bryan.txt

# Update metadata in final.txt to Season: 0
```

#### 2. Update Test Paths
**File**: `internal/scorer/scorer_test.go`
**Changes**: Update fixture paths to new structure

```go
// Before:
"../../test_fixtures/drafts/0.txt"
"../../test_fixtures/finals/0.txt"

// After:
"../../test_fixtures/drafts/0/bryan.txt"
"../../test_fixtures/drafts/0/final.txt"
```

#### 3. Update E2E Tests
**File**: `e2e_test.go`
**Changes**: Update test cases to use new paths where applicable

### Success Criteria:

#### Automated Verification:
- [x] All unit tests pass with updated fixture paths
- [x] E2E tests pass with new structure
- [x] Test scoring produces same results as before

#### Manual Verification:
- [x] Test fixtures match production structure
- [x] E2E tests work with season -1 data
- [x] No regressions in existing test behavior

---

## Phase 5: Documentation

### Overview
Update documentation and create migration script for the restructuring.

### Changes Required:

#### 1. Create Migration Script
**File**: `script/migrate-finals.sh`
**Changes**: Idempotent bash script to migrate finals

```bash
#!/bin/bash
# Migrate finals to new structure

for season in {44..49}; do
    old_path="finals/${season}.txt"
    new_path="drafts/${season}/final.txt"
    
    if [ -f "$old_path" ] && [ ! -f "$new_path" ]; then
        mkdir -p "drafts/${season}"
        cp "$old_path" "$new_path"
        echo "Migrated finals/${season}.txt to drafts/${season}/final.txt"
    fi
done

echo "Migration complete. Old finals/ directory preserved."
```

#### 2. Update README
**File**: `README.md`
**Changes**: Document new finals location and migration

#### 3. Update Plan Documentation
**File**: `doc/plan/plan-drafts-finals-restructure.md`
**Changes**: Update with implementation details

### Success Criteria:

#### Automated Verification:
- [x] Migration script runs without errors
- [x] Script is idempotent (safe to run multiple times)

#### Manual Verification:
- [x] README documents new finals location
- [x] Migration script successfully moves all finals
- [x] Documentation is accurate and complete

## Testing Strategy

### Unit Tests:
- Verify new finals path logic in score command
- Test auto-creation of empty finals
- Validate season -1 scoring produces expected Week 4 results

### Integration Tests:
- End-to-end scoring with new finals location
- Backward compatibility with old finals location
- Auto-creation behavior when finals missing

### Manual Testing Steps:
1. [x] Score season -1 and verify Week 4 analysis results
2. [x] Score existing season and verify unchanged behavior
3. [x] Score non-existent season and verify auto-creation
4. [x] Run migration script and verify finals moved correctly

## Performance Considerations

- File path checking adds minimal overhead (stat calls)
- Auto-creation only triggers for missing finals (rare case)
- No impact on scoring algorithm performance

## Migration Notes

- Old finals/ directory preserved during transition
- Migration script is idempotent and safe to run multiple times
- Auto-creation provides graceful handling for incomplete seasons
- Backward compatibility ensures existing workflows continue working

## References

- Original ticket: `thoughts/tickets/feature_drafts_finals_restructure.md`
- Research: `thoughts/research/2025-10-12_drafts_finals_restructure.md`
- Design docs: `thoughts/design/design-scoring-points-available.md`
- Current implementation: `cmd/score.go:111`, `internal/scorer/scorer.go`