# Architecture and Boundaries

## Purpose
Define component boundaries, dependency direction, and orchestration ownership for a maintainable rebuild.

## In Scope
1. Layer responsibilities.
2. Allowed and forbidden dependencies.
3. Cross-component interaction patterns.
4. Refactor constraints.

## Out of Scope
1. Framework-specific wiring details.
2. Build pipeline internals.
3. UI rendering concerns.

## Normative Rules
1. Global engineering and file-organization policies are defined in `AGENTS.md` and apply here.
2. Dependencies MUST flow inward toward domain contracts; only `core` MAY wire concrete providers.
3. `orchestrator.Orchestrator` MUST own core resource orchestration behavior, and app-layer workflows in `internal/app` MUST compose domain interfaces without importing providers.
4. Public/domain packages MUST depend on interfaces, never provider concrete types.
5. `internal/cli` MUST remain an adapter layer (parse/validate/output) and delegate workflow behavior to `internal/app` and/or domain orchestrators; it MUST NOT import providers.
6. Go layout MUST keep executable entrypoints in `cmd/*` and non-public implementation in `internal/*`.
7. Refactors affecting public contracts MUST update `agents/reference/interfaces.md` before implementation changes.
8. Refactors SHOULD be decomposed into reversible steps unless explicitly requested otherwise.

## Layer Model
1. `cmd/declarest`: executable entrypoint only.
2. `internal/cli`: command parsing, validation, and output formatting.
3. `internal/app`: application use-case services composed from domain interfaces.
4. Public domain packages: `config`, `resource`, `metadata`, `repository`, `server`, `secrets`, `orchestrator`.
5. Public shared primitives: `faults`.
6. Private provider implementations: `internal/providers/*`.
7. Public composition root: `core`.

## Allowed Dependency Directions
1. `cmd/declarest` -> `core`, `internal/cli`.
2. `internal/cli/*` -> `internal/cli/common`, `internal/app/*`, domain contracts (`config`, `orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`), and approved support primitives.
3. `internal/app/*` -> domain contracts (`orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`).
4. `orchestrator` -> `repository`, `metadata`, `server`, `secrets`, `resource`.
5. `core` -> provider implementations in `internal/providers/*`.
6. `internal/providers/*` -> owner package interfaces/types.

## Forbidden Dependencies
1. `internal/cli` importing provider implementation packages.
2. `internal/app` importing `internal/cli` packages.
3. Domain packages (`resource`, `metadata`, `orchestrator`, `core`) importing `cmd` or `internal/cli`.
4. `repository` directly invoking `server` provider code.
5. Any non-module consumer importing `internal/*`.

## Component Interaction

### Apply Flow
1. CLI forwards intent to orchestrator.
2. Orchestrator resolves metadata and identity.
3. Orchestrator builds request and executes remote mutation.
4. Orchestrator persists normalized local state.

### Refresh Flow
1. CLI requests remote read via orchestrator.
2. Orchestrator fetches remote resources and maps deterministic aliases.
3. Orchestrator writes normalized local resources.

### Diff/Explain Flow
1. Orchestrator loads local and remote states.
2. Compare transforms are applied.
3. Deterministic diff/explain output is returned to CLI.

## Failure Modes
1. Circular dependencies between domain contracts and providers.
2. CLI bypasses orchestrator and calls providers directly.
3. App workflows leak into provider packages or CLI argument parsing code.
4. Orchestration logic leaks into repository/server packages.

## Edge Cases
1. Context without remote server where local-only operations remain valid.
2. Secrets manager disabled while masked placeholders exist.
3. Metadata inference available with manual operation execution.

## Examples
1. Correct: orchestrator depends on `server.ResourceServer` interface and receives HTTP provider through `core`.
2. Incorrect: CLI imports `internal/providers/server/http` and issues requests directly.
