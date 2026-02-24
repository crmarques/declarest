---
name: spec-router
description: Route requests to the smallest useful set of reference files so context stays focused and deterministic.
---

# Spec Router

## Workflow
1. Identify the primary request type and changed bounded contexts.
2. Load `agents/reference/interfaces.md` first.
3. Use the `AGENTS.md` request-to-file matrix as the canonical source for required files.
4. Add only minimal supplemental files required by the scope-expanding factors below.
5. Record selected files and rationale before substantial edits.

## Scope-Expanding Factors
1. Add `agents/reference/quality.md` when behavior, contracts, security, or verification expectations change.
2. Add `agents/reference/use-cases.md` when scenario design, acceptance criteria, or corner-case coverage changes.
3. Add `AGENTS.md` and affected `agents/skills/*` files when instruction or skill workflows are touched.
4. For mixed request types, use the strict union of required files across all matched matrix rows.

## Guardrails
1. Keep context minimal; do not bulk-load unrelated domains.
2. Do not infer behavior that is not documented in loaded files.
3. Surface contradictions immediately and route to `agents/skills/spec-auditor/SKILL.md`.
4. If matrix guidance is missing or ambiguous, patch `AGENTS.md` first and then continue.
