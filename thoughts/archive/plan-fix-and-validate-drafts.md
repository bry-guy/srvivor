# Technical Plan: Draft Validation and Formatting Enhancement

## Overview

Enhance the srvivor application to accept poorly formatted drafts with approximate or nickname-based contestant names, automatically validate and correct them against a canonical season roster using intelligent name matching, and unify the draft storage model by eliminating the separate `finals/` directory
structure.

## Objectives

1. Accept drafts with imprecise contestant names and normalize them to canonical names
2. Intelligently match input names using fuzzy matching (Levenshtein distance)
3. Consolidate finals into `drafts/{season}/final.txt` structure
4. Maintain backward compatibility with existing draft format
5. Provide clear feedback on name corrections during validation

---

## Phase 1: Foundation - Roster System & Data Structures

### Ticket 1.1: Create Roster Package and Data Structures
**Priority**: Critical
**Estimated Effort**: 2-4 hours

**Description**:
Create the `internal/roster` package with data structures for managing season rosters and contestant information.

**Acceptance Criteria**:
- [ ] Create `internal/roster/roster.go` file
- [ ] Define `Contestant` struct with fields: `CanonicalName`, `FirstName`, `LastName`, `Nickname`
- [ ] Define `SeasonRoster` struct with fields: `Season`, `Contestants []Contestant`
- [ ] All fields exported (uppercase) for JSON marshaling
- [ ] Add struct tags for JSON serialization (`json:"canonical_name"`)
- [ ] Follow existing code style from `internal/scorer/scorer.go`

**Implementation Notes**:
```go
type Contestant struct {
    CanonicalName string `json:"canonical_name"`
    FirstName     string `json:"first_name"`
    LastName      string `json:"last_name"`
    Nickname      string `json:"nickname"`
}

type SeasonRoster struct {
    Season      int          `json:"season"`
    Contestants []Contestant `json:"contestants"`
}
```

**References**:
- Spec: Lines 61-75, 85-124
- Pattern: `internal/scorer/scorer.go` lines 12-26

---

### Ticket 1.2: Implement Roster JSON Loading
**Priority**: Critical
**Estimated Effort**: 3-5 hours

**Description**:
Implement roster file loading and parsing from JSON with validation and error handling.

**Acceptance Criteria**:
- [ ] Create `LoadRoster(season int) (*SeasonRoster, error)` function
- [ ] Build file path: `rosters/{season}.json`
- [ ] Use `os.Open()` + `json.NewDecoder()` for parsing
- [ ] Validate loaded data (non-empty contestants, valid season number)
- [ ] Return descriptive errors for file not found, invalid JSON, validation failures
- [ ] Use `slog` for debug/info logging
- [ ] Follow error handling pattern from `scorer.ProcessFile()` (line 310)

**Implementation Pattern**:
```go
func LoadRoster(season int) (*SeasonRoster, error) {
    filepath := fmt.Sprintf("./rosters/%d.json", season)
    file, err := os.Open(filepath)
    if err != nil {
        return nil, fmt.Errorf("roster file not found: %w", err)
    }
    defer file.Close()

    var roster SeasonRoster
    if err := json.NewDecoder(file).Decode(&roster); err != nil {
        return nil, fmt.Errorf("invalid roster JSON: %w", err)
    }

    // Validation...
    return &roster, nil
}
```

**References**:
- Spec: Lines 77-124
- Pattern: `internal/scorer/scorer.go` lines 310-319 (ProcessFile)

---

### Ticket 1.3: Create Season 49 Roster JSON File
**Priority**: Critical
**Estimated Effort**: 1-2 hours

**Description**:
Create the canonical roster JSON file for Season 49 based on existing finals and draft variations.

**Acceptance Criteria**:
- [ ] Create `rosters/` directory if not exists
- [ ] Create `rosters/49.json` file
- [ ] Include all contestants from `finals/49.txt`
- [ ] Add proper metadata for known variations:
  - Sophie S (FirstName: Sophie, LastName: Stevens)
  - Sophi B (FirstName: Sophi, LastName: Briggs)
  - Michelle (FirstName: Michelle, LastName: Cox, Nickname: MC)
  - Kristen (FirstName: Kristen)
- [ ] Empty strings for missing last names/nicknames
- [ ] Valid JSON syntax
- [ ] Season field matches filename (49)

**References**:
- Spec: Lines 85-124 (example format)
- Spec: Lines 43-54 (Season 49 variations table)
- Data: `finals/49.txt`, `drafts/49/*.txt`

