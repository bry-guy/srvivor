# srvivor Functional Requirements

`srvivor` is the legacy CLI for local Survivor draft scoring and draft normalization workflows.

## Inputs

- draft files under `drafts/<season>/`
- final placement files under `drafts/<season>/final.txt` or `finals/`
- roster files under `rosters/<season>.json`
- command-line flags and local filesystem paths

## Required capabilities

- score a single draft file for a selected season
- score one or more named drafters for a selected season
- validate contestant names against canonical season rosters
- normalize and fix draft files against canonical roster names
- preserve deterministic scoring behavior for historical seasons
- support local development, testing, and fixture-based regression checks

## Outputs

- terminal-readable scoring output
- validation and normalization feedback
- updated draft files when fix workflows are run without dry-run mode

## Current non-goals

- new platform feature development beyond maintenance of the legacy workflow
- replacing `castaway-web` as the long-term persistent system of record
