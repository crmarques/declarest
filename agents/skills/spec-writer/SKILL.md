---
name: spec-writer
description: Author or revise instruction/spec files with implementation-ready, testable guidance and minimal redundancy.
---

# Spec Writer

## Workflow
1. Load `agents/reference/interfaces.md` before editing domain specs.
2. Use `agents/skills/spec-writer/templates/domain-template.md` for new domain files.
3. Keep requirements explicit and testable; use `MUST` only for hard constraints.
4. Keep files cohesive; split only when split triggers are present.
5. Document contracts, failure modes, and at least one corner case for changed behavior.
6. Update `AGENTS.md` routing metadata when domain files are added or renamed.
7. Run `spec-auditor` after substantial changes.

## Writing Constraints
1. Prefer direct, implementation-ready statements over narrative prose.
2. Keep canonical interface names and shared contracts in `agents/reference/interfaces.md`.
3. Remove duplicated rules when a single canonical source already exists.
4. Keep examples concise and tied to real behavior.

## Split Triggers
1. Mixed concerns in one file.
2. Unrelated churn causing frequent conflicts.
3. Review cognitive load grows beyond practical limits.
4. File size/complexity impairs safe editing.
