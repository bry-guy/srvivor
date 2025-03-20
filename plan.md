# Plan

Date: 2025-03-19

I have a few goals for this application:

- Code: Organize to be more modular, reusable, and testable
- Feature: Validate draft file meets format
- Feature: Validate draft meets roster
- Feature: Automatically clean draft input to meet roster
- Feature: Points Avaiable is correct
- Rename: `srvivor-cli`

Some of the above may move to the following, more major goal - to build an web app. 

`srvivor`, the web server for persistent hosting and tracking of survivor drafts:
- Feature: Discord integration
- Feature: Live drafting
- Feature: Score draft
- Feature: Draft/season history

## Current Plan

### Organize Code

I see a few primary functionalities:
- draft i/o and processing
- scoring
- cobra cmds

I want cobra cmds to follow conventions, so let me go look this up.
