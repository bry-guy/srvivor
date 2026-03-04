## Codex Orchestration Workflow

When asked to implement a feature using the "codex orchestration workflow" (or similar),
follow this sequence. You (Opus) are the orchestrator throughout.

### Phase 1: Scout Context
Use the `context-scout` subagent. Tell it what feature you're about to implement and
which areas of the codebase are likely relevant. It will return a structured summary of
the actual file layout, key types/interfaces, conventions, and dependencies. This context
feeds into every subsequent phase — do not skip it.

### Phase 2: Plan
Write a detailed implementation plan using the scout's findings. Include file paths,
function signatures, data flow, and key design decisions. Reference actual existing
code — never assume structure.

### Phase 3: Plan Critique
Use the `plan-critic` subagent. Pass it your full plan AND the scout summary. It sends
both to GPT-5.3-Codex (high reasoning) for critique. You'll receive issues rated by
severity.

### Phase 4: Revise Plan
Fix every CRITICAL/HIGH issue from the critique. Use judgment on MEDIUM. Update the plan.

### Phase 5: Implement + Review Loop
For each unit of work in the revised plan:

1. **Write**: Use the `code-writer` subagent. Pass the plan section, scout context,
   and any relevant existing code. It uses GPT-5.3-Codex (low reasoning) for fast generation.

2. **Review**: Use the `code-reviewer` subagent. Pass the generated code, the plan section,
   and relevant existing code. It uses GPT-5.3-Codex (high reasoning) for thorough review.

3. **Fix** (if needed): If the review returns FAIL or PASS WITH CHANGES, read the critique
   yourself, then call `code-writer` again with the original code + specific fix instructions
   derived from the critique. Re-review once. If it still fails after two rounds, take over
   and fix it yourself.

### Phase 6: Integration
Once all code passes review, integrate it into the codebase yourself. Run tests, resolve
any remaining issues, and deliver the result.

### Important Rules
- Run subagents in the **foreground** (MCP tools don't work in background subagents).
- Always include scout context in every Codex call — it has no knowledge of the project.
- Each subagent call should be self-contained with all needed context.
- Keep the write→review→fix loop to max 2 iterations per unit. If it's still failing,
  you (Opus) are better positioned to fix it with your full context.
- For large features, batch into logical units and pipeline them sequentially.
