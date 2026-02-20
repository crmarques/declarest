---
name: spec-writer
description: Author or revise instruction/spec files with implementation-ready, testable guidance and minimal redundancy.
---

# Spec Writer

## Workflow
1. Run `spec-router` first and load the selected files; load `agents/reference/interfaces.md` before editing domain specs.
2. Use `agents/skills/spec-writer/templates/domain-template.md` for new domain files.
3. Keep requirements explicit and testable; use `MUST` only for hard constraints.
4. Keep files cohesive; split only when split triggers are present.
5. Keep `AGENTS.md` request routing and affected `agents/skills/*` workflows in sync when instruction files change.
6. Document contracts, failure modes, and at least one corner case for changed behavior.
7. Update `AGENTS.md` routing metadata when domain files are added or renamed.
8. Run `spec-auditor` after substantial changes.

## Writing Constraints
1. Prefer direct, implementation-ready statements over narrative prose.
2. Keep canonical interface names and shared contracts in `agents/reference/interfaces.md`.
3. Remove duplicated rules when a single canonical source already exists.
4. Use normative keywords consistently (`MUST`, `SHOULD`, `MAY`) and avoid ambiguous wording.
5. Keep examples concise and tied to real behavior.
6. Prefer deterministic, testable phrasing (explicit condition and expected outcome).
7. Optimize for efficiency by minimizing repeated guidance and cross-referencing canonical files.
8. Keep wording objective: specify observable behavior instead of intent-only language.

## Split Triggers
1. Mixed concerns in one file.
2. Unrelated churn causing frequent conflicts.
3. Review cognitive load grows beyond practical limits.
4. File size/complexity impairs safe editing.
