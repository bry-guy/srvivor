---
description: Handle pull requests - pushes, addressing comments, fixing issues identified by CI/CD.
---
**PULL REQUEST:** $ARGUMENTS

## Argument Validation

* PR identifier must be either:
  - A valid integer (e.g., "2673")
  - A GitHub PR URL (e.g., "https://github.com/owner/repo/pull/2673")
* If $ARGUMENTS is empty or invalid
  - Search for existing pulls for the current feature branch via `@github`
    - If PR found, use that PR
    - If none found, assume you are creating a new PR at origin

## Workflow: New PR

0. **Name**: Based on the changes you've made, define a name (referred to as `$NAME`). If the branch is already named, defer to that.
1. **Setup**: If you are on main, make a feature branch named `bry-guy/$NAME`. If you are on a feature branch, stay on that branch.
2. **Commit:** Commit your changes using /commit.
3. **Push:** Use `@github` to push all commits to PR branch.

## Workflow: Existing PR

0. **PR Number Extraction:** If $ARGUMENTS is a URL, extract the PR number using regex pattern `/pull/(\d+)`
1. **Fetch:** Use `@github` agent to get PR branch name and review comments. 
2. **Setup**: Checkout and sync the PR branch, if needed. 

### Code 

1. **Planning:** Create a todo list to address comments.
2. **Implementation:** Execute the todos.
3. **Self-Review:** Think deeply about your changes, and make improvements, if any.
4. **Commit:** Commit your changes using /commit.
5. **Push:** Use `@github` to push all commits to PR branch
6. **PR Update:** Use `@github` agent to mark resolved comments and update PR status

### CI/CD

1. **Search:** Use `@github` to find CI issues.
2. **Planning:** Create a todo list to address the issues.
3. **Self-Review:** Think deeply about your changes, and make improvements, if any.
4. **Commit:** Commit your changes using /commit.
5. **Push:** Use `@github` to push all commits to PR branch