---

### Ticket 1.4: Unit Tests for Roster Package
**Priority**: High
**Estimated Effort**: 2-3 hours

**Description**:
Comprehensive unit tests for roster loading and validation.

**Acceptance Criteria**:
- [ ] `TestLoadRoster_ValidFile` - Successfully loads Season 49 roster
- [ ] `TestLoadRoster_FileNotFound` - Returns error for missing season
- [ ] `TestLoadRoster_InvalidJSON` - Returns error for malformed JSON
- [ ] `TestLoadRoster_EmptyContestants` - Validates non-empty contestants
- [ ] Use `testify/assert` for assertions (existing pattern)
- [ ] Create test fixture: `test_fixtures/rosters/0.json`
- [ ] All tests pass with `go test ./internal/roster`

**References**:
- Pattern: `internal/scorer/scorer_test.go` (test organization)
- Pattern: `e2e_test.go` lines 13-20 (testify/assert usage)

---

## Phase 2: Name Matching Algorithm

### Ticket 2.1: Add Levenshtein Distance Dependency
**Priority**: Critical
**Estimated Effort**: 15 minutes

**Description**:
Add the fuzzy matching library dependency to the project.

**Acceptance Criteria**:
- [ ] Run: `go get github.com/agnivade/levenshtein@v1.2.1`
- [ ] Verify `go.mod` includes dependency
- [ ] Run `go mod tidy` to clean up
- [ ] Run `go mod verify` to confirm integrity
- [ ] Library specified in spec at line 459

**References**:
- Spec: Line 459 (explicit library reference)
- Research: Go fuzzy matching libraries report

---

### Ticket 2.2: Create Matcher Package with Core Algorithm
**Priority**: Critical
**Estimated Effort**: 5-8 hours

**Description**:
Implement the intelligent name matching algorithm specified in the spec using Levenshtein distance.

**Acceptance Criteria**:
- [ ] Create `internal/matcher/matcher.go`
- [ ] Implement `normalize(name string) string` - lowercase + trim + collapse spaces
- [ ] Implement `fuzzyMatch(s1, s2 string) float64` - Levenshtein similarity (1.0 - distance/maxLen)
- [ ] Implement `calculateMatchScore(input string, contestant Contestant) float64`:
  - Exact canonical name: 1.0
  - Exact nickname: 0.95
  - Exact first name: 0.85
  - Exact last name: 0.85
  - Full name match (first + last): 0.90
  - Fuzzy canonical: similarity * 0.8
  - Fuzzy nickname: similarity * 0.75
  - Fuzzy first/last: similarity * 0.7
  - Return max score across all methods
- [ ] Implement `MatchContestant(input string, roster []Contestant, threshold float64) (*Contestant, float64, string, error)`:
  - Return: matched contestant, confidence score, match type, error
  - Match types: "exact match", "nickname match", "name component match", "fuzzy match"
  - Return error if no match above threshold
- [ ] Default threshold: 0.70
- [ ] Proper error handling for ambiguous matches (multiple candidates at same score)

**Implementation Algorithm**:
Follows spec pseudocode at lines 140-232.

**References**:
- Spec: Lines 126-232 (complete matching algorithm)
- Spec: Lines 234-247 (Season 49 special cases)

---

### Ticket 2.3: Unit Tests for Matcher - Season 49 Cases
**Priority**: Critical
**Estimated Effort**: 3-4 hours

**Description**:
Comprehensive unit tests validating the matching algorithm against Season 49 variations.

**Acceptance Criteria**:
- [ ] `TestNormalize` - Validates trimming, lowercasing, space handling
- [ ] `TestFuzzyMatch` - Validates Levenshtein similarity calculation
- [ ] `TestMatchContestant_ExactMatch` - Canonical name exact match returns 1.0
- [ ] `TestMatchContestant_NicknameMatch` - "MC" → Michelle (0.95)
- [ ] `TestMatchContestant_FirstNameMatch` - "Sophie" → Sophie S (0.85)
- [ ] `TestMatchContestant_Season49Cases` - Table-driven test:
  - "Sophie" → Sophie S (name component match, ≥0.85)
  - "Sophi" → Sophi B (name component match, ≥0.85)
  - "MC" → Michelle (nickname match, ≥0.95)
  - "Kristina" → Kristen (fuzzy match, ≥0.70)
