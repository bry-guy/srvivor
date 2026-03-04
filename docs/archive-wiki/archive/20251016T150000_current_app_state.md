# Current State of srvivor Application

## Overview
srvivor is a Go-based CLI application for scoring Survivor TV show fantasy drafts. It calculates scores based on how accurately drafters predict the elimination order of contestants.

## Key Features
- **Score Calculation**: Computes draft scores by comparing draft picks to actual final elimination positions
- **Draft Fixing**: Normalizes contestant names using fuzzy matching against canonical rosters
- **Roster Management**: Stores season contestant data in JSON format for name validation
- **Validation**: Ensures draft entries match known contestants

## Architecture
- **Language**: Go 1.24.0
- **CLI Framework**: Cobra for command structure
- **Key Dependencies**:
  - `github.com/spf13/cobra`: CLI commands
  - `github.com/stretchr/testify`: Testing
  - `github.com/kelseyhightower/envconfig`: Configuration
  - `github.com/agnivade/levenshtein`: Fuzzy string matching

## Core Components
- `cmd/`: CLI command definitions (score, fix-drafts)
- `internal/scorer/`: Scoring logic with recent refactoring to separate current score and points available calculations
- `internal/matcher/`: Name matching and normalization
- `internal/roster/`: Roster loading and management
- `internal/config/`: Application configuration
- `internal/log/`: Logging setup

## Data Structure
- `drafts/[season]/[drafter].txt`: Individual draft files
- `finals/[season].txt`: Final elimination results
- `rosters/[season].json`: Canonical contestant information

## Recent Changes
- Refactored scoring functions for better testability and separation of concerns
- Added points available calculation to show potential remaining points
- Improved name matching with multiple strategies (exact, nickname, component, fuzzy)
- Enhanced validation and error handling

## Scoring Logic
- Score = sum(max(0, positionValue - distance)) for eliminated players
- Points Available = potential points from remaining survivors
- Position value based on draft pick order (earlier picks worth more)
- Distance = |draft_position - final_position|

## Development Status
- Active development with comprehensive test coverage
- Uses conventional Go practices (slog for logging, testify for assertions)
- Includes linting (golangci-lint) and build automation (Makefile)
- Supports multiple seasons with extensible roster system

## Future Considerations
- TODO items in code suggest adding scored draft output and printable results
- Potential for web interface or additional analysis features
- Roster management could be expanded for automated updates