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
7. `ResourceServer`: optional `server.ResourceServer` instance.

Invariants:
1. fields MUST reference interfaces, not provider concrete types.
2. clients MUST consume `core.DeclarestContext` as their primary dependency entrypoint.

Factory contract:
1. `core.NewDeclarestContext` MUST assemble default provider implementations.
2. `core.NewDeclarestContext` MUST resolve the selected or current context during startup and return an error when resolution or provider wiring fails.
3. clients MUST NOT instantiate provider implementations directly.
4. when `metadata.bundle` is configured and `resource-server.http.openapi` is empty, startup wiring MUST use bundle-provided OpenAPI source hints when available.

Corner case example:
1. when a repository provider satisfies `repository.ResourceStore` but does not satisfy `repository.RepositorySync`, `core.NewDeclarestContext` MUST return an `InternalError`.

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
5. `metadata` MUST define at most one of `base-dir` or `bundle`.

### Type: `config.ContextCatalog`
Represents persisted context catalog in one YAML file.

Required fields:
1. `Contexts`: list of full `config.Context` objects.
2. `CurrentCtx`: active context name mapped to YAML key `current-ctx`.

Optional fields:
1. `DefaultEditor`: default editor command mapped to YAML key `default-editor`.

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
3. exact placeholder directives (for example `{{secret .}}` and `{{resource_format .}}`) MUST remain string values until workflow-specific resolution.

### Type: `metadata.ResourceMetadata`
Holds behavior directives for a resource or collection.

Contract groups:
1. `resourceInfo` identity mapping (`idFromAttribute`, `aliasFromAttribute`) and optional `collectionPath` override.
2. `resourceInfo` secret mapping (`secretInAttributes`).
3. `operationInfo` directives (`createResource`, `updateResource`, `deleteResource`, `getResource`, `compareResources`, `listCollection`).
4. operation wire fields (`path`, `httpMethod`, `query`, `httpHeaders`, `body`, `payload.filterAttributes`, `payload.suppressAttributes`, `payload.jqExpression`, `validate.requiredAttributes`, `validate.assertions[*].{message,jq}`, `validate.schemaRef`), where media headers use `httpHeaders` entries (for example `Accept`, `Content-Type`) instead of separate wire fields.
5. `operationInfo.defaults` transform defaults (`payload.filterAttributes`, `payload.suppressAttributes`, `payload.jqExpression`) and compare transform fields (`compareResources.filterAttributes`, `compareResources.suppressAttributes`, `compareResources.jqExpression`).
6. metadata template helper functions include `{{resource_format .}}` for repository-format-aware values (`json` or `yaml`) in template-rendered metadata string fields.

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

Optional fields:
1. `Validate` (`metadata.OperationValidationSpec`).

### Type: `metadata.OperationValidationSpec`
Represents operation payload validation directives.

Fields:
1. `RequiredAttributes`: required top-level payload attributes.
2. `Assertions`: list of jq assertions that must evaluate truthy.
3. `SchemaRef`: OpenAPI schema reference (`openapi:request-body` or `openapi:#/...`).

### Type: `metadata.ValidationAssertion`
Represents one jq-based payload assertion.

Fields:
1. `Message`.
2. `JQ`.

### Type: `metadata.ResourceOperationSpecInput`
Represents a provider-safe resource descriptor used for operation-spec rendering without requiring `resource.Resource`.

Required fields:
1. `LogicalPath`.
2. `CollectionPath`.
3. `LocalAlias`.
4. `RemoteID`.
5. `Metadata`.
6. `Payload`.

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

### Type: `repository.WorktreeStatusEntry`
Represents one file-level local worktree change entry for verbose repository status output.

Required fields:
1. `Path`.
2. `Staging` (git-style index status code, for example `M` or `?`).
3. `Worktree` (git-style worktree status code, for example `M` or `?`).

### Type: `repository.RepositoryStatusDetailsReader`
Represents optional repository capability for verbose local worktree status inspection.

Method contract:
1. `WorktreeStatus(ctx)` MUST return deterministic path ordering for identical repository state.
2. `WorktreeStatus(ctx)` MUST return only local worktree/index change details and MUST NOT mutate repository state.

### Type: `repository.RepositoryTreeReader`
Represents optional repository capability for directory-tree inspection of local repository layout.

Method contract:
1. `Tree(ctx)` MUST return deterministic lexicographically sorted slash-delimited directory paths relative to the repository root.
2. `Tree(ctx)` MUST return directories only (no files) and MUST exclude the repository root path itself.
3. `Tree(ctx)` MUST exclude hidden control directories (for example `.git`) and reserved metadata namespace directories named `_`.

