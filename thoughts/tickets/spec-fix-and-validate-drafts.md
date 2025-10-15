# Specification: Draft Validation and Formatting Enhancement

## Overview

Enhance the srvivor application to accept poorly formatted drafts with approximate or nickname-based contestant names, automatically validate and correct them against a canonical season roster using intelligent name matching.

## Goals

1. Accept drafts with imprecise contestant names and normalize them to canonical names defined in `rosters/`
2. Intelligently match input names against contestant data (first name, last name, nickname, or variations)
5. Provide clear feedback on name corrections made during validation

## Current State

### Draft File Format
The existing draft file format must be preserved:
```
Drafter: [Name]
Date: [YYYY-MM-DD]
Season: [Number]
---
[Position]. [Contestant Name]
[Position]. [Contestant Name]
...
```

### Directory Structure (Current)
```
drafts/
  [season]/
    [drafter].txt
    final.txt
```

### Current Behavior
- Draft files must contain exact contestant names
- Final rankings are stored separately in `drafts/seasons/final.txt` "draft"
- Scoring compares drafts against finals using exact string matching

### Identified Issues in Season 49 Drafts

Looking at the attached season 49 drafts, several name variations exist:

| Canonical Name | Variations Found |
|---------------|------------------|
| Sophie S      | Sophie (amanda.txt, bryan.txt) |
| Sophi B       | Sophi (amanda.txt, bryan.txt, grant.txt) |
| Kristen       | Kristina (bryan.txt, amanda.txt) |
| Michelle      | MC (amanda.txt, bryan.txt, grant.txt) |

These will serve as test cases for the name matching algorithm.

## Proposed Changes

### 1. Canonical Season Roster

#### Data Structure
Create a data structure to represent the canonical contestant roster:

````go
type Contestant struct {
    CanonicalName string // The official name used in drafts (e.g., "Sophie S")
    FirstName     string // First name (e.g., "Sophie")
    LastName      string // Last name (e.g., "Stevens")
    Nickname      string // Preferred nickname if any (e.g., "MC" for Michelle)
}

type SeasonRoster struct {
    Season      int
    Contestants []Contestant
}
````

#### Storage
Store season rosters in a new directory structure:
```
rosters/
  [season].json
```

Example `rosters/49.json`:
````json
{
  "season": 49,
  "contestants": [
    {
      "canonical_name": "Alex",
      "first_name": "Alex",
      "last_name": "",
      "nickname": ""
    },
    {
      "canonical_name": "Sophie S",
      "first_name": "Sophie",
      "last_name": "Stevens",
      "nickname": ""
    },
    {
      "canonical_name": "Sophi B",
      "first_name": "Sophi",
      "last_name": "Briggs",
      "nickname": ""
    },
    {
      "canonical_name": "Michelle",
      "first_name": "Michelle",
      "last_name": "Cox",
      "nickname": "MC"
    },
    {
      "canonical_name": "Kristen",
      "first_name": "Kristen",
      "last_name": "",
      "nickname": ""
    }
  ]
}
````

**Note**: Empty strings indicate no last name or nickname is applicable.

### 2. Name Matching Algorithm

#### Matching Strategy

The algorithm should intelligently match input names by considering:
1. Exact match to canonical name
2. Match to first name only
3. Match to last name only
4. Match to nickname
5. Match to first + last name combination
6. Fuzzy match to any of the above with similarity scoring

#### Implementation Approach

````pseudocode
function matchContestant(inputName, roster):
    normalized_input = normalize(inputName)
    candidates = []
    
    for contestant in roster:
        score = calculateMatchScore(normalized_input, contestant)
        if score > 0:
            candidates.append({contestant, score})
    
    if candidates is empty:
        return null, NO_MATCH
    
    // Sort by score descending
    sort(candidates by score, descending)
    
    best = candidates[0]
    
    // Require minimum confidence threshold
    if best.score < MINIMUM_THRESHOLD:
        return null, NO_MATCH
    
    // Determine match type for reporting
    matchType = determineMatchType(normalized_input, best.contestant, best.score)
    
    return best.contestant, matchType

