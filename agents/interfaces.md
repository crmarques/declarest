# Canonical Interfaces and Contracts

## Purpose
Define stable contracts for manager interfaces, shared types, determinism, and error handling across bounded-context packages.

## In Scope
1. Canonical interface definitions.
2. Core type contracts and invariants.
3. Error taxonomy and propagation policy.
4. Determinism requirements.
5. Input/output expectations for interface method families.

## Out of Scope
1. Adapter-specific implementation details.
2. Language-specific framework wiring.
3. Transport retry and backoff tuning.

## Normative Rules
1. Modules MUST depend on interfaces from owner packages, not concrete implementations.
2. `resource.Info` MUST be the canonical resource descriptor.
3. Interface outcomes MUST be deterministic for identical inputs and state.
4. Errors MUST use the taxonomy below and preserve root cause context.
5. Cross-package data transfer MUST use declared contracts only.
6. Contract changes MUST be documented here before implementation changes.

## Data Contracts

### Type: `ctx.Runtime`
Represents active runtime context.

Required fields:
1. `Name`: selected context name.
2. `Environment`: normalized key-value overrides.
3. `Repository`: `repository.Manager` instance.
4. `Metadata`: `metadata.Manager` instance.
5. `Server`: optional `server.Manager` instance.
6. `Secrets`: optional `secrets.Manager` instance.

### Type: `ctx.Config`
Represents persisted context configuration.

Required fields:
1. `Name`.
2. `Repository` typed configuration object.

Optional fields:
1. `ManagedServer` typed server configuration object.
2. `SecretStore` typed secret store configuration object.
3. `Preferences` settings map.
4. `Metadata` typed metadata configuration object.

YAML key contract:
1. keys MUST use kebab-case.
2. unknown keys MUST fail strict decoding.

One-of invariants:
1. `repository` MUST define exactly one of `git` or `filesystem`.
2. `managed-server.http.auth` MUST define exactly one of `oauth2`, `basic-auth`, `bearer-token`, `custom-header`.
3. `secret-store` MUST define exactly one of `file` or `vault`.
4. `secret-store.file` MUST define exactly one of `key`, `key-file`, `passphrase`, `passphrase-file`.

### Type: `ctx.Catalog`
Represents persisted context catalog in one YAML file.

Required fields:
1. `Contexts`: list of full `ctx.Config` objects.
2. `CurrentCtx`: active context name mapped to YAML key `current-ctx`.

Invariants:
1. context names MUST be unique and non-empty.
2. `CurrentCtx` MUST reference an existing context when contexts are present.

### Type: `core.Resource`
Represents JSON-compatible content.

Allowed shapes:
1. map object.
2. array.
3. scalar (`string`, `number`, `bool`, `null`).

Invariants:
1. serialization MUST be deterministic.
2. numeric handling MUST avoid implicit precision loss.

### Type: `metadata.ResourceMetadata`
Holds behavior directives for a resource or collection.

Contract groups:
1. identity mapping (`idFromAttribute`, `aliasFromAttribute`).
2. operation directives (`path`, `method`, `query`, `headers`).
3. transforms (`filter`, `suppress`, `jq`).
4. template directives and defaults.

### Type: `metadata.OperationSpec`
Represents resolved operation request intent.

Required fields:
1. `Method`.
2. `Path`.
3. `Query`.
4. `Headers`.
5. `Accept`.
6. `ContentType`.
7. `Body`.

### Type: `resource.Info`
Canonical descriptor for resource state and routing.

Required fields:
1. `LogicalPath`: normalized repository path.
2. `CollectionPath`: normalized parent collection path.
3. `LocalAlias`: alias resolved for local storage.
4. `RemoteID`: identity used for remote path resolution.
5. `ResolvedRemotePath`: concrete remote operation path.
6. `Metadata`: fully resolved `metadata.ResourceMetadata`.
7. `Payload`: `core.Resource` content.

Invariants:
1. logical path MUST be absolute and normalized.
2. alias and remote ID resolution MUST be explicit and reproducible.
3. resource identity MUST not rely on implicit directory naming alone.

### Type: `resource.DiffEntry`
Represents one deterministic compare output item.

Required fields:
1. `Path`.
2. `Operation`.
3. `Local`.
4. `Remote`.

## Interface Contracts

### Interface: `ctx.Manager`
Responsibilities:
1. Manage context definitions.
2. Resolve active context.
3. Apply environment and runtime overrides.
4. Validate context configuration.

Method families:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `LoadResolvedConfig`.
4. `Validate`.

### Interface: `repository.Manager`
Responsibilities:
1. Persist resources by logical path.
2. Read/list/delete/move resources.
3. Manage repository synchronization with optional Git.
4. Enforce path safety and layout invariants.

Method families:
1. Resource IO: `Save/Get/Delete/List/Exists/Move`.
2. Repository lifecycle: `Init/Refresh/Reset/Check`.
3. Sync: `Push/ForcePush/PullStatus`.

### Interface: `metadata.Manager`
Responsibilities:
1. Read/write metadata.
2. Resolve layered metadata.
3. Render templates and derive operation directives.
4. Infer metadata from external hints such as OpenAPI.

Method families:
1. `Get/Set/Unset`.
2. `ResolveForPath`.
3. `RenderOperationSpec`.
4. `Infer`.

### Interface: `server.Manager`
Responsibilities:
1. Execute remote CRUD/list operations.
2. Resolve OpenAPI hints for operations.
3. Expose typed transport failures.

Method families:
1. `Get/Create/Update/Delete/List/Exists`.
2. `GetOpenAPISpec`.
3. `BuildRequestFromMetadata`.

### Interface: `secrets.Manager`
Responsibilities:
1. Store and retrieve secrets.
2. Mask/unmask payload values using placeholders.
3. Detect likely secret candidates.

Method families:
1. `Init`.
2. `Store/Get/Delete/List`.
3. `MaskPayload/ResolvePayload`.
4. `NormalizeSecretPlaceholders`.
5. `DetectSecretCandidates`.

### Interface: `reconciler.Reconciler`
Responsibilities:
1. Orchestrate repository, metadata, server, and secret managers.
2. Apply desired state to remote systems.
3. Refresh local state from remote systems.
4. Compute explain/diff/list outputs.

Method families:
1. `Get/Save/Apply/Create/Update/Delete`.
2. `ListLocal/ListRemote`.
3. `Explain/Diff/Template`.
4. `RepoInit/RepoRefresh/RepoPush/RepoReset/RepoCheck`.

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
1. Input paths MUST be logical absolute paths.
2. Methods returning collections MUST specify ordering semantics.
3. IO methods MUST declare whether they mutate local, remote, or both.
4. Irreversible operations MUST support dry-run explanation mode at reconciler boundary.

## Failure Modes
1. Context resolution failure due to missing or invalid config.
2. Path safety failure due to traversal attempts.
3. Metadata render failure due to unresolved template variables.
4. Secret resolution failure due to missing keys or store unavailability.
5. Remote operation failure due to transport or auth errors.

## Edge Cases
1. Alias collision across sibling resources in one collection.
2. Metadata directives present in collection scope but missing at resource scope.
3. Resource payloads with mixed numeric representations.
4. Partial context configuration with optional managers absent.

## Examples
1. A `reconciler.Reconciler.Apply` call resolves metadata, builds request intent, executes remote mutation, and persists local state only after successful remote mutation.
2. A `secrets.Manager.MaskPayload` call stores extracted values and replaces them with placeholders before `repository.Manager.Save`.
3. A `ctx.Manager.LoadResolvedConfig` call merges persisted context and environment overrides, then validates before building `ctx.Runtime`.
