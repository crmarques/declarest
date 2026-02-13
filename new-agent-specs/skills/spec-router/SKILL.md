---
name: spec-router
description: Route engineering requests to the minimum required files in new-agent-specs. Use when implementing, reviewing, refactoring, or planning work so context loading stays focused and deterministic.
---

# Spec Router

## Workflow
1. Read the user request and identify primary intent.
2. Load `new-agent-specs/agents/interfaces.md` first.
3. Select minimal additional files from the routing table.
4. Add `new-agent-specs/agents/quality.md` when behavior changes are requested.
5. Add `new-agent-specs/agents/use-cases.md` when acceptance scenarios are needed.
6. Keep context small and avoid loading unrelated domains.

## Routing Table
1. Architecture or refactor request: load `architecture.md`, `code.md`, `quality.md`.
2. Domain or behavior modeling request: load `domain.md`, `metadata.md`, `reconciler.md`.
3. Repository or path safety request: load `resource-repo.md`, `metadata.md`, `quality.md`.
4. Remote API behavior request: load `resource-server.md`, `metadata.md`, `reconciler.md`, `quality.md`.
5. Context/config request: load `context-config.md`, `domain.md`, `quality.md`.
6. Secret lifecycle request: load `secrets.md`, `reconciler.md`, `quality.md`.
7. CLI behavior request: load `cli.md`, `reconciler.md`, `quality.md`.
8. Spec-authoring request: load target file and `code.md` for writing standards.

## Guardrails
1. Prefer explicit bounded-context file selection.
2. Avoid guessing behavior not documented in loaded files.
3. Raise contradictions immediately and route to `spec-auditor`.