- [ ] `TestMatchContestant_NoMatch` - Returns error for input below threshold
- [ ] `TestMatchContestant_Disambiguation` - Ensures "Sophie" prefers exact "Sophie" over fuzzy "Sophi"
- [ ] Use Season 49 roster from Ticket 1.3
- [ ] All tests use `testify/assert`

**References**:
- Spec: Lines 234-247 (Season 49 test cases table)
- Pattern: `internal/scorer/scorer_test.go` (test structure)

---

### Ticket 2.4: Matcher Benchmark Tests
**Priority**: Medium
**Estimated Effort**: 1-2 hours

**Description**:
Performance benchmarks to ensure matching is fast enough for interactive use.

**Acceptance Criteria**:
- [ ] `BenchmarkMatchContestant` - Single name match against 20-contestant roster
- [ ] `BenchmarkBatchMatching` - 18 names against 20-contestant roster (full draft validation)
- [ ] Target: <10ms for batch matching (18 picks * 20 contestants = 360 comparisons)
- [ ] Run: `go test -bench=. -benchmem ./internal/matcher`
- [ ] Document results in benchmark output

**References**:
- Research: Levenshtein library performance (350ns/comparison)

---

## Phase 3: Draft Validation Command

### Ticket 3.1: Export Entry Fields in Scorer Package
**Priority**: Critical (Blocker for 3.2)
**Estimated Effort**: 30 minutes

**Description**:
Modify the `Entry` struct to export fields so validation tools can access position and player name.

**Acceptance Criteria**:
- [ ] Change `position` to `Position` in `internal/scorer/scorer.go` line 24
- [ ] Change `playerName` to `PlayerName` in line 25
- [ ] Update all references in `scorer.go` (lines 289-299 in readDraft, etc.)
- [ ] Run all existing tests to verify no breakage: `make test`
- [ ] All 11 existing tests pass

**Implementation**:
```go
// Before:
type Entry struct {
    position   int
    playerName string
}

// After:
type Entry struct {
    Position   int
    PlayerName string
}
```

**References**:
- Current: `internal/scorer/scorer.go` lines 23-26
- Rationale: Research report notes private fields block validation tools

---

### Ticket 3.2: Create Draft Validation Core Logic
**Priority**: Critical
**Estimated Effort**: 4-6 hours

**Description**:
Implement the core logic for validating and correcting draft files against rosters.

**Acceptance Criteria**:
- [ ] Create `internal/validator/validator.go`
- [ ] Define `Correction` struct:
  - `LineNumber int`
  - `OriginalName string`
  - `CorrectedName string`
  - `MatchType string`
  - `Confidence float64`
- [ ] Define `ValidationResult` struct:
  - `Corrections []Correction`
  - `Unchanged int`
  - `Errors []ValidationError`
- [ ] Implement `ValidateDraft(draft *scorer.Draft, roster *roster.SeasonRoster, threshold float64) ValidationResult`:
  - Iterate through `draft.Entries`
  - Call `matcher.MatchContestant()` for each entry
  - Build corrections list with line numbers (header = 4 lines, then position-based)
  - Track unchanged count (exact matches)
  - Collect errors for names below threshold
  - Return comprehensive result
