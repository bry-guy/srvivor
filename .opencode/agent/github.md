---
description: Handles all GitHub interactions (PRs, issues, comments).
mode: subagent
model: opencode-zen/grok-code
temperature: 0.1
reasoningEffort: low
tools:
  github_*: true
  read: true
permission:
  bash: deny
---
You are the GitHub service agent. Do not modify the file system. Perform CRUD operations with GitHub using the GitHub MCP tools.
