# srvivor Production Readiness Checklist

This checklist applies to the final stable CLI line and archive-quality releases.

## Release quality
- [ ] Build, lint, and test workflows pass
- [ ] README and command examples match actual CLI behavior
- [ ] Changelog updated for the release/tag
- [ ] Version/tag strategy documented for the stable CLI line

## Reliability
- [ ] Regression coverage remains intact for historical scoring behavior
- [ ] Roster validation and draft normalization workflows verified
- [ ] Example local workflow tested end-to-end

## Archive readiness
- [ ] CLI scope clearly marked as legacy/maintenance mode
- [ ] Relationship to newer platform apps documented
- [ ] Any future maintenance expectations documented

## Follow-up thread

- `plans/archive-policy-planning.md`

## Current status

Current state: ready to serve as the stable legacy CLI reference, with future changes expected to be maintenance-oriented.
