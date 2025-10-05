# Technical Plan: Fix Points Available Implementation

## **SPECIFICATION SUMMARY**

### Overview
This plan addresses critical bugs in the points available calculation system within the Survivor scoring application. The current implementation contains architectural flaws that produce incorrect results, particularly evident in Week 4 test data where current calculations show 3/12 instead of the expected 8/13.

### Design Reference
This implementation follows the comprehensive design specified in `doc/design-scoring-points-available.md`, which outlines the separation of Current Score and Points Available calculations and the direct per-survivor algorithm approach.

### Key Goals
- Fix systematic calculation errors in points available logic
- Implement proper separation of Current Score and Points Available calculations
- Maintain backward compatibility with existing CLI output format
- Ensure all existing tests continue to pass while adding comprehensive coverage
- Preserve negative Points Available values to show guaranteed losses

### Expected Outcomes
- Week 4 example produces correct results: Score=8, Points=13, Total=21
- Robust, maintainable codebase with clear separation of concerns
- Comprehensive test coverage for all scoring scenarios
- Documentation that clearly explains the algorithm and implementation

## **CURRENT STATE ANALYSIS**

### Implementation Bugs Identified
1. **Mixed Logic Architecture**: Current Score and Points Available calculations are intertwined in a single function, making debugging and maintenance difficult
2. **Critical Floor Bug**: `max(0, pointsAvailable)` inappropriately removes negative values that should be preserved to show guaranteed losses
3. **Indirect Algorithm**: Current implementation uses an indirect approach instead of the optimal direct per-survivor calculation method
4. **Position Value Errors**: Systematic errors in position value and distance calculations produce incorrect results

### Test Foundation Status
- **Positive**: E2E tests have been fixed and all existing tests are passing
- **Positive**: Test infrastructure is stable and ready for additional coverage
- **Gap**: Missing comprehensive test coverage specifically for Points Available calculations
- **Gap**: No test cases covering edge scenarios (negative values, tied positions, etc.)

### Week 4 Example Discrepancy
- **Current Implementation**: Score=3, Points=12, Total=15
- **Expected Results**: Score=8, Points=13, Total=21
- **Impact**: 6-point discrepancy representing a 40% error in total score calculation

## **TECHNICAL TICKETS**

### **TICKET 1: Separate Current Score and Points Available Calculations**
- **Priority**: High
- **Type**: Architecture Refactor
- **Location**: Lines 40-131 in scorer.go
- **Estimated Effort**: 4-6 hours

**Problem Statement**:
Current implementation mixes Current Score and Points Available logic in a single function, making the code difficult to debug, test, and maintain. This architectural flaw contributes to calculation errors and makes it impossible to isolate issues.

**Solution**:
Extract separate, focused functions for each calculation type:
- `calculateCurrentScore()` - handles only current week scoring
- `calculatePointsAvailable()` - handles only remaining points calculation
- Maintain clear interfaces between functions

**Acceptance Criteria**:
1. Current Score calculation is isolated in its own function with clear inputs/outputs
2. Points Available calculation is isolated in its own function with clear inputs/outputs
3. All existing tests continue to pass after refactoring
4. Code coverage remains at current levels or improves
5. Function signatures are well-documented with clear parameter descriptions

### **TICKET 2: Fix Critical Points Available Floor Bug**
- **Priority**: Critical
- **Type**: Bug Fix
- **Location**: Line 128 in scorer.go
- **Estimated Effort**: 2-3 hours

**Problem Statement**:
The current implementation uses `max(0, pointsAvailable)` which incorrectly removes negative Points Available values. Negative values are semantically important as they indicate guaranteed losses and should be preserved for user visibility.

**Solution**:
Remove the floor operation from Points Available calculation while maintaining it for Current Score where it's appropriate. Negative Points Available values should flow through to the final output.

**Acceptance Criteria**:
1. Points Available calculation preserves negative values without flooring
2. Current Score calculation retains appropriate flooring where needed
3. Week 4 test case shows correct negative values where applicable
4. CLI output displays negative Points Available values correctly

### **TICKET 3: Implement Direct Per-Survivor Points Available Algorithm**
- **Priority**: High
- **Type**: Algorithm Rewrite
- **Location**: Lines 113-127 in scorer.go
- **Estimated Effort**: 6-8 hours

**Problem Statement**:
Current implementation uses an indirect calculation method that doesn't properly account for optimal position assignment for each survivor. This leads to systematic underestimation of available points.

**Solution**:
Implement the direct per-survivor algorithm as specified in the design document:
- For each remaining survivor, calculate optimal position assignment
- Sum individual optimal points to get total Points Available
- Account for position constraints and elimination order

**Acceptance Criteria**:
1. Algorithm directly calculates points for each remaining survivor
2. Optimal position assignment logic is correctly implemented
3. Week 4 example produces Points Available = 13
4. Performance remains acceptable for typical dataset sizes

### **TICKET 4: Fix Position Value and Distance Calculations**
- **Priority**: Medium
- **Type**: Bug Fix
- **Location**: Lines 72-73 in scorer.go
- **Estimated Effort**: 3-4 hours

**Problem Statement**:
Systematic errors in position value calculation formulas contribute to incorrect scoring results. The current implementation may have off-by-one errors or incorrect distance calculations.

