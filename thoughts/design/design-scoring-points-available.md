# Points Available Calculation

## Overview

Points Available represents **how many additional points a draft can still earn** during the remainder of a season. This calculation becomes increasingly important for mid-season analysis as it shows both the upside potential and guaranteed losses for a draft.

## Definition

Points Available is the sum of additional points possible from all remaining survivors in a draft, calculated by finding each survivor's optimal remaining position and computing their potential point contribution.

**Key Formula**: For each remaining survivor, `additional_points = position_value - best_distance`

Where:
- `position_value = total_survivors + 1 - draft_position` (inverse position value)
- `best_distance = min(abs(draft_position - remaining_position))` for all available positions

## Algorithm Components

The calculation has two distinct components:

1. **Current Score**: Fixed points already earned from eliminated survivors (uses standard scoring with 0-point floor)
2. **Points Available**: Additional points possible from remaining survivors (can be negative, no floor applied)

### Key Principle

Unlike the scoring system which floors individual survivor scores at 0, Points Available shows the **actual additional impact** each remaining survivor can have on the final score. This includes guaranteed point losses when a survivor cannot possibly reach a position that would yield positive points.

### Calculation Steps

1. Calculate current actual score from eliminated survivors using standard scoring rules
2. Identify all remaining survivors and available final positions
3. For each remaining survivor, find their best possible remaining position (minimizes distance)
4. Calculate additional points possible (position value - best distance) **without applying 0-point floor**
5. Sum all additional points (including negative values)

## Example: Week 4 Analysis

### Draft
```
1. Tom
2. Dick  
3. Harry
4. Cosmo
5. Elaine
6. Larry
7. Moe
8. Curly
```

### Week 4 Eliminations
```
5. Larry
6. Dick
7. Harry  
8. Moe
```

### Current Score Calculation
Using standard scoring rules (with 0-point floor):
- Larry: +3 value - 1 distance = +2 points
- Dick: +7 value - 4 distance = +3 points  
- Harry: +6 value - 4 distance = +2 points
- Moe: +2 value - 1 distance = +1 point

**Current Score: +8 points**

### Points Available Calculation

Remaining survivors: Tom, Cosmo, Elaine, Curly  
Remaining positions: 1, 2, 3, 4

For each remaining survivor, we find their optimal remaining position:
- Tom (draft pos 1): Best remaining position is 1st → +8 value - 0 distance = +8 points
- Cosmo (draft pos 4): Best remaining position is 4th → +5 value - 0 distance = +5 points  
- Elaine (draft pos 5): Best remaining position is 4th → +4 value - 1 distance = +3 points
- Curly (draft pos 8): Best remaining position is 4th → +1 value - 4 distance = **-3 points**

**Points Available: +13 points**

**Total Possible Final Score: 8 + 13 = 21 points**

## Behavioral Properties

- **Decreasing over time**: Points Available decreases as more positions are locked in
- **Approaches zero**: As the season nears completion, Points Available approaches 0
- **Shows guaranteed losses**: Negative contributions reveal survivors that will definitely lose points
- **Conservative estimate**: Provides an upper bound on final score potential