function calculateMatchScore(input, contestant):
    max_score = 0.0
    
    // Exact match to canonical name (highest priority)
    if input == normalize(contestant.canonical_name):
        return 1.0
    
    // Exact match to nickname
    if contestant.nickname != "" and input == normalize(contestant.nickname):
        return 0.95
    
    // Exact match to first name
    if input == normalize(contestant.first_name):
        max_score = max(max_score, 0.85)
    
    // Exact match to last name
    if contestant.last_name != "" and input == normalize(contestant.last_name):
        max_score = max(max_score, 0.85)
    
    // Match to "FirstName LastName" or "LastName FirstName"
    if contestant.last_name != "":
        full_name = normalize(contestant.first_name + " " + contestant.last_name)
        reverse_name = normalize(contestant.last_name + " " + contestant.first_name)
        if input == full_name or input == reverse_name:
            max_score = max(max_score, 0.90)
    
    // Fuzzy matching with Levenshtein distance
    canonical_similarity = fuzzyMatch(input, normalize(contestant.canonical_name))
    max_score = max(max_score, canonical_similarity * 0.8)
    
    if contestant.nickname != "":
        nickname_similarity = fuzzyMatch(input, normalize(contestant.nickname))
        max_score = max(max_score, nickname_similarity * 0.75)
    
    first_similarity = fuzzyMatch(input, normalize(contestant.first_name))
    max_score = max(max_score, first_similarity * 0.7)
    
    if contestant.last_name != "":
        last_similarity = fuzzyMatch(input, normalize(contestant.last_name))
        max_score = max(max_score, last_similarity * 0.7)
    
    return max_score

function fuzzyMatch(s1, s2):
    // Use Levenshtein distance to calculate similarity
    distance = levenshtein(s1, s2)
    max_len = max(len(s1), len(s2))
    if max_len == 0:
        return 1.0
    return 1.0 - (distance / max_len)

function normalize(name):
    return lowercase(trim(remove_extra_spaces(name)))

function determineMatchType(input, contestant, score):
    if score == 1.0:
        return "exact match"
    if score >= 0.95:
        return "nickname match"
    if score >= 0.85:
        return "name component match"
    if score >= 0.7:
        return "fuzzy match"
    return "low confidence match"

MINIMUM_THRESHOLD = 0.70
````

#### Special Cases for Season 49

Based on the provided drafts, here's how the algorithm should handle known variations:

| Input      | Should Match To | Match Type              |
|------------|----------------|-------------------------|
| Sophie     | Sophie S       | First name match (0.85) |
| Sophi      | Sophi B        | First name match (0.85) |
| MC         | Michelle       | Nickname match (0.95)   |
| Kristina   | Kristen        | Fuzzy match (~0.85)     |

When multiple contestants share a first name (Sophie S vs Sophi B), the algorithm needs disambiguation logic:
- If input "Sophie" matches both "Sophie S" (exact first name) and "Sophi B" (fuzzy first name), prefer the exact match
- If input "Sophi" matches "Sophi B" (exact) and "Sophie S" (fuzzy), prefer the exact match

### 3. New Command: `drafts`

#### Purpose
Validate and fix draft files by normalizing contestant names against the canonical roster.

#### Usage
````bash
srvivor drafts -s [season] [-d drafter1,drafter2]
srvivor drafts fix -s [season] [-d drafter1,drafter2] [--dry-run] [--threshold float]
srvivor drafts validate -s [season] [-d drafter1,drafter2]

srvivor drafts -s 49 -d "*"  # Show all drafts for season 49
srvivor drafts fix -s 49 -d "*"  # Fix all drafts for season 49
srvivor drafts validate -s 49 -d "*"  # Validate all drafts for season 49

srvivor drafts -s 49 -d bryan.txt  # Show specific file
srvivor drafts fix -s 49 -d bryan.txt  # Fix specific file
srvivor drafts validate -s 49 -d bryan.txt  # Validate specific file

srvivor drafts fix -s 49 -d amanda,bryan --dry-run  # Preview changes without writing
srvivor drafts fix -s 49 -d grant --threshold 0.80  # Use custom threshold
````

#### Flags
- `-s, --season` (required): Season number
- `-d, --drafters`: Comma-separated drafter names or "*" for all 
- `--dry-run`: Preview changes without modifying files
- `--threshold`: Minimum confidence threshold for fuzzy matching (default: 0.70)

#### Behavior
1. Load canonical roster for specified season from `rosters/[season].json`
2. Read draft file(s)
3. For each contestant name in draft:
   - Attempt to match against roster using intelligent matching
   - If match found above threshold, replace with canonical name
   - Track correction made with confidence score
4. If not dry-run, write corrected draft back to file
5. Display summary of corrections

#### Output Format
````
Fixing draft: drafts/49/amanda.txt
  Line 6: "MC" -> "Michelle" (nickname match, confidence: 0.95)
  Line 10: "Sophie" -> "Sophie S" (name component match, confidence: 0.85)
  Line 20: "Kristina" -> "Kristen" (fuzzy match, confidence: 0.85)

Summary: 3 corrections made, 15 names unchanged
Draft saved to: drafts/49/amanda.txt
````

For dry-run mode:
````
[DRY RUN] Previewing changes for: drafts/49/amanda.txt
  Line 6: "MC" -> "Michelle" (nickname match, confidence: 0.95)
  Line 10: "Sophie" -> "Sophie S" (name component match, confidence: 0.85)
  Line 20: "Kristina" -> "Kristen" (fuzzy match, confidence: 0.85)

