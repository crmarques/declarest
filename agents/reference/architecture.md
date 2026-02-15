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
8. Public interfaces and shared types MUST remain stable and centrally documented in `agents/reference/interfaces.md`.
9. Legacy disorganization patterns MUST NOT influence new architecture.
10. Dependencies MUST flow inward toward domain contracts in domain packages; only the composition root package (`core`) may depend outward on provider implementations for wiring.
11. Provider selection and concrete implementation construction MUST happen only in `core`.
12. `reconciler.ResourceReconciler` MUST be the only orchestrator combining repository, metadata, server, and secret concerns.
13. Architecture and package layout MUST follow the implementation language community standards.
14. Go projects MUST keep command entrypoints in `cmd/*` and non-public implementation details in `internal/*`.
15. Bash-based test/support tooling MUST follow common shell project practices and lintable structure conventions.

## Layer Model
1. `cmd/declarest`: executable entrypoint only.
2. `internal/cli`: command parsing, validation, and output formatting.
3. public domain packages: `config`, `resource`, `metadata`, `repository`, `server`, `secrets`, `reconciler`.
4. public shared primitives: `faults`.
5. private provider implementations: `internal/providers/config/file`, `internal/providers/metadata/stub`, `internal/providers/reconciler/stub`, `internal/providers/repository/*`, `internal/providers/server/*`, `internal/providers/secrets/*`, `internal/providers/support/notimpl`.
6. public composition root: `core`.

## Allowed Dependency Directions
1. `cmd/declarest` -> `core`, `internal/cli`.
2. `internal/cli/*` -> `internal/cli/common`, `config`, `reconciler`.
3. `reconciler` -> `repository`, `metadata`, `server`, `secrets`, `resource`.
4. `config` -> standard library only.
5. `core` -> `internal/providers/config/file`, `internal/providers/reconciler/stub`, `internal/providers/metadata/stub`, `internal/providers/repository/*`, `internal/providers/server/*`, `internal/providers/secrets/*`.
6. `internal/providers/*` -> owner package interfaces/types.

## Forbidden Dependencies
1. `internal/cli` directly importing implementation provider packages (`internal/providers/config/file`, `internal/providers/metadata/stub`, `internal/providers/reconciler/stub`, `internal/providers/repository/*`, `internal/providers/server/*`, `internal/providers/secrets/*`).
2. `core`, `resource`, `metadata`, `reconciler` importing `internal/cli` or `cmd`.
3. `repository` directly calling `server` providers.
4. `internal` packages imported by non-module consumers.
5. `internal/providers/config/file` importing sibling provider implementation packages.

## Component Interaction

### Apply Flow
1. CLI parses command and forwards intent to `reconciler.ResourceReconciler`.
2. Reconciler resolves metadata and identity.
3. Reconciler builds request from metadata and optional OpenAPI hints.
4. Reconciler executes remote mutation.
5. Reconciler persists normalized local state.

### Refresh Flow
1. CLI requests remote read through reconciler.
2. Reconciler fetches remote list/content.
3. Reconciler maps remote identity to `resource.Resource`.
4. Reconciler writes local resources with resolved aliases.

### Diff/Explain Flow
1. Reconciler loads local and remote states.
2. Compare transforms from metadata are applied.
3. Deterministic diff is returned to CLI formatter.

## Refactor Policy
1. Refactors MUST preserve interface contracts unless `interfaces.md` is updated first.
2. Refactors SHOULD reduce coupling and improve testability.
3. Large refactors MUST be decomposed into reversible steps when not explicitly requested as big-bang.
4. Directory moves MUST preserve bounded-context clarity.

## Failure Modes
1. Circular dependency introduction between public contracts and provider implementations.
2. Hidden side effects leaking from providers into domain logic.
3. Oversized orchestration functions with mixed concerns.

## Edge Cases
1. Context without remote server configured but local operations still valid.
2. Context with secrets disabled while masked placeholders already exist.
3. Metadata inference available while operation execution remains manual.

## Examples
1. Correct: `reconciler` depends on `server.ResourceServerManager` interface and receives HTTP provider via dependency injection.
2. Incorrect: CLI command handler imports `internal/providers/server/http` and issues requests directly.
3. Correct split trigger: one file defines path normalization, git push conflict handling, and metadata mapping; split by responsibility into repository path service, sync service, and metadata locator.