- [ ] Pure function with no side effects (doesn't modify draft or write files)

**Implementation Pattern**:
```go
type ValidationError struct {
    LineNumber    int
    OriginalName  string
    BestCandidate string
    BestScore     float64
}

func ValidateDraft(draft *scorer.Draft, roster *roster.SeasonRoster, threshold float64) ValidationResult {
    var result ValidationResult
    lineNum := 5 // Start after "Drafter:\nDate:\nSeason:\n---\n"

    for _, entry := range draft.Entries {
        contestant, score, matchType, err := matcher.MatchContestant(
            entry.PlayerName, roster.Contestants, threshold)

        if err != nil {
            // Handle no match case
        } else if entry.PlayerName != contestant.CanonicalName {
            // Add correction
        } else {
            result.Unchanged++
        }
        lineNum++
    }

    return result
}
```

**References**:
- Spec: Lines 278-288 (behavior description)

---

### Ticket 3.3: Create Drafts Command Structure (Cobra)
**Priority**: Critical
**Estimated Effort**: 3-4 hours

**Description**:
Create the `drafts` command with subcommands for validation and fixing drafts.

**Acceptance Criteria**:
- [ ] Create `cmd/drafts.go`
- [ ] Implement `newDraftsCmd()` factory function returning `*cobra.Command`
- [ ] Command structure:
  - `srvivor drafts validate -s [season] -d [drafters]` - Read-only validation
  - `srvivor drafts fix -s [season] -d [drafters] [--dry-run] [--threshold float]` - Fix with write
- [ ] Flags:
  - `-s, --season` (int, required): Season number
  - `-d, --drafters` (string slice, required): Drafter names or "*"
  - `--dry-run` (bool): Preview changes without writing
  - `--threshold` (float64, default 0.70): Minimum confidence threshold
- [ ] Register command in `cmd/root.go` init() function
- [ ] Follow existing pattern from `cmd/score.go` (factory + runFunc)

**Implementation Pattern**:
```go
func newDraftsCmd() *cobra.Command {
    draftsCmd := &cobra.Command{
        Use:   "drafts [validate|fix]",
        Short: "Validate and fix draft files against canonical rosters",
    }

    validateCmd := &cobra.Command{
        Use:   "validate -s [season] -d [drafters]",
        Short: "Validate draft names without modifying files",
        Run:   runDraftsValidate,
    }

    fixCmd := &cobra.Command{
        Use:   "fix -s [season] -d [drafters]",
        Short: "Fix draft names by normalizing to canonical roster",
        Run:   runDraftsFix,
    }

    // Add flags to both subcommands
    for _, cmd := range []*cobra.Command{validateCmd, fixCmd} {
        cmd.Flags().IntP("season", "s", 0, "Season number")
        cmd.MarkFlagRequired("season")
        cmd.Flags().StringSliceP("drafters", "d", []string{}, "Drafter names or '*'")
        cmd.MarkFlagRequired("drafters")
    }

    fixCmd.Flags().Bool("dry-run", false, "Preview changes without writing")
    fixCmd.Flags().Float64("threshold", 0.70, "Minimum confidence threshold")

    draftsCmd.AddCommand(validateCmd, fixCmd)
    return draftsCmd
}
```

**References**:
- Spec: Lines 249-277 (command usage)
- Pattern: `cmd/score.go` lines 16-32, `cmd/root.go` lines 13-33

---

### Ticket 3.4: Implement Drafts Validate Command
**Priority**: High
**Estimated Effort**: 3-4 hours

**Description**:
Implement the read-only validation command that reports issues without modifying files.

**Acceptance Criteria**:
- [ ] Implement `runDraftsValidate(cmd *cobra.Command, args []string)` in `cmd/drafts.go`
- [ ] Parse flags: season, drafters
- [ ] Support wildcard: "-d *" expands to all drafts (use `filepath.Glob` pattern from score command)
- [ ] Load roster: `roster.LoadRoster(season)`
- [ ] For each drafter:
  - Load draft: `scorer.ProcessFile(filepath)`
  - Validate: `validator.ValidateDraft(draft, roster, 0.70)`
  - Print validation report
- [ ] Output format:
  ```
  Validating: drafts/49/amanda.txt
    ✓ 15 names valid
    ⚠ 3 names need correction:
      Line 6: "MC" → "Michelle" (nickname match, 0.95)
      Line 10: "Sophie" → "Sophie S" (name component match, 0.85)
    ✗ 1 name has no match:
      Line 15: "UnknownPlayer" (best: "Jason", 0.45)
  ```
- [ ] Exit code 0 if all drafts valid, exit code 1 if any errors
- [ ] Use `slog.Info` for progress, `fmt.Printf` for report output

**References**:
- Spec: Lines 255-270 (usage examples)
- Spec: Lines 310-325 (error handling)
- Pattern: `cmd/score.go` lines 38-128 (flag parsing, file loading, output)

---

### Ticket 3.5: Implement Drafts Fix Command
**Priority**: Critical
**Estimated Effort**: 5-7 hours

**Description**:
Implement the fix command that rewrites draft files with corrected contestant names.

**Acceptance Criteria**:
- [ ] Implement `runDraftsFix(cmd *cobra.Command, args []string)` in `cmd/drafts.go`
- [ ] Parse flags: season, drafters, dry-run, threshold
- [ ] Support wildcard: "-d *"
- [ ] Load roster
- [ ] For each drafter:
  - Load draft
  - Validate draft with custom threshold
  - If corrections needed AND not dry-run:
    - Rewrite file preserving format (metadata + separator + corrected entries)
    - Use same line endings as original
  - Print correction report
- [ ] Output format (normal mode):
  ```
  Fixing draft: drafts/49/amanda.txt
    Line 6: "MC" → "Michelle" (nickname match, confidence: 0.95)
    Line 10: "Sophie" → "Sophie S" (name component match, confidence: 0.85)

  Summary: 2 corrections made, 16 names unchanged
  Draft saved to: drafts/49/amanda.txt
  ```
- [ ] Output format (dry-run mode):
  ```
  [DRY RUN] Previewing changes for: drafts/49/amanda.txt
    Line 6: "MC" → "Michelle" (nickname match, confidence: 0.95)

  Summary: 1 correction would be made, 17 names unchanged
  [DRY RUN] No changes written to file
  ```
- [ ] If errors (no match), print error and continue to next file
- [ ] Exit code 0 if all successful, 1 if any errors

**File Rewriting Logic**:
```go
func rewriteDraft(draft *scorer.Draft, corrections []validator.Correction, filepath string) error {
    var buf bytes.Buffer

    // Write metadata
    buf.WriteString(fmt.Sprintf("Drafter: %s\n", draft.Metadata.Drafter))
    buf.WriteString(fmt.Sprintf("Date: %s\n", draft.Metadata.Date))
    buf.WriteString(fmt.Sprintf("Season: %s\n", draft.Metadata.Season))
    buf.WriteString("---\n")

    // Write entries with corrections applied
    correctionMap := buildCorrectionMap(corrections)
    for _, entry := range draft.Entries {
        name := entry.PlayerName
        if correction, ok := correctionMap[entry.Position]; ok {
            name = correction.CorrectedName
        }
        buf.WriteString(fmt.Sprintf("%d. %s\n", entry.Position, name))
    }

    return os.WriteFile(filepath, buf.Bytes(), 0644)
}
```

**References**:
- Spec: Lines 278-308 (behavior and output format)
- Pattern: `internal/scorer/scorer.go` lines 248-307 (file format understanding)

---

### Ticket 3.6: Integration Tests for Drafts Command
**Priority**: High
**Estimated Effort**: 4-5 hours

**Description**:
Comprehensive integration tests for the drafts command covering validation and fixing.

**Acceptance Criteria**:
- [ ] Create test fixtures:
  - `test_fixtures/drafts/-1/amanda.txt` (with MC, Sophie, Kristina)
  - `test_fixtures/drafts/-1/bryan.txt` (with Sophi, MC, Kristina)
  - `test_fixtures/rosters/-1.json` (Season 49 roster copy)
- [ ] `TestDraftsValidate_AllValid` - Validates correctly formatted draft
- [ ] `TestDraftsValidate_WithErrors` - Reports corrections needed
- [ ] `TestDraftsFix_DryRun` - Previews without writing
- [ ] `TestDraftsFix_ApplyCorrections` - Writes corrected file
- [ ] `TestDraftsFix_CustomThreshold` - Uses custom threshold
- [ ] `TestDraftsFix_PreservesFormat` - Metadata and structure intact after fix
- [ ] `TestDraftsWildcard` - "-d *" processes all drafts
- [ ] All tests use testify/assert
- [ ] Tests create temp copies to avoid modifying fixtures

**References**:
- Pattern: `internal/scorer/scorer_test.go` (integration test style)
- Pattern: `e2e_test.go` (table-driven tests)

---

## Phase 4: Directory Structure Migration

### Ticket 4.1: Update Score Command for New Finals Location
**Priority**: High
**Estimated Effort**: 2-3 hours

**Description**:
Modify the score command to look for finals in the new location with backward compatibility fallback.

**Acceptance Criteria**:
- [ ] Modify `runScore()` in `cmd/score.go` (line 111+)
- [ ] Try new location first: `./drafts/{season}/final.txt`
- [ ] If not found, fall back to old location: `./finals/{season}.txt`
- [ ] If using old location, log deprecation warning:
  ```
  slog.Warn("Using deprecated finals/ directory. Consider migrating to drafts/{season}/final.txt")
  ```
- [ ] If neither found, error with both paths shown
- [ ] No changes to command-line interface
- [ ] Existing tests continue to pass

**Implementation**:
```go
// Try new location first
finalFilepath := fmt.Sprintf("./drafts/%d/final.txt", season)
if _, err := os.Stat(finalFilepath); os.IsNotExist(err) {
    // Fall back to old location
    finalFilepath = fmt.Sprintf("./finals/%d.txt", season)
    if _, err := os.Stat(finalFilepath); err == nil {
        slog.Warn("Using deprecated finals/ directory structure",
            "new_location", fmt.Sprintf("./drafts/%d/final.txt", season))
    } else {
        slog.Error("Final rankings not found in either location",
            "new_location", fmt.Sprintf("./drafts/%d/final.txt", season),
            "old_location", fmt.Sprintf("./finals/%d.txt", season))
        os.Exit(1)
    }
}
```

**References**:
- Spec: Lines 369-401 (score command changes)
- Current: `cmd/score.go` lines 111-116

---

### Ticket 4.2: Create Test Fixtures with New Structure
**Priority**: High
**Estimated Effort**: 1-2 hours

**Description**:
Update test fixtures to use new directory structure for testing.

**Acceptance Criteria**:
- [ ] Create `test_fixtures/drafts/-1/` directory
- [ ] Copy Season 49 drafts with name variations to season -1
- [ ] Create `test_fixtures/drafts/-1/final.txt` (copy from `finals/48.txt`, update metadata)
- [ ] Update metadata: Season: -1, Date: appropriate test date
- [ ] Verify existing test fixtures remain unchanged (for backward compat testing)
- [ ] Create `test_fixtures/rosters/-1.json` (Season 49 roster)

**References**:
- Spec: Lines 336-368 (directory structure)
- Spec: Lines 355-368 (migration checklist)

---

### Ticket 4.3: E2E Tests for New Finals Location
**Priority**: High
**Estimated Effort**: 2-3 hours

**Description**:
Add E2E tests verifying the score command works with both old and new finals locations.

**Acceptance Criteria**:
- [ ] Add tests to `e2e_test.go`:
  - `TestE2E_Score_NewFinalsLocation` - Uses `drafts/{season}/final.txt`
  - `TestE2E_Score_OldFinalsLocation` - Uses `finals/{season}.txt`
  - `TestE2E_Score_BackwardCompatibility` - Verifies fallback works
- [ ] Tests verify score calculations are identical regardless of location
- [ ] Tests check for deprecation warning in stderr when using old location
- [ ] Use Season -1 test fixtures (isolated from production data)
- [ ] All tests pass with `go test -run "E2E"`

**References**:
- Pattern: `e2e_test.go` lines 13-129 (existing E2E test structure)
- Spec: Lines 369-401 (backward compatibility requirements)

---

### Ticket 4.4: Manual Migration for Existing Seasons
**Priority**: Medium
**Estimated Effort**: 30 minutes

**Description**:
Manually migrate existing finals files to new location (production data).

**Acceptance Criteria**:
- [ ] For each season (44, 45, 46, 47, 48, 49):
  - Copy `finals/{season}.txt` to `drafts/{season}/final.txt`
  - Verify file copied correctly (diff check)
- [ ] Keep original `finals/` directory (for backward compatibility)
- [ ] Run `make test && make e2e` to verify nothing breaks
- [ ] Document migration in commit message

**Commands**:
```bash
for season in 44 45 46 47 48 49; do
    cp "finals/${season}.txt" "drafts/${season}/final.txt"
done
```

**References**:
- Spec: Lines 343-368 (migration path)

---

## Phase 5: Validation Flag for Score Command

### Ticket 5.1: Add Validation Flag to Score Command
**Priority**: Medium
**Estimated Effort**: 3-4 hours

**Description**:
Add optional `--validate` flag to score command for strict roster validation before scoring.

**Acceptance Criteria**:
- [ ] Add flag to `cmd/score.go`:
  ```go
  scoreCmd.Flags().Bool("validate", false, "Validate draft names against roster before scoring")
  ```
- [ ] If flag set:
  - Load roster for season
  - Validate all drafts + final against roster
  - Require exact matches only (no fuzzy matching)
  - If any errors, print validation report and exit code 1
  - If all valid, proceed with normal scoring
- [ ] Output format:
  ```
  Validating drafts for season 49...
    drafts/49/amanda.txt:
      Line 6: "MC" is not an exact match (did you mean "Michelle"?)
    drafts/49/bryan.txt:
      Line 14: "Kristina" is not an exact match (did you mean "Kristen"?)

  Validation failed: 2 names do not exactly match roster
  Suggestion: Run 'srvivor drafts fix -s 49 -d "*"' to automatically correct
  ```
- [ ] No changes to default behavior (validation opt-in only)

**Implementation**:
```go
validate, _ := cmd.Flags().GetBool("validate")
if validate {
    roster, err := roster.LoadRoster(season)
    if err != nil {
        slog.Error("Failed to load roster for validation", "error", err)
        os.Exit(1)
    }

    // Validate all drafts + final
    allValid := true
    for _, draft := range append(drafts, final) {
        result := validator.ValidateDraft(draft, roster, 1.0) // Threshold 1.0 = exact only
        if len(result.Errors) > 0 {
            // Print errors
            allValid = false
        }
    }

    if !allValid {
        fmt.Printf("\nSuggestion: Run 'srvivor drafts fix -s %d -d \"*\"' to automatically correct\n", season)
        os.Exit(1)
    }
}
```

**References**:
- Spec: Lines 403-439 (validation during scoring)

---

### Ticket 5.2: Integration Tests for Score Validation
**Priority**: Medium
**Estimated Effort**: 2-3 hours

**Description**:
Integration tests for the score command validation flag.

**Acceptance Criteria**:
- [ ] `TestScoreValidation_AllValid` - Validation passes, scoring proceeds
- [ ] `TestScoreValidation_InvalidNames` - Validation fails, exits before scoring
- [ ] `TestScoreValidation_SuggestionMessage` - Error output includes fix command suggestion
- [ ] Tests use Season -1 fixtures with intentional errors
- [ ] Verify exit code 1 on validation failure
- [ ] Verify scoring does NOT run when validation fails

**References**:
- Pattern: `internal/scorer/scorer_test.go` (integration test structure)
- Spec: Lines 427-439 (validation output format)

---

## Phase 6: Documentation and Polish

### Ticket 6.1: Update README with Drafts Command
**Priority**: Medium
**Estimated Effort**: 1-2 hours

**Description**:
Update the main README with documentation for the new drafts command.

**Acceptance Criteria**:
- [ ] Add "Commands" section to README.md
- [ ] Document `drafts validate` with usage examples
- [ ] Document `drafts fix` with flags: `--dry-run`, `--threshold`
- [ ] Add "Roster Management" section explaining roster JSON format
- [ ] Add example roster JSON snippet
- [ ] Include Season 49 test cases as examples
- [ ] Update existing score command docs to mention new finals location

**References**:
- Spec: Lines 663-668 (documentation updates)

---

### Ticket 6.2: Create Roster Format Documentation
**Priority**: Low
**Estimated Effort**: 1 hour

**Description**:
Create detailed documentation for roster JSON format and guidelines.

**Acceptance Criteria**:
- [ ] Create `doc/roster-format.md`
- [ ] Document JSON schema with field descriptions
- [ ] Provide examples for different contestant types:
  - Single name (e.g., "Alex")
  - First + Last (e.g., "Sophie Stevens")
  - With nickname (e.g., Michelle "MC" Cox)
- [ ] Explain empty string convention for missing fields
- [ ] Include validation rules
- [ ] Link from main README

**References**:
- Spec: Lines 61-124 (roster structure)

---

### Ticket 6.3: Create Name Matching Algorithm Documentation
**Priority**: Low
**Estimated Effort**: 1-2 hours

**Description**:
Document the intelligent name matching algorithm for future reference.

**Acceptance Criteria**:
- [ ] Create `doc/name-matching.md`
- [ ] Explain matching strategy (exact → fuzzy)
- [ ] Document scoring system with examples
- [ ] Include Season 49 test cases with expected scores
- [ ] Explain Levenshtein distance calculation
- [ ] Document threshold tuning guidance
- [ ] Link from main README

**References**:
- Spec: Lines 126-232 (complete algorithm)
- Spec: Lines 234-247 (test cases)

---

### Ticket 6.4: Update Command Help Text
**Priority**: Low
**Estimated Effort**: 30 minutes

**Description**:
Update help text for all commands to reflect new features.

**Acceptance Criteria**:
- [ ] Update `score` command Long description to mention `--validate` flag
- [ ] Ensure `drafts` command has clear Short and Long descriptions
- [ ] Verify flag descriptions are clear and concise
- [ ] Test: `srvivor help` shows all commands
- [ ] Test: `srvivor help score` shows updated help
- [ ] Test: `srvivor help drafts` shows subcommands

**References**:
- Current: `cmd/score.go` lines 16-21
- Pattern: Cobra command help conventions

---

## Testing Summary

### Unit Tests
- **Roster Package**: Load, validate, error handling (Ticket 1.4)
- **Matcher Package**: Normalize, fuzzy match, Season 49 cases, benchmarks (Tickets 2.3, 2.4)
- **Validator Package**: Validation logic, correction tracking (Ticket 3.6)

### Integration Tests
- **Drafts Command**: Validate, fix, dry-run, wildcard, threshold (Ticket 3.6)
- **Score Command**: Validation flag, new finals location (Tickets 4.3, 5.2)

### E2E Tests
- **New Finals Location**: Both old and new paths, backward compat (Ticket 4.3)
- **Drafts Workflow**: Validate → Fix → Score pipeline (Ticket 3.6)

### Test Data
- **Season -1**: Non-destructive test environment (Ticket 4.2)
- **Test Fixtures**: Rosters, drafts with variations, finals (Tickets 1.4, 4.2)

---

## Implementation Order

**Critical Path** (Must complete in order):
1. Phase 1 (Foundation): Tickets 1.1 → 1.2 → 1.3 → 1.4
2. Phase 2 (Matching): Tickets 2.1 → 2.2 → 2.3
3. Phase 3 (Command): Tickets 3.1 → 3.2 → 3.3 → 3.4 → 3.5

**Parallel Workstreams** (Can work concurrently after Phase 3.5):
- Phase 4 (Migration): Tickets 4.1 → 4.2 → 4.3 → 4.4
- Phase 5 (Validation): Tickets 5.1 → 5.2
- Phase 6 (Docs): Tickets 6.1, 6.2, 6.3, 6.4 (anytime after Phase 3)

**Blockers**:
- Ticket 3.2 blocked by 3.1 (needs exported Entry fields)
- Ticket 3.4/3.5 blocked by 3.3 (needs command structure)
- All Phase 4/5/6 blocked by Phase 3.5 (needs working fix command)

---

## Risk Mitigation

### Risk: Breaking Existing Functionality
**Mitigation**:
- Use Season -1 for all testing (non-destructive)
- Maintain backward compatibility for finals location
- Run full test suite before migration (Ticket 4.4)
- Export Entry fields as first step (isolated change)

### Risk: Incorrect Name Matching
**Mitigation**:
- Comprehensive unit tests with Season 49 known cases
- Dry-run mode for preview before committing changes
- Configurable threshold for tuning
- Manual review of first season migration

### Risk: Performance Degradation
**Mitigation**:
- Benchmark tests targeting <10ms for batch validation
- Levenshtein library is pre-vetted (350ns/comparison)
- Lazy evaluation (exact match early exit)

### Risk: Data Loss During File Rewrite
**Mitigation**:
- Atomic writes using temp file + rename pattern
- Keep original finals/ directory during migration
- Backup recommendation in migration docs
- Extensive integration tests for file format preservation

---

## Definition of Done

A ticket is complete when:
- [ ] All acceptance criteria met
- [ ] Code follows existing style (gofmt, golangci-lint)
- [ ] Unit tests written and passing
- [ ] Integration/E2E tests passing
- [ ] Documentation updated (if applicable)
- [ ] Code reviewed (self-review minimum)
- [ ] No regressions in existing tests
- [ ] Changes tested manually with real data (Season 49)

---

## Estimated Total Effort

| Phase | Tickets | Estimated Hours |
|-------|---------|-----------------|
| Phase 1: Foundation | 4 | 8-14 hours |
| Phase 2: Matching | 4 | 11-17 hours |
| Phase 3: Command | 6 | 20-29 hours |
| Phase 4: Migration | 4 | 5-8 hours |
| Phase 5: Validation | 2 | 5-7 hours |
| Phase 6: Documentation | 4 | 3-5 hours |
| **Total** | **24 tickets** | **52-80 hours** |

**Recommendation**: Plan for ~2-3 weeks of focused development, or 4-6 weeks at part-time pace.

---

## Success Metrics

- [ ] All Season 49 name variations automatically corrected
- [ ] Scoring works with both old and new finals locations
- [ ] Validation catches all non-canonical names before scoring
- [ ] Dry-run mode provides accurate preview
- [ ] Custom threshold allows tuning for different seasons
- [ ] Zero data loss during migration
- [ ] All 24 tickets completed with tests passing
- [ ] Documentation complete and accurate

---

**INSTRUCTIONS**:
1. Create the directory `doc/plan/` if it doesn't exist
2. Write this complete plan to `doc/plan/plan-fix-and-validate-drafts.md`
3. Ensure proper markdown formatting
4. Return confirmation when complete with file path and line count

