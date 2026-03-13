# Monorepo Restructure Plan

Status: `done`

## Goal

Restructure the repository from a single CLI app into a monorepo foundation that can support multiple Castaway apps while preserving existing CLI workflows.

## Delivered scope

- moved the legacy gameplay tooling into `apps/cli/`
- established app-local and shared package boundaries for future apps
- standardized task execution around `mise`
- updated CI and developer workflows for the monorepo layout
- preserved historical notes in `docs/archive-wiki/`

## Historical references

- `docs/archive-wiki/archive/20251016T150000_current_app_state.md`
- `docs/archive-wiki/archive/20251016T160000_monorepo_restructure.md`
- `docs/archive-wiki/archive/archived_docs.md`
- `docs/archive-wiki/archive/archived_thoughts.md`