### Type: `repository.HistoryFilter`
Represents local VCS history query filters for repository backends that support history.

Fields:
1. `MaxCount`.
2. `Author`.
3. `Grep`.
4. `Since`.
5. `Until`.
6. `Paths`.
7. `Reverse`.

Invariants:
1. `Since` and `Until` MAY be nil to indicate open-ended ranges.
2. `Paths` entries MUST be interpreted as repository-relative logical prefixes by history-capable providers.

### Type: `repository.HistoryEntry`
Represents one local VCS commit entry returned by repository history readers.

Required fields:
1. `Hash`.
2. `Author`.
3. `Email`.
4. `Date`.
5. `Subject`.

Optional fields:
1. `Body`.

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
1. `ResourcePath`.
2. `Path`.
3. `Operation`.
4. `Local`.
5. `Remote`.

Field semantics:
1. `ResourcePath` MUST be the normalized absolute logical resource path for the compared resource.
2. `Path` MUST be an RFC 6901 JSON Pointer relative to the resource payload root.
3. `Path == ""` MUST represent a root payload replacement for the resource.

Corner case example:
1. When the entire payload for `/customers/acme` differs (for example remote resource missing), the diff entry MUST use `ResourcePath="/customers/acme"` and `Path=""`.

### Type: `server.ListJQResourceResolver`
Represents logical-path resource resolution callback used by list-operation `jq` `resource("<logical-path>")` calls.

Signature:
1. `func(ctx context.Context, logicalPath string) (resource.Value, error)`.

Invariants:
1. identical `(logicalPath, state)` inputs MUST return deterministic payload output.
2. resolver implementations MUST treat `logicalPath` as normalized absolute path input.

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

Composition:
1. `config.ContextService` composes `config.ContextCatalogWriter`, `config.ContextCatalogReader`, `config.ContextResolver`, and `config.ContextValidator`.

### Interface: `config.ContextCatalogWriter`
Responsibilities:
1. Mutate persisted context catalog entries and active selection.

Method families:
1. `Create/Update/Delete/Rename`.
2. `SetCurrent`.

### Interface: `config.ContextCatalogReader`
Responsibilities:
1. Read persisted context catalog entries and the current selection.

Method families:
1. `List`.
2. `GetCurrent`.

### Interface: `config.ContextCatalogEditor`
Responsibilities:
1. Read and replace the persisted full context catalog as one validated document.

Method families:
1. `GetCatalog`.
2. `ReplaceCatalog`.

### Interface: `config.ContextResolver`
Responsibilities:
1. Resolve one effective context from catalog state plus runtime/environment overrides.

Method families:
1. `ResolveContext`.

### Interface: `config.ContextValidator`
Responsibilities:
1. Validate context configuration semantics and one-of invariants.

Method families:
1. `Validate`.

### Interface: `repository.ResourceStore`
Responsibilities:
1. Persist resources by logical path.
2. Read/list/delete resources.
3. Enforce path safety and layout invariants.

Method families:
1. `Save/Get/Delete(policy)/List(policy)/Exists`.

### Interface: `repository.RepositoryCommitter`
Responsibilities:
1. Create a local VCS commit for repository mutations when supported by the active backend.

Method families:
1. `Commit(message)`.

Invariants:
1. `Commit` MUST return `(false, nil)` when there are no local changes to commit.

### Interface: `repository.RepositoryHistoryReader`
Responsibilities:
1. Read local VCS commit history with deterministic filtering when supported by the active backend.

Method families:
1. `History(filter)`.

### Interface: `repository.RepositoryTreeReader`
Responsibilities:
1. Expose a deterministic local repository directory tree view for CLI inspection workflows.

Method families:
1. `Tree`.

### Interface: `repository.RepositorySync`
Responsibilities:
1. Manage repository lifecycle and synchronization operations.
2. Expose deterministic sync status.
3. Expose destructive local cleanup of uncommitted repository changes.

Method families:
1. Lifecycle: `Init/Refresh/Clean/Reset/Check`.
2. Sync: `Push/SyncStatus`.

### Interface: `metadata.MetadataService`
Responsibilities:
1. Read/write metadata.
2. Resolve layered metadata.
3. Render templates and derive operation directives.

Method families:
1. `Get/Set/Unset`.
2. `ResolveForPath`.
3. `RenderOperationSpec`.

