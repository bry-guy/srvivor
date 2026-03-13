# State Backend and Operations Plan

Status: `planning`

## Goal

Define the production storage and operational model for `castaway-discord-bot`.

## Open questions

- whether local file-backed state is sufficient for the intended deployment model
- what backup and restore expectations apply to saved defaults
- what restart, outage, and token-rotation runbooks are required
- whether multi-instance deployment is in scope
