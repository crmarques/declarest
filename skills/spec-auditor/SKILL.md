---
name: spec-auditor
description: Audit new-agent-specs for consistency, completeness, and testability. Use when reviewing spec changes, validating cross-file contracts, or checking quality and security coverage.
---

# Spec Auditor

## Workflow
1. Load changed files and `new-agent-specs/agents/interfaces.md`.
2. Run checks from `new-agent-specs/skills/spec-auditor/checklists/consistency-checklist.md`.
3. Identify contradictions, missing contracts, and untested normative rules.
4. Report findings ordered by severity with exact file references.
5. Propose minimal corrective edits aligned with bounded contexts.

## Audit Priorities
1. Interface contract drift.
2. Boundary violations across architecture, reconciler, and adapters.
3. Missing edge cases for metadata, secrets, and path safety.
4. CLI contract changes without matching quality coverage.
5. File organization violations or unnecessary proliferation.

## Output Rules
1. Mark each check as pass, fail, or needs clarification.
2. Require concrete remediation for each fail item.
3. Keep recommendations implementation-ready.
