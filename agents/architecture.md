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
8. Public interfaces and shared types MUST remain stable and centrally documented in `agents/interfaces.md`.
9. Legacy disorganization patterns MUST NOT influence new architecture.
10. Dependencies MUST flow inward toward domain contracts, never outward from core/domain packages to adapter implementations.
11. `reconciler.Reconciler` MUST be the only orchestrator combining repository, metadata, server, and secret concerns.
12. Architecture and package layout MUST follow the implementation language community standards.
13. Go projects MUST keep command entrypoints in `cmd/*` and non-public implementation details in `internal/*`.
14. Bash-based test/support tooling MUST follow common shell project practices and lintable structure conventions.

## Layer Model
1. `cmd/declarest`: executable entrypoint only.
2. `internal/cli`: command parsing, validation, and output formatting.
3. public domain packages: `core`, `ctx`, `resource`, `metadata`, `repository`, `server`, `secrets`, `reconciler`.
4. private adapter implementations: `internal/adapters/ctx/file`, `internal/adapters/repository/*`, `internal/adapters/server/*`, `internal/adapters/secrets/*`, `internal/adapters/noop/*`.
5. private composition root: `internal/app`.

## Allowed Dependency Directions
1. `cmd/declarest` -> `internal/app`, `internal/cli`.
2. `internal/cli/*` -> `internal/cli/common`, `ctx`, `reconciler`.
3. `reconciler` -> `repository`, `metadata`, `server`, `secrets`, `resource`, `core`.
4. `ctx` -> `repository`, `metadata`, optional `server`, optional `secrets`.
5. `internal/app` -> `internal/adapters/ctx/file`, `internal/adapters/noop/*`, `internal/cli/common`.
6. `internal/adapters/*` -> owner package interfaces/types.

## Forbidden Dependencies
1. `internal/cli` directly importing implementation adapter packages (`internal/adapters/repository/*`, `internal/adapters/server/*`, `internal/adapters/secrets/*`, `internal/adapters/noop/*`).
2. `core`, `resource`, `metadata`, `reconciler` importing `internal/cli` or `cmd`.
3. `repository` directly calling `server` adapters.
4. `internal` packages imported by non-module consumers.

## Component Interaction

### Apply Flow
1. CLI parses command and forwards intent to `reconciler.Reconciler`.
2. Reconciler resolves metadata and identity.
3. Reconciler builds request from metadata and optional OpenAPI hints.
4. Reconciler executes remote mutation.
5. Reconciler persists normalized local state.

### Refresh Flow
1. CLI requests remote read through reconciler.
2. Reconciler fetches remote list/content.
3. Reconciler maps remote identity to `resource.Info`.
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
1. Circular dependency introduction between public contracts and adapter implementations.
2. Hidden side effects leaking from adapters into domain logic.
3. Oversized orchestration functions with mixed concerns.

## Edge Cases
1. Context without remote server configured but local operations still valid.
2. Context with secrets disabled while masked placeholders already exist.
3. Metadata inference available while operation execution remains manual.

## Examples
1. Correct: `reconciler` depends on `server.Manager` interface and receives HTTP adapter via dependency injection.
2. Incorrect: CLI command handler imports `internal/adapters/server/http` and issues requests directly.
3. Correct split trigger: one file defines path normalization, git push conflict handling, and metadata mapping; split by responsibility into repository path service, sync service, and metadata locator.
