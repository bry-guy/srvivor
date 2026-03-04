---
description: Commits agent local changes as conventional commits.
---

Make git commit(s).

## Workflow

Identify the uncommitted changes you've made so far. Define logical, atomic commits. Try to ensure each commit keeps the codebase valid - passing tests and compile-able. 

Other agents may be making changes in the codebase at the same time - if you have a chat history or are given any context, avoid picking files not relevant to your session. If you have no history, assume you are being invoked from scratch, and handle all uncommitted changes.

Do not make code changes.

Make a plan for the commit(s). Make the commit.