**Solution**:
Verify and fix the mathematical formulas for:
- Position value calculation based on total contestants
- Distance calculation between predicted and actual positions
- Edge case handling for first/last positions

**Acceptance Criteria**:
1. Position value formulas match design specification exactly
2. Distance calculations produce expected results for known test cases
3. Edge cases (first position, last position) are handled correctly
4. All existing position-based tests continue to pass

### **TICKET 5: Add Comprehensive Test Coverage for New Algorithm**
- **Priority**: High
- **Type**: Test Enhancement
- **Location**: scorer_test.go
- **Estimated Effort**: 4-5 hours

**Problem Statement**:
Current test suite lacks specific coverage for Points Available calculations, making it difficult to verify correctness and catch regressions. Need comprehensive test cases covering normal operation and edge cases.

**Solution**:
Add comprehensive test coverage including:
- Week 4 example as a regression test
- Edge cases (negative values, tied positions, single survivor remaining)
- Boundary conditions (first week, final week)
- Integration tests for the complete scoring pipeline

**Acceptance Criteria**:
1. Week 4 example is covered by a specific regression test
2. Test cases cover all identified edge cases and boundary conditions
3. Code coverage for Points Available functions reaches >90%
4. Tests are well-documented with clear purpose statements
5. All new tests pass consistently

### **TICKET 6: Integration and Documentation**
- **Priority**: Low
- **Type**: Integration
- **Location**: Documentation files and CLI interface
- **Estimated Effort**: 2-3 hours

**Problem Statement**:
Implementation changes must maintain backward compatibility and be properly documented to ensure smooth adoption and future maintenance.

**Solution**:
- Ensure CLI output format remains unchanged for backward compatibility
- Update code documentation to reflect new architecture
- Add clear comments explaining the algorithm implementation
- Update any relevant design documentation

**Acceptance Criteria**:
1. CLI output format is identical to previous version
2. Code includes comprehensive comments explaining algorithm logic
3. Function documentation follows Go conventions
4. Any user-facing changes are documented appropriately

## **IMPLEMENTATION STRATEGY**

### Priority Order
Implementation should follow strict priority order to manage risk and ensure critical issues are addressed first:

1. **Critical Priority**: Ticket 2 (Floor Bug Fix) - Addresses the most severe calculation error
2. **High Priority**: Tickets 1, 3, 5 (Architecture, Algorithm, Tests) - Core implementation changes
3. **Medium Priority**: Ticket 4 (Position Calculations) - Supporting calculation fixes
4. **Low Priority**: Ticket 6 (Integration/Documentation) - Final polish and documentation

### Dependencies
- **Ticket 1 must complete before Ticket 3**: Architecture separation enables clean algorithm implementation
- **Ticket 2 can be implemented independently**: Critical bug fix can proceed in parallel
- **Ticket 5 should begin after Ticket 3**: Algorithm implementation needed before comprehensive testing
- **Ticket 4 can be implemented in parallel**: Position calculation fixes are independent
- **Ticket 6 should be last**: Final integration requires all implementation changes to be complete

### Testing Approach
- **Continuous Testing**: Run full test suite after each ticket completion
- **Incremental Validation**: Validate Week 4 example results after each major change
- **Regression Prevention**: Ensure all existing tests continue to pass throughout implementation
- **Documentation Testing**: Verify code examples in documentation remain accurate

## **SUCCESS CRITERIA**

### Primary Success Metrics
- **Correct Results**: Week 4 example produces Score=8, Points=13, Total=21
- **Test Integrity**: All existing tests continue to pass without modification
- **Backward Compatibility**: CLI output format maintained exactly
- **Performance**: No degradation in calculation speed for typical datasets

### Quality Metrics
- **Code Coverage**: Maintain or improve current test coverage levels
- **Documentation**: All public functions have clear, accurate documentation
- **Maintainability**: Code is well-structured with clear separation of concerns
- **Robustness**: Edge cases are properly handled and tested

### User Experience Metrics
- **Accuracy**: Negative Points Available values are preserved and displayed
- **Consistency**: Results are deterministic and reproducible
- **Transparency**: Algorithm behavior is clearly documented and understandable

## **RISK MITIGATION**

### Technical Risks
- **Test Foundation Secured**: All existing tests pass, providing confidence in current functionality
- **Incremental Implementation**: Each ticket can be validated independently before proceeding
- **Separation of Concerns**: Architectural refactoring reduces complexity and makes debugging easier
- **Comprehensive Testing**: New test coverage will catch regressions early

### Process Risks
- **Clear Dependencies**: Implementation order is specified to avoid conflicts
- **Rollback Strategy**: Each ticket can be reverted independently if issues arise
- **Validation Gates**: Week 4 example provides clear success/failure criteria at each step
- **Documentation**: Changes are documented to ensure future maintainability

### Business Risks
- **Backward Compatibility**: CLI format preservation ensures no disruption to existing users
- **Performance Monitoring**: Implementation includes performance validation
- **Gradual Rollout**: Changes can be deployed incrementally to minimize impact
- **Clear Success Criteria**: Objective metrics ensure implementation meets requirements

---

*This plan provides a comprehensive blueprint for fixing the points available implementation while maintaining system integrity and user experience. Each ticket is designed to be independently implementable and verifiable, reducing risk and ensuring steady progress toward the goal.*