Summary: 3 corrections would be made, 15 names unchanged
[DRY RUN] No changes written to file
````

#### Error Handling
- If roster file not found, exit with error
- If no match found (below threshold), report as error and list unmatched name
- Continue processing other names even if one fails
- Provide summary of failures at end

Example error output:
````
Fixing draft: drafts/49/kate.txt
  Line 7: "Sophi" -> "Sophi B" (name component match, confidence: 0.85)
  ERROR Line 15: "Unknown Person" - No match found (highest confidence: 0.45 for "Jason")

Summary: 1 correction made, 16 names unchanged, 1 error
Draft partially saved to: drafts/49/kate.txt
````

### 5. Updated `score` Command

#### Changes
- Look for final rankings at `./drafts/[season]/final.txt`
- No change to command-line interface

#### Implementation
````go
// filepath: cmd/score.go
// ...existing code...
func runScore(cmd *cobra.Command, args []string) {
	// ...existing flag parsing...

	// Try new location first
	finalFilepath := fmt.Sprintf("./drafts/%d/final.txt", season)
	if _, err := os.Stat(finalFilepath); os.IsNotExist(err) {
		// Fall back to old location with warning
		finalFilepath = fmt.Sprintf("./finals/%d.txt", season)
		if _, err := os.Stat(finalFilepath); err == nil {
			slog.Error("Final rankings file not found", 
				"new_location", fmt.Sprintf("./drafts/%d/final.txt", season)
			os.Exit(1)
		}
	}

	final, err := scorer.ProcessFile(finalFilepath)
	// ...existing code...
}
````

### 6. Validation During Scoring

#### Optional Strict Mode
Add optional validation during scoring:

````bash
srvivor score -s 49 -d "*" --validate
````

#### Flags
- `--validate`: Validate all contestant names against roster before scoring

#### Behavior
When `--validate` flag is present:
1. Load canonical roster for season
2. For each draft (including final):
   - Validate all contestant names exist in roster with exact match
   - Collect validation errors
3. If any validation errors found:
   - Display all errors
   - Suggest running `fix-drafts` command
   - Exit with error code 1
4. If all valid, proceed with scoring

#### Output Format
````
Validating drafts for season 49...
  drafts/49/amanda.txt: 
    Line 6: "MC" is not an exact match for any contestant
    Line 10: "Sophie" is not an exact match for any contestant
  drafts/49/bryan.txt:
    Line 14: "Kristina" is not an exact match for any contestant

Validation failed: 3 names do not exactly match roster
Suggestion: Run 'srvivor fix-drafts -s 49 -d "*"' to automatically correct names
````

## Implementation Plan

### Phase 1: Core Infrastructure (Week 1)
1. Create `internal/roster` package with data structures
2. Implement roster JSON loading and parsing
3. Add JSON schema validation
4. Create roster files for seasons 48, 49 (based on finals and provided drafts)
5. Add unit tests for roster loading

**Deliverables**:
- `internal/roster/roster.go` with `Contestant` and `SeasonRoster` types
- `internal/roster/loader.go` with roster loading logic
- `rosters/48.json` and `rosters/49.json`
- Unit tests for roster package

### Phase 2: Name Matching (Week 2)
1. Create `internal/matcher` package
2. Implement name normalization utilities
3. Implement matching algorithm with all match types
4. Add Levenshtein distance library (`github.com/agnivade/levenshtein`)
5. Write comprehensive tests using season -1 data

**Deliverables**:
- `internal/matcher/matcher.go` with intelligent matching logic
- `internal/matcher/normalize.go` with normalization utilities
- Unit tests with season -1 test cases (Sophie, Sophi, MC, Kristina)
- Benchmark tests for performance

### Phase 3: Fix Command (Week 3)
1. Create `cmd/fix_drafts.go` with command structure
2. Implement draft file reading with line tracking
3. Implement name replacement logic preserving format
4. Implement correction tracking and reporting
5. Add dry-run and threshold flags
6. Add unit and integration tests

**Deliverables**:
- `cmd/fix_drafts.go` with full command implementation
- Draft file rewriting logic that preserves format
- Comprehensive output formatting
- E2E tests using season -1 drafts

### Phase 5: Integration and Validation (Week 5)
1. Add `--validate` flag to `score` command
2. Implement validation logic
3. Add integration tests for validation
4. Update documentation (README, command help)
5. Performance testing and optimization

**Deliverables**:
- Validation feature in score command
- Complete documentation update
- Performance benchmarks
- Final integration tests

## Testing Strategy

### Unit Tests

#### Roster Package
````go
func TestLoadRoster(t *testing.T)
func TestLoadRoster_InvalidFile(t *testing.T)
func TestLoadRoster_InvalidJSON(t *testing.T)
````

#### Matcher Package
````go
func TestNormalize(t *testing.T)
func TestMatchContestant_ExactMatch(t *testing.T)
func TestMatchContestant_NicknameMatch(t *testing.T)
func TestMatchContestant_FirstNameMatch(t *testing.T)
func TestMatchContestant_LastNameMatch(t *testing.T)
func TestMatchContestant_FuzzyMatch(t *testing.T)
func TestMatchContestant_NoMatch(t *testing.T)
func TestMatchContestant_Season49Cases(t *testing.T) // Sophie, Sophi, MC, Kristina
func TestCalculateMatchScore(t *testing.T)
func BenchmarkMatchContestant(b *testing.B)
````

### Integration Tests

#### Fix Drafts Command
````go
func TestFixDrafts_Season49_AllDrafts(t *testing.T)
func TestFixDrafts_Season49_Amanda(t *testing.T)
func TestFixDrafts_Season49_Bryan(t *testing.T)
func TestFixDrafts_DryRun(t *testing.T)
func TestFixDrafts_CustomThreshold(t *testing.T)
func TestFixDrafts_NoMatchesFound(t *testing.T)
func TestFixDrafts_RosterNotFound(t *testing.T)
````

#### Score Command with Validation
````go
func TestScore_WithValidation_ValidNames(t *testing.T)
func TestScore_WithValidation_InvalidNames(t *testing.T)
func TestScore_NewFinalLocation(t *testing.T)
func TestScore_BackwardCompatibility(t *testing.T)
````

### E2E Tests

Add to `e2e_test.go`:

````go
// filepath: e2e_test.go
// ...existing code...

func TestE2E_FixDrafts(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		expected []string
		exitCode int
	}{
		{
			name:     "fix all season -1 drafts",
			args:     []string{"fix-drafts", "-s", "49", "-d", "*"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"corrections made", "Amanda:", "Bryan:", "Grant:", "Kate:"},
			exitCode: 0,
		},
		{
			name:     "fix single draft with dry-run",
			args:     []string{"fix-drafts", "-s", "49", "-d", "amanda", "--dry-run"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"[DRY RUN]", "MC", "Michelle", "No changes written"},
			exitCode: 0,
		},
		{
			name:     "fix with custom threshold",
			args:     []string{"fix-drafts", "-s", "49", "-d", "bryan", "--threshold", "0.80"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"corrections made"},
			exitCode: 0,
		},
		{
			name:     "fix without roster file",
			args:     []string{"fix-drafts", "-s", "99", "-d", "test"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"roster file not found"},
			exitCode: 1,
		},
	}
	// Test implementation similar to existing TestE2E_Score
}

