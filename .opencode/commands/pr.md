# /pr - Handle Pull Request Workflows

## Description
The /pr command handles pull request workflows for the srvivor repository. It supports both new PR creation and management of existing PRs, with special emphasis on resolving git conflicts before proceeding with other tasks.

## Usage
/pr [PR_IDENTIFIER]

Where PR_IDENTIFIER is optional:
- If provided: an integer (e.g., 4) or GitHub PR URL (e.g., https://github.com/bry-guy/srvivor/pull/4)
- If omitted: searches for existing PRs on the current branch

## Workflow

### Constraints
- ALL GitHub interactions MUST use the `@github` agent exclusively. Do NOT use external CLI tools like `gh`, `git`, or any bash commands for GitHub operations.
- Stick STRICTLY to the defined workflows. Do NOT perform actions outside of the listed steps (e.g., no merging, no creating or deleting branches, no direct pushes).
- If an action is not explicitly defined in the workflow, do NOT perform it. Seek user confirmation for any deviations.

### Conflict Resolution
- **Autonomous Handling**: If conflicts can be resolved without altering main's existing behavior (e.g., adding new imports, non-logic changes), agents may proceed autonomously. Log the resolution for transparency.
- **User Involvement**: If resolution requires modifying main's behavior (e.g., logic changes, overwriting code), agents MUST NOT proceed. Instead, propose changes to the user as a series of accept/deny options, e.g.:
  - "Accept: Keep main's version of [file:line]?"
  - "Deny: Use incoming version?"
  - "Custom: Suggest merge with [specific edit]?"
  - Wait for user response before continuing.
- **Safety**: Abort operations if uncertain; never overwrite without approval. Conflicts reported by the hosting platform (e.g., GitHub) should trigger this protocol.

### Argument Validation
* PR identifier must be either:
  - A valid integer (e.g., "4")
  - A GitHub PR URL (e.g., "https://github.com/bry-guy/srvivor/pull/4")
* If $ARGUMENTS is empty or invalid
  - Search for existing pulls for the current feature branch via `@github`
    - If PR found, use that PR
    - If none found, assume you are creating a new PR at origin

Note: GitHub username is bry-guy.

### Workflow: New PR
0. **Name**: Based on the changes you've made, define a name (referred to as `$NAME`). If the branch is already named, defer to that.
1. **Setup**: If you are on main, make a feature branch named `bry-guy/$NAME`. If you are on a feature branch, stay on that branch.
2. **Commit:** Commit your changes using /commit.
3. **Push:** Use `@github` to push all commits to PR branch.

### Workflow: Existing PR
0. **PR Number Extraction:** If $ARGUMENTS is a URL, extract the PR number using regex pattern `/pull/(\d+)`
1. **Fetch:** Use `@github` agent to get PR branch name and review comments.
2. **Setup**: Checkout and sync the PR branch, if needed. **MANDATORY: Check for git conflicts on the target PR. If conflicts exist, resolve them per Conflict Resolution guidelines before proceeding.** Only continue after conflicts are resolved.
3. **Planning:** Create a todo list to address comments.
4. **Implementation:** Execute the todos.
5. **Self-Review:** Think deeply about your changes, and make improvements, if any.
6. **Commit:** Commit your changes using /commit.
7. **Push:** Use `@github` to push all commits to PR branch
8. **PR Update:** Use `@github` agent to mark resolved comments and update PR status

### CI/CD
1. **Search:** Use `@github` to find CI issues.
2. **Planning:** Create a todo list to address the issues. If CI issues involve conflicts, follow Conflict Resolution protocols.
3. **Self-Review:** Think deeply about your changes, and make improvements, if any.
4. **Commit:** Commit your changes using /commit.
5. **Push:** Use `@github` to push all commits to PR branch

If no comments or issues, summarize status and await user instructions. Do NOT merge or close the PR.