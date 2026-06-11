# Architecture and Boundaries

## Purpose
Define component layers, dependency direction, orchestrator-vs-app split, OLM packaging boundary, and component-interaction flows.

## Normative Rules
1. Global engineering and file-organization policies in `AGENTS.md` apply here.
2. Dependencies MUST flow inward toward domain contracts; only composition roots (for example `internal/bootstrap`) MAY wire concrete providers.
3. Public/domain packages MUST depend on interfaces, never on provider concrete types.
4. Go layout MUST keep executable entrypoints in `cmd/*` and non-public implementation in `internal/*`.
5. `internal/cli` MUST remain an adapter layer (parse/validate/output), MUST delegate workflow behavior to `internal/app` and/or domain orchestrators, and MUST NOT import providers.
6. `internal/operator/controllers` MUST remain an operator adapter layer (reconcile/watch/webhook) and MUST delegate resource-sync behavior to domain/application contracts via bootstrapped sessions.
7. Refactors affecting public contracts MUST update `agents/reference/interfaces.md` before implementation changes.
8. Refactors SHOULD be decomposed into reversible steps unless explicitly requested otherwise.
9. OLM packaging artifacts (`config/manifests/`, `config/olm/`, `bundle/`, `catalog/`, `bundle.Dockerfile`, `catalog.Dockerfile`) MUST remain packaging-only overlays over the kustomize base in `config/default/`, MUST NOT introduce a new runtime dependency layer, MUST be derived from that base, and MUST NOT hand-duplicate controller or webhook logic.

## Layer Model
1. `cmd/declarest`: CLI executable entrypoint only.
2. `cmd/declarest-operator-manager`: operator executable entrypoint only.
3. `internal/cli`: command parsing, validation, output formatting.
4. `internal/app`: application use-case services composed from domain interfaces.
5. `internal/operator/controllers`: reconcilers/webhook server adapting CRD intent into domain workflows.
6. Public domain packages: `config`, `resource`, `metadata`, `repository`, `managedservice`, `secrets`, `orchestrator`, `cronexpr`, `envref`.
7. Public shared primitives: `faults`.
8. Private provider implementations: `internal/providers/*`.
9. Bootstrap/wiring: `internal/bootstrap`.
10. Packaging artifacts: `config/` (kustomize base + release/manifests/OLM overlays), `bundle/` (operator-sdk `registry+v1` bundle), `catalog/` (file-based catalog); these wrap the same Deployment/RBAC/CRDs and MUST NOT host reconcile or webhook code.

## Orchestrator vs App Split
1. `internal/orchestrator/*` owns the default `orchestrator.Orchestrator`: pure domain workflow. It MUST NOT host prompts, confirmation/dry-run shims, progress reporting, or interactive retry UX. Errors MUST propagate as `faults.TypedError` values.
2. `internal/app/*` owns CLI/operator-facing use-case services composing domain interfaces. Side-effects around the core (interactive prompts, confirmation gates, progress output, retry UX, batching loops, CLI/operator-specific error shaping) MUST live here.
3. New workflow behavior SHOULD land in `internal/orchestrator/*` when it is pure domain logic reused across CLI and operator, and in `internal/app/*` when it is specific to a CLI or operator invocation context.

## Allowed Dependency Directions
1. `cmd/declarest` -> `internal/bootstrap`, `internal/cli`.
2. `cmd/declarest-operator-manager` -> `internal/operator/controllers`, `internal/operator/observability`, `api/v1alpha1`.
3. `internal/cli/*` -> `internal/cli/cliutil`, `internal/app/*`, `internal/promptauth`, domain contracts (`config`, `orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`), approved support primitives.
4. `internal/app/*` -> domain contracts (`orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`).
5. `internal/operator/controllers` -> `api/v1alpha1`, `internal/bootstrap`, `internal/app/resource/*`, domain contracts (`config`, `orchestrator`, `repository`, `metadata`, `resource`, `secrets`, `faults`), Kubernetes/controller-runtime primitives.
6. `orchestrator` -> `repository`, `metadata`, `managedservice`, `secrets`, `resource`.
7. `internal/bootstrap` -> `internal/providers/*`.
8. `internal/providers/*` -> owner package interfaces/types.

## Forbidden Dependencies
1. `internal/cli` importing provider implementation packages.
2. `internal/app` importing `internal/cli` packages.
3. Domain packages (`resource`, `metadata`, `orchestrator`) importing `cmd`, `internal/cli`, or `internal/operator`.
4. `internal/operator/controllers` importing `internal/cli` or CLI parsing/output helpers.
5. `repository` directly invoking `managedservice` provider code.
6. Any non-module consumer importing `internal/*`.

## Component Interaction Flows

### Apply
1. CLI forwards intent to orchestrator.
2. Orchestrator resolves metadata and identity.
3. Orchestrator builds the request and executes the remote mutation.
4. Orchestrator persists normalized local state.

### Refresh
1. CLI requests remote read via orchestrator.
2. Orchestrator fetches remote resources and maps deterministic aliases.
3. Orchestrator writes normalized local resources.

### Diff/Explain
1. Orchestrator loads local and remote states.
2. Compare transforms are applied.
3. Deterministic diff/explain output is returned to CLI.

### Operator Sync
1. `SyncPolicy` reconcile resolves referenced CRDs (`ResourceRepository`, `ManagedService`, `SecretStore`) and validates overlap/schedule/dependency constraints.
2. Controller builds a runtime `config.Context` and bootstraps domain services through `internal/bootstrap`.
3. Controller executes apply/prune orchestration through domain/application contracts and persists deterministic status/condition updates.

### OLM Packaging
1. `make bundle` renders the CSV template from `config/manifests/` into `bundle/manifests/` via `operator-sdk generate bundle --manifests`, preserves the hand-authored `bundle.Dockerfile`, `bundle/metadata/annotations.yaml`, and scorecard config, and stamps CSV image metadata from `VERSION`.
2. `make release-bundle VERSION=<VERSION>` regenerates bundle and file-based catalog together, validates `config/olm/`, `operator-sdk bundle validate`, `opm validate`, and version/image consistency; it is the release workflow's canonical OLM staging target.
3. The tag-triggered release workflow publishes operator image, bundle image, catalog image, and GitHub release as one ordered DAG; standalone image workflows are manual smoke builds only.
4. `make olm-install` applies `config/olm/` (Namespace/OperatorGroup/CatalogSource/Subscription) against a target cluster; OLM then reconciles CSVs and injects webhook certificates for the operator `Deployment`.

## Failure Modes
1. Circular dependencies between domain contracts and providers.
2. CLI bypasses orchestrator and calls providers directly.
3. Operator controller bypasses domain contracts and performs ad hoc sync.
4. App workflows leak into provider packages or CLI/operator argument parsing.
5. Orchestration logic leaks into `repository`/`managedservice` packages.

## Edge Cases
1. Context without remote server: local-only operations remain valid.
2. Secrets manager disabled while masked placeholders exist.
3. Metadata inference available alongside manual operation execution.

## Examples
1. Correct: `orchestrator` depends on `managedservice.ManagedServiceClient` and receives the HTTP provider through `internal/bootstrap`.
2. Incorrect: CLI imports `internal/providers/managedservice/http` and issues requests directly.
