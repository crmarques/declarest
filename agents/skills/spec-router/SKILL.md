---
name: spec-router
description: Route requests to the smallest useful set of reference files so context stays focused and deterministic.
---

# Spec Router

## Workflow
1. Identify the primary request type and changed bounded contexts.
2. Load `agents/reference/interfaces.md` first.
3. Apply the `AGENTS.md` request-to-file matrix for the minimum required files.
4. Add only the minimal additional files from the route map.
5. Add `agents/reference/quality.md` when behavior changes if not already loaded.
6. Add `agents/reference/use-cases.md` when scenario design or acceptance coverage is needed.
7. Record the selected files and rationale before making substantial edits.

## Route Map
1. Architecture/refactor: `agents/reference/architecture.md`, `agents/reference/code.md`, `agents/reference/quality.md`.
2. Domain modeling/behavior: `agents/reference/domain.md`, `agents/reference/metadata.md`, `agents/reference/orchestrator.md`.
3. Repository/path safety: `agents/reference/resource-repo.md`, `agents/reference/metadata.md`, `agents/reference/quality.md`.
4. Remote API behavior: `agents/reference/resource-server.md`, `agents/reference/metadata.md`, `agents/reference/orchestrator.md`, `agents/reference/quality.md`.
5. Context/config: `agents/reference/context-config.md`, `agents/reference/domain.md`, `agents/reference/quality.md`.
6. Secrets lifecycle: `agents/reference/secrets.md`, `agents/reference/orchestrator.md`, `agents/reference/quality.md`.
7. CLI behavior/output: `agents/reference/cli.md`, `agents/reference/orchestrator.md`, `agents/reference/domain.md`, `agents/reference/quality.md`.
8. E2E harness/profile/component: `agents/reference/e2e.md`, `agents/reference/quality.md`, `agents/reference/use-cases.md`.
9. Spec authoring only: target file plus `agents/reference/interfaces.md`, `agents/reference/code.md`, and `agents/reference/quality.md`.
10. Instruction/skill workflow change: `AGENTS.md`, affected `agents/skills/*`, `agents/reference/interfaces.md`, `agents/reference/code.md`, `agents/reference/quality.md`.
11. Quality strategy/test-policy change: `agents/reference/quality.md`, `agents/reference/use-cases.md`, `agents/reference/interfaces.md`.

## Guardrails
1. Keep context minimal; do not bulk-load unrelated domains.
2. Do not infer behavior that is not documented in loaded files.
3. Surface contradictions immediately and route to `spec-auditor`.
4. If matrix and route-map guidance diverge, use the stricter superset and patch the docs to remove drift.
