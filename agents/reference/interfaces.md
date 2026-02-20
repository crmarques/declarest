# Canonical Interfaces and Contracts

## Purpose
Define stable contracts for shared interfaces, types, determinism, and error handling across bounded-context packages.

## In Scope
1. Canonical interface definitions.
2. Core type contracts and invariants.
3. Error taxonomy and propagation policy.
4. Determinism requirements.
5. Input/output expectations for interface method families.

## Out of Scope
1. Provider-specific implementation details.
2. Language-specific framework wiring.
3. Transport retry and backoff tuning.

## Normative Rules
1. Modules MUST depend on interfaces from owner packages, not concrete implementations.
2. `resource.Resource` MUST be the canonical resource descriptor.
3. Interface outcomes MUST be deterministic for identical inputs and state.
4. Errors MUST use the taxonomy below and preserve root cause context.
5. Cross-package data transfer MUST use declared contracts only.
6. Contract changes MUST be documented here before implementation changes.

## Data Contracts

### Type: `config.ContextSelection`
Represents context-resolution inputs.

Fields:
1. `Name`: explicit context name, optional.
2. `Overrides`: runtime key-value overrides.

### Type: `core.DeclarestContext`
Represents client-facing application state assembled at startup.

Required fields:
1. `Contexts`: `config.ContextService` instance.
2. `Orchestrator`: `orchestrator.Orchestrator` instance.
3. `ResourceStore`: `repository.ResourceStore` instance.
4. `RepositorySync`: `repository.RepositorySync` instance.
5. `Metadata`: optional `metadata.MetadataService` instance.
6. `Secrets`: optional `secrets.SecretProvider` instance.

Invariants:
1. fields MUST reference interfaces, not provider concrete types.
2. clients MUST consume `core.DeclarestContext` as their primary dependency entrypoint.
3. `Repository` MAY be exposed only as temporary compatibility union while callers migrate to split repository contracts.

Factory contract:
1. `core.NewDeclarestContext` MUST assemble default provider implementations.
2. `core.NewDeclarestContext` MUST resolve the selected or current context during startup and return an error when resolution or provider wiring fails.
3. clients MUST NOT instantiate provider implementations directly.

### Type: `core.BootstrapConfig`
Represents startup wiring inputs.

Fields:
1. `ContextCatalogPath`: optional explicit context catalog path.

### Type: `orchestrator.DefaultOrchestrator`
Represents the default concrete orchestrator assembled by the composition root.

Required fields:
1. `Repository`: `repository.ResourceStore` instance.
2. `Metadata`: `metadata.MetadataService` instance.
3. `Server`: optional `server.ResourceServer` instance.
4. `Secrets`: optional `secrets.SecretProvider` instance.

### Type: `config.Context`
Represents persisted context configuration.

Required fields:
1. `Name`.
2. `Repository` typed configuration object.
3. `ResourceServer` typed server configuration object.

Optional fields:
1. `SecretStore` typed secret store configuration object.
2. `Preferences` settings map.
3. `Metadata` typed metadata configuration object.

YAML key contract:
1. keys MUST use kebab-case.
2. unknown keys MUST fail strict decoding.

One-of invariants:
1. `repository` MUST define exactly one of `git` or `filesystem`.
2. `resource-server.http.auth` MUST define exactly one of `oauth2`, `basic-auth`, `bearer-token`, `custom-header`.
3. `secret-store` MUST define exactly one of `file` or `vault`.
4. `secret-store.file` MUST define exactly one of `key`, `key-file`, `passphrase`, `passphrase-file`.

### Type: `config.ContextCatalog`
Represents persisted context catalog in one YAML file.

Required fields:
1. `Contexts`: list of full `config.Context` objects.
2. `CurrentCtx`: active context name mapped to YAML key `current-ctx`.

Invariants:
1. context names MUST be unique and non-empty.
2. `CurrentCtx` MUST reference an existing context when contexts are present.

### Type: `resource.Value`
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
1. `resourceInfo` identity mapping (`idFromAttribute`, `aliasFromAttribute`) and optional `collectionPath` override.
2. `resourceInfo` secret mapping (`secretInAttributes`).
3. `operationInfo` directives (`createResource`, `updateResource`, `deleteResource`, `getResource`, `compareResources`, `listCollection`).
4. operation fields (`path`, `method`, `query`, `headers`, `accept`, `contentType`, `body`, `filter`, `suppress`, `jq`).
5. `operationInfo.defaults` transform defaults (`filter`, `suppress`, `jq`).

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

