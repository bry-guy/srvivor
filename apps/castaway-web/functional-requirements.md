# castaway-web Functional Requirements

`castaway-web` provides the persistent HTTP API for Castaway draft state.

## Inputs

- HTTP requests to the documented API routes
- PostgreSQL-backed persisted state
- seed data from `seeds/historical-seasons.json`
- local verification seed data from `seeds/verification-merge-gameplay.json`
- environment-based runtime configuration

## Required capabilities

- expose a health endpoint for local and production monitoring
- create and list instances
- import an instance from structured submissions
- create and list contestants for an instance
- create and list participants for an instance
- create and retrieve draft picks for a participant
- create and retrieve ordered outcome positions
- compute and return leaderboard results from drafts plus outcomes, including linked Discord user ids and current tribe names for bot-facing score formatting
- support bot-friendly filters for instances, participants, contestants, activities, and leaderboard lookups
- support bonus gameplay persistence and resolution for:
  - tribal pony
  - tribe wordle
  - journeys
  - Stir the Pot
  - individual pony auctions and ownership
  - Loan Shark borrowing and repayment
  - individual pony immunity payouts
- support player-context write flows via linked Discord users for merge gameplay actions
- allow instance admins to submit Stir the Pot contributions and individual pony bids on behalf of named participants
- allow instance admins to inspect the current Stir the Pot total for a named tribe without exposing other tribes' totals
- bind newly opened Stir the Pot rounds and auction lots to the next scheduled episode for the instance
- reveal consumed secret bonus points into public-safe ledger rows when hidden spends use them
- support instance-admin write flows for opening/closing merge gameplay windows and recording immunity winners
- seed historical seasons into the database for development and testing
- keep the documented API contract aligned with the running server

## Outputs

- JSON responses for all public API routes
- persisted instance, contestant, participant, draft, outcome, bonus ledger, pony ownership, loan, and gameplay state in PostgreSQL
- generated OpenAPI output derived from TypeSpec

## Current non-goals

- production-grade multi-tenant auth and authorization beyond current trusted-local workflows
