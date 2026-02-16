---
name: spec-auditor
description: Audit instruction/spec changes for contract consistency, completeness, and actionable quality coverage.
---

# Spec Auditor

## Workflow
1. Load changed files and `agents/reference/interfaces.md`.
2. Run `agents/skills/spec-auditor/checklists/consistency-checklist.md`.
3. Flag contradictions, missing contracts, and rules without test expectations.
4. Report findings by severity with exact file references.
5. Propose minimal corrective edits aligned with bounded contexts.

## Audit Priorities
1. Interface drift from `agents/reference/interfaces.md`.
2. Boundary violations across architecture, reconciler, and providers.
3. Gaps in metadata, secrets, path safety, and CLI safeguard coverage.
4. Unnecessary duplication or file fragmentation.

## Output Rules
1. Mark each checklist item as pass, fail, or needs clarification.
2. Provide concrete remediation for every fail.
3. Keep recommendations implementation-ready and scoped.