### Type: `metadata.Operation`
Represents the supported operation identifiers.

Allowed values:
1. `get`.
2. `create`.
3. `update`.
4. `delete`.
5. `list`.
6. `compare`.

### Type: `metadata.InferenceRequest`
Represents metadata inference behavior options.

Fields:
1. `Apply`.
2. `Recursive`.

### Type: `repository.ResetPolicy`
Represents repository reset behavior options.

Fields:
1. `Hard`.

### Type: `repository.PushPolicy`
Represents repository push behavior options.

Fields:
1. `Force`.

### Type: `repository.ListPolicy`
Represents repository list behavior options.

Fields:
1. `Recursive`.

### Type: `repository.DeletePolicy`
Represents repository delete behavior options.

Fields:
1. `Recursive`.

### Type: `repository.SyncReport`
Represents repository local/remote sync status.

Required fields:
1. `State`.
2. `Ahead`.
3. `Behind`.
4. `HasUncommitted`.

### Type: `orchestrator.DeletePolicy`
Represents local delete behavior options.

Fields:
1. `Recursive`.

### Type: `orchestrator.ListPolicy`
Represents local/remote list behavior options.

Fields:
1. `Recursive`.

### Type: `resource.Resource`
Canonical descriptor for resource state and routing.

Required fields:
1. `LogicalPath`: normalized repository path.
2. `CollectionPath`: normalized parent collection path.
3. `LocalAlias`: alias resolved for local storage.
4. `RemoteID`: identity used for remote path resolution.
5. `ResolvedRemotePath`: concrete remote operation path.
6. `Metadata`: fully resolved `metadata.ResourceMetadata`.
7. `Payload`: `resource.Value` content.

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

### Interface: `config.ContextService`
Responsibilities:
1. Manage context definitions.
2. Resolve active context.
3. Apply environment and runtime overrides.
4. Validate context configuration.

Method families:
1. `Create/Update/Delete/Rename/List`.
2. `SetCurrent/GetCurrent`.
3. `ResolveContext`.
4. `Validate`.

### Interface: `repository.ResourceStore`
Responsibilities:
1. Persist resources by logical path.
2. Read/list/delete resources.
3. Enforce path safety and layout invariants.

Method families:
1. `Save/Get/Delete(policy)/List(policy)/Exists`.

### Interface: `repository.RepositorySync`
Responsibilities:
1. Manage repository lifecycle and synchronization operations.
2. Expose deterministic sync status.

Method families:
1. Lifecycle: `Init/Refresh/Reset/Check`.
2. Sync: `Push/SyncStatus`.

### Interface: `repository.ResourceRepository`
Responsibilities:
1. Compatibility union for consumers that still require one dependency.

Method families:
1. Embeds `repository.ResourceStore`.
2. Embeds `repository.RepositorySync`.

### Interface: `metadata.MetadataService`
Responsibilities:
1. Read/write metadata.
2. Resolve layered metadata.
3. Render templates and derive operation directives.

Method families:
1. `Get/Set/Unset`.
2. `ResolveForPath`.
3. `RenderOperationSpec`.

### Interface: `server.ResourceServer`
Responsibilities:
1. Execute remote CRUD/list operations.
2. Execute ad-hoc HTTP operations against resource-server endpoints.
3. Resolve OpenAPI hints for operations.
4. Expose typed transport failures.

Method families:
1. `Get/Create/Update/Delete/List/Exists`.
2. `AdHoc`.
3. `GetOpenAPISpec`.

### Interface: `secrets.SecretProvider`
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

### Interface: `orchestrator.Orchestrator`
Responsibilities:
1. Orchestrate repository-store, metadata, server, and secret-provider workflows.
2. Apply desired state to remote systems.
3. Refresh local state from remote systems.
4. Compute explain/diff/list outputs.

Method families:
1. `GetLocal/GetRemote/AdHoc/GetOpenAPISpec/Save/Apply/Create/Update/Delete`.
2. `ListLocal(policy)/ListRemote(policy)`.
3. `Explain/Diff/Template`.

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
4. Irreversible operations MUST support dry-run explanation mode at orchestrator boundary.

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
1. A `orchestrator.Orchestrator.Apply` call resolves metadata, builds request intent, and executes a remote create/update mutation derived from repository desired state.
2. A `secrets.SecretProvider.MaskPayload` call stores extracted values and replaces them with placeholders before `repository.ResourceStore.Save`.
3. A `config.ContextService.ResolveContext` call merges persisted context and environment overrides, then validates and returns one resolved `config.Context`.
