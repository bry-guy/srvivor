# castaway-web Functional Requirements

`castaway-web` provides the persistent HTTP API for Castaway draft state.

## Inputs

- HTTP requests to the documented API routes
- PostgreSQL-backed persisted state
- seed data from `seeds/historical-seasons.json`
- environment-based runtime configuration

## Required capabilities

- expose a health endpoint for local and production monitoring
- create and list instances
- import an instance from structured submissions
- create and list contestants for an instance
- create and list participants for an instance
- create and retrieve draft picks for a participant
- create and retrieve ordered outcome positions
- compute and return leaderboard results from drafts plus outcomes
- support bot-friendly filters for instances, participants, and leaderboard lookups
- seed historical seasons into the database for development and testing
- keep the documented API contract aligned with the running server

## Outputs

- JSON responses for all public API routes
- persisted instance, contestant, participant, draft, and outcome state in PostgreSQL
- generated OpenAPI output derived from TypeSpec

## Current non-goals

- bonus points systems (`ponies`, immunity, journeys, and similar mechanics)
- production-grade multi-tenant auth and authorization beyond current trusted-local workflows
