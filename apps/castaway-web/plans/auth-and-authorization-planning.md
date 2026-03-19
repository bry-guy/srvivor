# Auth and Authorization Plan

Status: `planning`

## Goal

Define the production authentication and authorization model for `castaway-web`.

## Open questions

- what authenticates bot-to-API traffic
- whether any human-facing API access is needed
- what write workflows require server-side authorization
- how secrets and credential rotation should be handled in production

## Related threads

- `service-to-service-authentication-planning.md` — first concrete auth slice for the self-hosted deployment path
