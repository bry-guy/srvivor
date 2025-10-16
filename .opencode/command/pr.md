---
description: Handle pull requests - pushes, addressing comments, fixing issues identified by CI/CD.
---
**PULL REQUEST:** $ARGUMENTS

## Argument Validation

* PR identifier must be either:
  - A valid integer (e.g., "2673")
  - A GitHub PR URL (e.g., "https://github.com/owner/repo/pull/2673")
* If $ARGUMENTS is empty or invalid, respond: 'Please provide a valid PR number or GitHub PR URL'

## Workflow

0. **PR Number Extraction:** If $ARGUMENTS is a URL, extract the PR number using regex pattern `/pull/(\d+)`
1. **Fetch:** Use `@github` agent to get PR branch name and review comments. 
2. **Setup**: Checkout and sync the PR branch, if needed.
3. **Planning:** Create a todo list to address comments.
4. **Implementation:** Execute the todos.
5. **Self-Review:** Think deeply about your changes, and make improvements, if any.
6. **Commit:** Commit your changes using /commit.
6. **Push:** Use `@github` to push all commits to PR branch
7. **PR Update:** Use `@github` agent to mark resolved comments and update PR status
