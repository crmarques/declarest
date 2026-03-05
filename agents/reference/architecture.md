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
2. Dependencies MUST flow inward toward domain contracts; only composition roots (for example `internal/bootstrap`) MAY wire concrete providers.
3. `orchestrator.Orchestrator` MUST own core resource orchestration behavior, and app-layer workflows in `internal/app` MUST compose domain interfaces without importing providers.
4. Public/domain packages MUST depend on interfaces, never provider concrete types.
5. `internal/cli` MUST remain an adapter layer (parse/validate/output) and delegate workflow behavior to `internal/app` and/or domain orchestrators; it MUST NOT import providers.
6. Go layout MUST keep executable entrypoints in `cmd/*` and non-public implementation in `internal/*`.
7. `internal/operator/controllers` MUST remain an operator adapter layer (Kubernetes reconciliation/watch/webhook handling) and MUST delegate resource-sync behavior to domain/application contracts via bootstrapped sessions.
8. Refactors affecting public contracts MUST update `agents/reference/interfaces.md` before implementation changes.
9. Refactors SHOULD be decomposed into reversible steps unless explicitly requested otherwise.

## Layer Model
1. `cmd/declarest`: executable entrypoint only.
2. `cmd/declarest-operator-manager`: Kubernetes operator entrypoint only.
3. `internal/cli`: command parsing, validation, and output formatting.
4. `internal/app`: application use-case services composed from domain interfaces.
5. `internal/operator/controllers`: Kubernetes reconcilers/webhook server that adapt CRD intent into domain workflows.
6. Public domain packages: `config`, `resource`, `metadata`, `repository`, `managedserver`, `secrets`, `orchestrator`.
7. Public shared primitives: `faults`.
8. Private provider implementations: `internal/providers/*`.
9. Bootstrap/wiring: `internal/bootstrap`.

## Allowed Dependency Directions
1. `cmd/declarest` -> `internal/bootstrap`, `internal/cli`.
2. `cmd/declarest-operator-manager` -> `internal/operator/controllers`, `internal/operator/observability`, `api/v1alpha1`.
3. `internal/cli/*` -> `internal/cli/cliutil`, `internal/app/*`, domain contracts (`config`, `orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`), and approved support primitives.
4. `internal/app/*` -> domain contracts (`orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`).
5. `internal/operator/controllers` -> `api/v1alpha1`, `internal/bootstrap`, `internal/app/resource/*`, domain contracts (`config`, `orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`), and Kubernetes/controller-runtime primitives.
6. `orchestrator` -> `repository`, `metadata`, `managedserver`, `secrets`, `resource`.
7. `internal/bootstrap` -> provider implementations in `internal/providers/*`.
8. `internal/providers/*` -> owner package interfaces/types.

## Forbidden Dependencies
1. `internal/cli` importing provider implementation packages.
2. `internal/app` importing `internal/cli` packages.
3. Domain packages (`resource`, `metadata`, `orchestrator`) importing `cmd`, `internal/cli`, or `internal/operator`.
4. `internal/operator/controllers` importing `internal/cli` or CLI parsing/output helpers.
5. `repository` directly invoking `managedserver` provider code.
6. Any non-module consumer importing `internal/*`.

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

### Operator Sync Flow
1. `SyncPolicy` reconcile resolves referenced CRDs (`ResourceRepository`, `ManagedServer`, `SecretStore`) and validates overlap/schedule/dependency constraints.
2. Controller builds a runtime `config.Context` and bootstraps domain services through `internal/bootstrap`.
3. Controller executes apply/prune orchestration through domain/application contracts and persists deterministic status/condition updates.

## Failure Modes
1. Circular dependencies between domain contracts and providers.
2. CLI bypasses orchestrator and calls providers directly.
3. Operator controller logic bypasses domain contracts and performs ad hoc sync behavior.
4. App workflows leak into provider packages or CLI/operator argument parsing code.
5. Orchestration logic leaks into repository/managedserver packages.

## Edge Cases
1. Context without remote server where local-only operations remain valid.
2. Secrets manager disabled while masked placeholders exist.
3. Metadata inference available with manual operation execution.

## Examples
1. Correct: orchestrator depends on `managedserver.ManagedServerClient` and receives HTTP provider through `internal/bootstrap`.
2. Incorrect: CLI imports `internal/providers/managedserver/http` and issues requests directly.