Composition:
1. `metadata.MetadataService` composes `metadata.MetadataStore`, `metadata.MetadataResolver`, and `metadata.OperationSpecRenderer`.

### Interface: `metadata.MetadataStore`
Responsibilities:
1. Read and persist metadata overrides by logical path.

Method families:
1. `Get/Set/Unset`.

### Interface: `metadata.MetadataResolver`
Responsibilities:
1. Resolve layered metadata for a logical path.

Method families:
1. `ResolveForPath`.

### Interface: `metadata.OperationSpecRenderer`
Responsibilities:
1. Render operation specs from metadata using a resource scope.

Method families:
1. `RenderOperationSpec`.

### Interface: `metadata.ResourceOperationSpecRenderer`
Responsibilities:
1. Render operation specs using `metadata.ResourceOperationSpecInput` as the resource descriptor.

Method families:
1. `RenderOperationSpecForResource`.

### Interface: `server.ResourceServer`
Responsibilities:
1. Execute remote CRUD/list operations.
2. Execute request HTTP operations against resource-server endpoints.
3. Resolve OpenAPI hints for operations.
4. Expose typed transport failures.
5. Honor list-operation `jq` resolver context when list transforms call `resource("<logical-path>")`.

Method families:
1. `Get/Create/Update/Delete/List/Exists`.
2. `Request`.
3. `GetOpenAPISpec`.

Optional capability:
1. providers MAY additionally implement `server.AccessTokenProvider` for token inspection workflows.

### Interface: `server.AccessTokenProvider`
Responsibilities:
1. Fetch a resource-server access token using configured auth settings.

Method families:
1. `GetAccessToken`.

Corner case example:
1. Callers MUST treat missing `server.AccessTokenProvider` capability as a validation/configuration error, not a transport error.

Context helper contract:
1. `server.WithListJQResourceResolver` MUST attach one resolver per request context and preserve deterministic cache/cycle-guard state for nested resolution calls.
2. `server.ResolveListJQResource` MUST return `(value, resolved=true, err=nil)` on success, `(nil, resolved=false, err=nil)` when no resolver is attached, and `(nil, resolved=true, err!=nil)` when resolution fails.

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

Composition:
1. `secrets.SecretProvider` composes `secrets.SecretStore`, `secrets.PayloadSecretProcessor`, and `secrets.SecretCandidateDetector`.

### Interface: `secrets.SecretStore`
Responsibilities:
1. Store lifecycle and key/value secret operations.

Method families:
1. `Init`.
2. `Store/Get/Delete/List`.

### Interface: `secrets.PayloadSecretProcessor`
Responsibilities:
1. Mask, resolve, and normalize secret placeholders in payloads.

Method families:
1. `MaskPayload/ResolvePayload`.
2. `NormalizeSecretPlaceholders`.

### Interface: `secrets.SecretCandidateDetector`
Responsibilities:
1. Detect likely plaintext secret candidates in payloads.

Method families:
1. `DetectSecretCandidates`.

### Interface: `orchestrator.Orchestrator`
Responsibilities:
1. Orchestrate repository-store, metadata, server, and secret-provider workflows.
2. Apply desired state to remote systems.
3. Refresh local state from remote systems.
4. Compute explain/diff/list outputs.

Method families:
1. `GetLocal/GetRemote/Request/GetOpenAPISpec/Save/Apply/Create/Update/Delete`.
2. `ListLocal(policy)/ListRemote(policy)`.
3. `Explain/Diff/Template`.

Composition:
1. `orchestrator.Orchestrator` composes `orchestrator.LocalReader`, `orchestrator.RemoteReader`, `orchestrator.OpenAPISpecReader`, `orchestrator.RequestExecutor`, `orchestrator.RepositoryWriter`, `orchestrator.ResourceMutator`, `orchestrator.DiffReader`, and `orchestrator.TemplateRenderer`.

### Interface: `orchestrator.CompletionService`
Responsibilities:
1. Provide read-only local/remote/OpenAPI capabilities for CLI path completion and template-path expansion.

Method families:
1. `GetLocal/ListLocal`.
2. `GetRemote/ListRemote`.
3. `GetOpenAPISpec`.

### Interface: `orchestrator.RemoteReader`
Responsibilities:
1. Provide remote read/list operations without mutation behavior.

Method families:
1. `GetRemote/ListRemote`.

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
5. `resource.json` directive placeholders MUST be exact-string matches (`{{secret ...}}`, `{{resource_format .}}`); embedded interpolation text remains unsupported unless a caller documents broader templating.

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
