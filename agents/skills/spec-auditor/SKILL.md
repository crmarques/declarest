---
name: spec-auditor
description: Audit instruction/spec changes for contract consistency, completeness, and actionable quality coverage.
---

# Spec Auditor

## Workflow
1. Load changed files, `AGENTS.md`, and `agents/reference/interfaces.md`.
2. When instruction/skill files changed, also load affected `agents/skills/*` files.
3. Run `agents/skills/spec-auditor/checklists/consistency-checklist.md`.
4. Flag contradictions, missing contracts, and rules without test expectations.
5. Report findings by severity with exact file references.
6. Mark each checklist item as pass, fail, or needs clarification.
7. Propose minimal corrective edits aligned with bounded contexts.

## Audit Priorities
1. Interface drift from `agents/reference/interfaces.md`.
2. Boundary violations across architecture, orchestrator, and providers.
3. Gaps in metadata, secrets, path safety, and CLI safeguard coverage.
4. Routing/skill-order inconsistencies between `AGENTS.md` and `agents/skills/*`.
5. Unnecessary duplication or file fragmentation.

## Output Rules
1. Mark each checklist item as pass, fail, or needs clarification.
2. Provide concrete remediation for every fail.
3. State explicit `no findings` when all checks pass and mention residual risk/testing gaps.
4. Keep recommendations implementation-ready and scoped.
