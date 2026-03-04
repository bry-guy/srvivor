# Archived Documentation

## Overview
The `doc/` directory contained design documents explaining the core scoring mechanics of the srvivor application. These documents have been archived here as the functionality is now implemented and the design is stable.

## Documents Summary

### design-scoring.md
- **Purpose**: Fundamental explanation of the Survivor draft scoring system
- **Key Concepts**:
  - Position-adjusted scoring where higher draft positions have higher value ceilings
  - Survivor Position Distance = |draft_position - final_position|
  - Score = position_value - distance (minimum 0)
  - Favors accurate predictions of top performers
- **Examples**: 3-survivor season calculations and mid-season scoring logic
- **Status**: Core scoring implemented in `internal/scorer/scorer.go`

### design-scoring-points-available.md
- **Purpose**: Detailed specification for "Points Available" calculation
- **Key Concepts**:
  - Shows additional points possible from remaining survivors
  - Formula: additional_points = position_value - best_distance (no 0-floor)
  - Conservative upper bound on final score potential
  - Decreases over time as positions are locked in
- **Examples**: Week 4 analysis with 8-survivor draft
- **Status**: Implemented in `calculatePointsAvailable` function in scorer.go

## Archive Rationale
These design documents were moved to wiki for historical reference as:
- The scoring system is fully implemented and tested
- Design decisions are documented in code comments
- Wiki provides better long-term knowledge management
- Reduces clutter in main codebase

## Related Files
- Implementation: `internal/scorer/scorer.go`
- Tests: `internal/scorer/scorer_test.go`
- Current state: See `wiki/20251016T150000_current_app_state.md`