func TestE2E_ScoreWithValidation(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		expected []string
		exitCode int
	}{
		{
			name:     "score with validation passing",
			args:     []string{"score", "-s", "49", "-d", "bryan", "--validate"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"Validation passed", "Bryan:"},
			exitCode: 0,
		},
		{
			name:     "score with validation failing",
			args:     []string{"score", "-s", "49", "-d", "amanda", "--validate"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"Validation failed", "MC", "is not an exact match"},
			exitCode: 1,
		},
	}
	// Test implementation similar to existing TestE2E_Score
}
````

### Test Fixtures

Create test fixtures based on season -1:

```
test_fixtures/
  drafts/
    -1/
      amanda.txt  # With MC, Sophie, Kristina variations
      bryan.txt   # With Sophi, MC, Kristina variations
      grant.txt   # With Sophi, MC variations
      kate.txt    # With Sophi, Sophie variations (already correct)
      final.txt   # Copy of finals/48.txt with updated metadata
  rosters/
    -1.json       # Complete season 49 roster
```

## Configuration

Add configuration options in `internal/config`:

````go
// filepath: internal/config/config.go
// ...existing code...

type Config struct {
	// ...existing fields...
	
	// Validation settings
	FuzzyMatchThreshold float64 `env:"SRVVR_FUZZY_THRESHOLD" envDefault:"0.70"`
	RequireExactMatch   bool    `env:"SRVVR_REQUIRE_EXACT" envDefault:"false"`
}
````

## Documentation Updates

### README.md
Add new sections:
1. **Commands** - Document `drafts` command with examples
2. **Roster Management** - How to create and maintain roster files
3. **Name Matching** - Explain intelligent matching algorithm

### New Documentation Files
1. `docs/roster-format.md` - JSON schema and examples
2. `docs/name-matching.md` - Detailed explanation of matching algorithm

### Command Help Text
Update help text for all commands to reflect new features and options.
