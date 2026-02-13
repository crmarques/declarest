# Canonical Interfaces and Contracts

## Purpose
Define the stable contracts for the rebuild so all modules share one source of truth for types, interface boundaries, determinism, and error handling.

## In Scope
1. Canonical interface definitions.
2. Core type contracts and invariants.
3. Error taxonomy and propagation policy.
4. Determinism requirements.
5. Input/output expectations for interface method families.

## Out of Scope
1. Adapter-specific implementation details.
2. Language-specific syntax and framework code.
3. Transport-specific retry and backoff tuning.

## Normative Rules
1. All modules MUST depend on these contracts, not on concrete adapters.
2. `ResourceInfo` MUST replace legacy `ResourceRecord` semantics as the canonical resource descriptor.
3. Interface method outcomes MUST be deterministic for identical inputs and state.
4. Errors MUST be typed using the taxonomy below and preserve root cause context.
5. Cross-module data transfer MUST use declared contracts only.
6. Backward compatibility is not required when it conflicts with cleaner contracts, but changes MUST be documented here first.

## Data Contracts

### Type: `Context`
Represents active runtime context.

Required fields:
- `Name`: selected context name.
- `Environment`: normalized key-value overrides.
- `Repository`: `ResourceRepositoryManager` instance.
- `Metadata`: `ResourceMetadataManager` instance.
- `Server`: optional `ResourceServerManager` instance.
- `Secrets`: optional `SecretManager` instance.

### Interface: `ContextConfigManager`
Responsibilities:
- Manage context definitions.
- Resolve active context.
- Apply env and runtime overrides.
- Validate context configuration.

Method families:
- `Create/Update/Delete/Rename/List`.
- `SetCurrent/GetCurrent`.
- `LoadResolvedConfig`.
- `Validate`.

### Type: `Resource`
Represents JSON-compatible content.

Allowed shapes:
- map object.
- array.
- scalar (`string`, `number`, `bool`, `null`).

Invariants:
- serialization MUST be deterministic.
- numeric handling MUST avoid implicit precision loss.

### Type: `ResourceMetadata`
Holds behavior directives for a resource or collection.

Contract groups:
- identity mapping (`idFromAttribute`, `aliasFromAttribute`).
- operation directives (`path`, `method`, `query`, `headers`).
- transforms (`filter`, `suppress`, `jq`).
- template directives and defaults.

### Type: `ResourceInfo`
Canonical descriptor for resource state and routing.

Required fields:
- `LogicalPath`: normalized repository path.
- `CollectionPath`: normalized parent collection path.
- `LocalAlias`: alias resolved for local storage.
- `RemoteID`: identity used for remote path resolution.
- `ResolvedRemotePath`: concrete remote operation path.
- `Metadata`: fully resolved `ResourceMetadata`.
- `Payload`: `Resource` content.

Invariants:
- logical path MUST be absolute and normalized.
- alias and remote ID resolution MUST be explicit and reproducible.
- resource identity MUST not rely on implicit directory naming alone.

### Interface: `ResourceRepositoryManager`
Responsibilities:
- Persist resources by logical path.
- Read/list/delete/move resources.
- Manage repository synchronization with optional Git.
- Enforce path safety and layout invariants.

Method families:
- resource IO: `Save/Get/Delete/List/Exists/Move`.
- repository lifecycle: `Init/Refresh/Reset/Check`.
- sync: `Push/ForcePush/PullStatus`.

### Interface: `ResourceMetadataManager`
Responsibilities:
- Read/write metadata.
- Resolve layered metadata.
- Render templates and derive operation directives.
- Infer metadata from external hints like OpenAPI.

Method families:
- `Get/Set/Unset`.
- `ResolveForPath`.
- `RenderOperationSpec`.
- `Infer`.

### Interface: `ResourceServerManager`
Responsibilities:
- Execute remote CRUD/list operations.
- Resolve OpenAPI hints for operations.
- Expose typed transport failures.

Method families:
- `Get/Create/Update/Delete/List/Exists`.
- `GetOpenAPISpec`.
- `BuildRequestFromMetadata`.

### Interface: `SecretManager`
Responsibilities:
- Store and retrieve secrets.
- Mask/unmask payload values using placeholders.
- Detect likely secret candidates.

Method families:
- `Init`.
- `Store/Get/Delete/List`.
- `MaskPayload/ResolvePayload`.
- `NormalizeSecretPlaceholders`.
- `DetectSecretCandidates`.

### Interface: `Reconciler`
Responsibilities:
- Orchestrate repo, metadata, server, and secret managers.
- Apply desired state to remote systems.
- Refresh local state from remote systems.
- Compute explain/diff/list outputs.

Method families:
- `Get/Save/Apply/Create/Update/Delete`.
- `ListLocal/ListRemote`.
- `Explain/Diff/Template`.
- `RepoInit/RepoRefresh/RepoPush/RepoReset/RepoCheck`.

## Error Taxonomy and Propagation
Error categories:
1. `ValidationError`: invalid input, shape, path, or config.
2. `NotFoundError`: missing local or remote resource.
3. `ConflictError`: divergence, non-unique identity, or write collision.
4. `AuthError`: authn/authz failure for repository, server, or secret store.
5. `TransportError`: network, TLS, timeout, and protocol issues.
6. `InternalError`: unexpected invariant violations.

Propagation rules:
1. Wrappers MUST preserve category and root cause.
2. User-facing messages SHOULD provide actionable remediation.
3. Secret values MUST never appear in error messages.

## Determinism Requirements
1. Metadata resolution order MUST be deterministic.
2. List outputs MUST be sorted with documented ordering.
3. Equivalent payloads MUST produce stable compare/diff behavior.
4. Template rendering with the same context MUST produce identical outputs.

## Input/Output Expectations
1. All input paths MUST be logical absolute paths.
2. Methods returning collections MUST specify ordering semantics.
3. IO methods MUST declare whether they mutate local, remote, or both.
4. Operations with irreversible side effects MUST support dry-run explanation mode at the reconciler boundary.

## Failure Modes
1. Context resolution failure due to missing or invalid config.
2. Path safety failure due to traversal attempts.
3. Metadata render failure due to unresolved template variables.
4. Secret resolution failure due to missing keys or store unavailability.
5. Remote operation failure due to transport or auth errors.

## Edge Cases
1. Alias collision across sibling resources in the same collection.
2. Metadata directives present in collection scope but missing at resource scope.
3. Resource payloads with mixed numeric representations.
4. Partial context configuration with optional managers absent.

## Examples
1. A `Reconciler.Apply` call uses `ResourceMetadataManager.ResolveForPath`, then `ResourceServerManager.BuildRequestFromMetadata`, then persists final local state through `ResourceRepositoryManager` only after successful remote mutation.
2. A `SecretManager.MaskPayload` call stores extracted values and replaces them with placeholders before `ResourceRepositoryManager.Save`.
3. A `ContextConfigManager.LoadResolvedConfig` call merges persisted context and environment overrides, then validates before building `Context`.
