# Architecture and Boundaries

## Purpose
Define component boundaries, dependency direction, and orchestration flows for a maintainable rebuild.

## In Scope
1. Architectural layers and responsibilities.
2. Allowed and forbidden dependencies.
3. Cross-component interaction patterns.
4. Refactor and evolution rules.

## Out of Scope
1. Framework-specific wiring.
2. Build tooling internals.
3. UI rendering details.

## Normative Rules
1. Architecture and implementation decisions MUST meet senior software engineering best practices.
2. Directory structure MUST be human-legible from the tree alone through bounded contexts and predictable naming.
3. Files MUST have scoped responsibility and clear ownership.
4. Files MUST be sufficiently informative and self-contained so architecture intent is clear from repository shape and file content.
5. File proliferation MUST be avoided; keep cohesive files until split triggers are met.
6. Files MUST be split when at least one trigger applies: mixed concerns, unstable churn, review cognitive load growth, or size/complexity threshold exceeded.
7. Any new split file MUST be dedicated and narrowly scoped.
8. Public interfaces and shared types MUST remain stable and centrally documented in `new-agent-specs/agents/interfaces.md`.
9. Legacy disorganization patterns MUST NOT influence new architecture.
10. Dependencies MUST flow inward toward domain contracts, never outward from core contracts to adapters.
11. The reconciler MUST be the only orchestrator combining repository, metadata, server, and secret concerns.

## Data Contracts
Primary runtime components:
1. `ContextConfigManager` builds `Context`.
2. `Context` wires `ResourceRepositoryManager`, `ResourceMetadataManager`, optional `ResourceServerManager`, optional `SecretManager`.
3. `Reconciler` coordinates use cases using only interfaces from `interfaces.md`.

Layer model:
1. `cli` layer: parse input and format output.
2. `application` layer: reconciler orchestration.
3. `domain` layer: metadata, identity, invariants.
4. `adapter` layer: filesystem, git, HTTP/OpenAPI, secret stores.

Allowed dependency directions:
1. `cli` -> `application`.
2. `application` -> `domain interfaces`.
3. `adapter` -> `domain interfaces`.
4. `domain` -> no adapter dependency.

Forbidden dependencies:
1. `cli` directly calling adapters.
2. `domain` importing transport or persistence libraries.
3. `repository` directly calling remote server adapters.

## Component Interaction

### Apply Flow
1. CLI parses command and forwards intent to `Reconciler`.
2. `Reconciler` resolves metadata and identity.
3. `Reconciler` builds request from metadata and optional OpenAPI hints.
4. `Reconciler` executes remote mutation.
5. `Reconciler` persists normalized local state.

### Refresh Flow
1. CLI requests remote read through `Reconciler`.
2. `Reconciler` fetches remote resource list/content.
3. `Reconciler` maps remote identity to `ResourceInfo`.
4. `Reconciler` writes local resources with resolved aliases.

### Diff/Explain Flow
1. `Reconciler` loads local and remote states.
2. Compare transforms from metadata are applied.
3. Deterministic diff is returned to CLI formatter.

## Refactor Policy
1. Refactors MUST preserve interface contracts unless `interfaces.md` is updated first.
2. Refactors SHOULD reduce coupling and improve testability.
3. Large refactors MUST be decomposed into reversible steps.
4. Directory moves MUST preserve bounded-context clarity.

## Failure Modes
1. Circular dependency introduction between core and adapters.
2. Hidden side effects leaking from adapters into domain logic.
3. Oversized orchestration functions with mixed concerns.

## Edge Cases
1. Context without remote server configured but local operations still valid.
2. Context with secrets disabled while masked placeholders already exist.
3. Metadata inference available while operation execution remains manual.

## Examples
1. Correct: `reconciler` depends on `ResourceMetadataManager` interface and receives HTTP adapter via dependency injection.
2. Incorrect: CLI command handler directly builds HTTP requests bypassing `Reconciler`.
3. Correct split trigger: one file defines path normalization, git push conflict handling, and metadata mapping; split by responsibility into repository path service, sync service, and metadata locator.
