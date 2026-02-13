---
name: spec-writer
description: Create or update instruction/spec files in new-agent-specs with normative, implementation-ready language. Use when authoring domain specs, AGENTS bootstrap rules, templates, or interface contracts.
---

# Spec Writer

## Workflow
1. Load `new-agent-specs/agents/interfaces.md` before editing any domain file.
2. Use `new-agent-specs/skills/spec-writer/templates/domain-template.md` for each domain document.
3. Write imperative and normative language using MUST and SHOULD where behavior is mandatory or recommended.
4. Keep files cohesive and split only when split triggers are present.
5. Document data contracts, failure modes, edge cases, and examples in each domain file.
6. Update `new-agent-specs/AGENTS.md` load matrix when adding or renaming domain files.
7. Route to `spec-auditor` after substantial updates.

## Writing Constraints
1. Keep guidance implementation-ready and decision complete.
2. Preserve stable interface names and contracts in `interfaces.md`.
3. Avoid legacy style or accidental compatibility requirements unless explicitly defined.
4. Avoid filler prose; prefer structured, direct rules.

## Split Triggers
1. Mixed concerns in one file.
2. Unstable churn from unrelated edits.
3. Increasing review cognitive load.
4. Exceeded size or complexity threshold.
