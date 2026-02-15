---
name: spec-router
description: Route engineering requests to the minimum required files in new-agent-specs. Use when implementing, reviewing, refactoring, or planning work so context loading stays focused and deterministic.
---

# Spec Router

## Workflow
1. Read the user request and identify primary intent.
2. Load `agents/reference/interfaces.md` first.
3. Select minimal additional files from the routing table.
4. Add `agents/reference/quality.md` when behavior changes are requested.
5. Add `agents/reference/use-cases.md` when acceptance scenarios are needed.
6. Keep context small and avoid loading unrelated domains.

## Routing Table
1. Architecture or refactor request: load `agents/reference/architecture.md`, `agents/reference/code.md`, `agents/reference/quality.md`.
2. Domain or behavior modeling request: load `agents/reference/domain.md`, `agents/reference/metadata.md`, `agents/reference/reconciler.md`.
3. Repository or path safety request: load `agents/reference/resource-repo.md`, `agents/reference/metadata.md`, `agents/reference/quality.md`.
4. Remote API behavior request: load `agents/reference/resource-server.md`, `agents/reference/metadata.md`, `agents/reference/reconciler.md`, `agents/reference/quality.md`.
5. Context/config request: load `agents/reference/context-config.md`, `agents/reference/domain.md`, `agents/reference/quality.md`.
6. Secret lifecycle request: load `agents/reference/secrets.md`, `agents/reference/reconciler.md`, `agents/reference/quality.md`.
7. CLI behavior request: load `agents/reference/cli.md`, `agents/reference/reconciler.md`, `agents/reference/quality.md`.
8. Spec-authoring request: load target file and `agents/reference/code.md` for writing standards.

## Guardrails
1. Prefer explicit bounded-context file selection.
2. Avoid guessing behavior not documented in loaded files.
3. Raise contradictions immediately and route to `spec-auditor`.
