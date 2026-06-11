---
name: spec-authoring
description: Route to minimal context, then author or revise spec/instruction files with lean, testable, deduplicated guidance, and self-audit the result.
---

# Spec Authoring

Use when creating or revising any file under `agents/` (reference specs, `AGENTS.md`, skills). Covers context routing, writing, and self-audit in one pass.

## 1. Route to minimal context
1. Identify the request type and changed bounded contexts.
2. Load `agents/reference/interfaces.md` first (canonical contracts).
3. Load the files from the `AGENTS.md` request-to-file matrix for the matched row(s); for mixed requests use the union.
4. Add `agents/reference/quality.md` when behavior, contracts, security, verification, or any normative MUST/SHOULD rule changes.
5. Add `AGENTS.md` and affected `agents/skills/*` when instruction or skill workflows change.
6. Do not bulk-load unrelated domains.

## 2. Write / revise
1. For a new domain file, start from `templates/domain-template.md`.
2. Keep one dominant responsibility per file (see ownership map in `AGENTS.md`). Split only on a real split trigger: mixed concerns, conflict-causing churn, or size that impairs safe editing.
3. State observable, testable behavior: explicit condition → outcome. Use `MUST`/`SHOULD`/`MAY` consistently; reserve `MUST` for hard constraints.
4. Own each concept in exactly one file. If another file owns it, write a one-line pointer instead of restating the rule.
5. Keep canonical type/interface names in `agents/reference/interfaces.md`; reference them, never redefine.
6. Document contracts, failure modes, and at least one corner case for changed behavior. Keep examples concise and tied to real behavior.
7. Renumber lists cleanly; never leave duplicate list numbers.

## 3. Keep cross-references in sync
1. When a domain file is added, renamed, or has its scope changed, update the `AGENTS.md` domain catalog and request-to-file matrix in the same change.
2. When persisted metadata/context/bundle contracts change, update `schemas/*.json` in the same change (see `agents/reference/quality.md`).

## 4. Self-audit (before handoff)
1. Run `checklists/consistency-checklist.md` and mark each item pass / fail / needs-clarification.
2. Provide concrete remediation for every fail; resolve fails before handoff or surface them as blockers.
3. Confirm: no interface drift from `interfaces.md`, no duplicated rules across files, no references to missing files, normative language consistent, every new rule has a traceable test expectation.
