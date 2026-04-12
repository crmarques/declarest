# Canonical Interfaces and Contracts

**Quick navigation:** [Data Contracts (Types)](#data-contracts) | [Interface Contracts](#interface-contracts) | [Error Taxonomy](#error-taxonomy-and-propagation) | [Determinism](#determinism-requirements)

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

### Type: `bootstrap.Session`
Represents client-facing application state assembled at startup.

Required fields:
1. `Contexts`: `config.ContextService` instance.
2. `Orchestrator`: `orchestrator.Orchestrator` instance.
3. `Services`: `orchestrator.ServiceAccessor` instance — provides access to individual domain services (`RepositoryStore()`, `RepositorySync()`, `MetadataService()`, `SecretProvider()`, `ManagedServiceClient()`).

Invariants:
1. fields MUST reference interfaces, not provider concrete types.
2. clients MUST consume `bootstrap.Session` as their primary dependency entrypoint.
3. clients that need direct access to sub-services (metadata inspection, secret management) MUST use `Services` via the `orchestrator.ServiceAccessor` interface rather than instantiating providers directly.

Factory contract:
1. `bootstrap.NewSession` MUST assemble default provider implementations.
2. `bootstrap.NewSession` MUST resolve the selected or current context during startup and return an error when resolution or provider wiring fails.
3. clients MUST NOT instantiate provider implementations directly.
4. when `metadata.bundle` or `metadata.bundleFile` is configured and `managedService.http.openapi` is empty, startup wiring MUST use bundle-provided OpenAPI source hints when available.

Corner case example:
1. when a repository provider satisfies `repository.ResourceStore` but does not satisfy `repository.RepositorySync`, `bootstrap.NewSession` MUST return an `InternalError`.

### Type: `bootstrap.BootstrapConfig`
Represents startup wiring inputs.

Fields:
1. `ContextCatalogPath`: optional explicit context catalog path.

### Type: `orchestrator.DefaultOrchestrator`
Represents the default concrete orchestrator assembled by the composition root.

Required fields:
1. `Repository`: `repository.ResourceStore` instance.
2. `Metadata`: `metadata.MetadataService` instance.
3. `Server`: optional `managedservice.ManagedServiceClient` instance.
4. `Secrets`: optional `secrets.SecretProvider` instance.

### Type: `config.Context`
Represents persisted context configuration.

Required fields:
1. `Name`.

Optional fields:
1. `Repository` typed configuration object.
2. `ManagedService` typed server configuration object.
3. `SecretStore` typed secret store configuration object.
4. `Preferences` settings map.
5. `Metadata` typed metadata configuration object.
6. `managedService.http.healthCheck` optional probe target used by `server check`.
7. Metadata JSON Pointer references (`requiredAttributes[*]`, `secretAttributes[*]`, `externalizedAttributes[*].path`, transform `selectAttributes`/`excludeAttributes`, compare exclude/select fields, and `validate.requiredAttributes[*]`) MUST use RFC 6901 JSON Pointer strings; `resource.id` and `resource.alias` are identity template strings that MAY embed one or more JSON Pointer expressions, MUST accept raw JSON Pointer shorthand such as `/id`, and metadata string-template fields that render payload/context attributes MUST accept canonical RFC 6901 placeholder form such as `{{/id}}` plus one-level shorthand such as `{{id}}`.

User-config key contract:
1. persisted keys MUST use camelCase.
2. on-disk catalog readers MUST reject legacy aliases and any non-canonical persisted keys.
3. unknown keys MUST fail strict decoding.

One-of invariants:
1. `config.Context` MUST define at least one of `repository` or `managedService`.
2. `repository` MUST define exactly one of `git` or `filesystem` when configured.
3. `managedService.http.auth` MUST define exactly one of `oauth2`, `basic`, or `customHeaders` when `managedService.http` is configured.
4. `secretStore` MUST define exactly one of `file` or `vault`.
5. `secretStore.file` MUST define exactly one of `key`, `keyFile`, `passphrase`, `passphraseFile`.
6. `metadata` MUST define at most one of `baseDir`, `bundle`, or `bundleFile`.
7. `managedService.http.proxy` MAY define any subset of `http`, `https`, `noProxy`, and `auth`; an empty block explicitly disables inherited or environment proxy resolution, and effective runtime proxying requires at least one resolved proxy URL after environment merge.
8. `managedService.http.proxy.auth` MUST define `basic.credentialsRef` when configured.
9. `repository.git.remote.auth` MUST define exactly one of `basic`, `ssh`, or `accessKey`.
10. `secretStore.vault.auth` MUST define exactly one of `token`, `password`, or `appRole`.
11. `managedService.http.requestThrottling` MUST define at least one of `maxConcurrentRequests` or `requestsPerSecond` when configured.
12. `managedService.http.requestThrottling.queueSize` MUST NOT be set unless `maxConcurrentRequests` is set.
13. `managedService.http.requestThrottling.burst` MUST NOT be set unless `requestsPerSecond` is set.

### Type: `config.Credential`
Represents one reusable username/password definition stored at catalog scope.

Fields:
1. `Name`: unique credential identifier.
2. `Username`: `config.CredentialValue`.
3. `Password`: `config.CredentialValue`.

Invariants:
1. credential names MUST be unique within one catalog.
2. persisted context components MUST reference catalog credentials by `credentialsRef.name` instead of inlining reusable username/password pairs.

### Type: `config.CredentialValue`
Represents one credential attribute that is either literal or prompted at runtime.

Fields:
1. literal string value, or
2. prompt object `{prompt: true, persistInSession?: bool}`.

Invariants:
1. `persistInSession: true` MUST mean reuse for later `declarest` commands only within one shell session that exported `DECLAREST_PROMPT_AUTH_SESSION_ID`.
2. new prompt-auth session cache files MUST be written only under `XDG_RUNTIME_DIR/declarest/prompt-auth/`.
3. when `XDG_RUNTIME_DIR` or `DECLAREST_PROMPT_AUTH_SESSION_ID` is unavailable, prompted values MAY be reused only inside the current `declarest` process and MUST NOT be persisted under the home directory.

### Type: `config.CredentialsRef`
Represents a placeholder to inject a named catalog credential into one auth block.

Fields:
1. `Name`: referenced catalog credential name.

Invariants:
1. when a context component defines `credentialsRef`, runtime MUST inject the referenced credential object into that location while omitting the credential `name` field.
2. referenced prompt-backed attributes MUST prompt only when the owning component first needs that value at runtime.
3. non-interactive execution MUST fail when a required prompt-backed attribute has no cached session value.

### Type: `config.ContextCatalog`
Represents persisted context catalog in one YAML file.

Required fields:
1. `Contexts`: list of full `config.Context` objects.
2. `CurrentContext`: active context name mapped to persisted key `currentContext`.

Optional fields:
1. `DefaultEditor`: default editor command mapped to persisted key `defaultEditor`.
2. `Credentials`: reusable catalog-scoped credentials referenced by `credentialsRef`.

Invariants:
1. context names MUST be unique and non-empty.
2. `CurrentContext` MUST reference an existing context when contexts are present.
3. every resolved `credentialsRef.name` MUST reference an entry in `ContextCatalog.Credentials`.

### Type: `resource.Value`
Represents structured or opaque resource content.

Allowed shapes:
1. map object.
2. array.
3. scalar (`string`, `number`, `bool`, `null`).
4. `resource.BinaryValue`.

Invariants:
1. serialization MUST be deterministic.
2. numeric handling MUST avoid implicit precision loss.
3. exact placeholder directives (for example `{{secret .}}`, `{{payload_type .}}`, `{{payload_media_type .}}`, `{{payload_extension .}}`, and metadata-configured externalized-attribute include placeholders) MUST remain string values until workflow-specific resolution.
4. opaque binary payloads MUST use `resource.BinaryValue` instead of raw `[]byte`.
5. when a whole resource payload is persisted as the exact placeholder `{{secret .}}`, workflow-specific resolution MUST treat `.` as the root payload scope for that logical path.
6. orchestrator and CLI workflows MAY compose one effective merge-capable object payload from resolved metadata `resource.defaults` plus raw `resource.<ext>` repository payloads; metadata-managed defaults artifacts MUST be referenced only through exact include placeholders such as `{{include defaults.yaml}}` or `{{include defaults-prod.properties}}`, arrays MUST replace, explicit `resource.<ext>` values MUST win, supported defaults artifact codecs are `json|yaml|yml|properties`, and non-object or unsupported payload types MUST fail defaults validation instead of being merged implicitly.

### Type: `resource.BinaryValue`
Represents opaque binary payload content.

Required fields:
1. `Bytes`.

Invariants:
1. `Bytes` MUST be treated as opaque content and MUST NOT assume UTF-8 text semantics.
2. structured CLI output MUST serialize `resource.BinaryValue` as a base64 wrapper object rather than raw byte arrays.

### Type: `metadata.ResourceMetadata`
Holds behavior directives for a resource or collection.

Contract groups:
1. top-level selector mapping (`selector.descendants`) for persisted collection-selector scope; it MUST be accepted only on collection metadata and MUST NOT be treated as a mergeable resolved directive.
2. `resource` identity mapping (`id`, `alias`, `requiredAttributes`), optional `remoteCollectionPath` override, and optional `format` directive; `format` MUST accept the supported payload formats plus `any`, drives default repository/request media behavior when concrete, and when omitted `id` and `alias` default to `/id` for identity resolution.
3. `resource` secret mapping (`secret`, `secretAttributes`).
4. `resource` defaults mapping (`defaults.mode`, `defaults.useProfiles`, `defaults.value`, `defaults.profiles`) for metadata-native defaults layering; `defaults.value` and `defaults.profiles[*]` MUST accept either one structured object or one exact include placeholder pointing to deterministic selector-local files named `defaults.<ext>` or `defaults-<profile>.<ext>`.
5. `resource` externalized attribute mapping (`externalizedAttributes[*].{path,file,template,mode,saveBehavior,renderBehavior,enabled}`).
6. `operations` directives (`create`, `update`, `delete`, `get`, `compare`, `list`).
7. operation wire fields (`path`, `method`, `query`, `headers`, `body`, `transforms[*].{selectAttributes,excludeAttributes,jqExpression}`, `validate.requiredAttributes`, `validate.assertions[*].{message,jq}`, `validate.schemaRef`), where attribute references use RFC 6901 JSON Pointer strings and media headers use `headers` entries (for example `Accept`, `Content-Type`) instead of separate wire fields.
8. `operations.defaults.transforms` is an ordered pipeline applied before operation-specific `transforms`.
9. metadata template helper functions include `{{payload_type .}}`, `{{payload_media_type .}}`, and `{{payload_extension .}}`, plus descendant-scope helpers `{{/descendantPath}}` and `{{/descendantCollectionPath}}` when a descendant-enabled collection selector matched the handled target; `resource.id` and `resource.alias` remain segment-safe and MUST NOT resolve to slashful values.

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

### Type: `managedservice.RequestSpec`
Represents one concrete managed-service HTTP request.

Required fields:
1. `Method`.
2. `Path`.

Optional fields:
1. `Query`.
2. `Headers`.
3. `Accept`.
4. `ContentType`.
5. `Body`.

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
5. `PayloadDescriptor`.
6. `Metadata`.
7. `Payload`.

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

### Type: `orchestrator.ApplyPolicy`
Represents apply behavior options.

Fields:
1. `Force`.

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

### Type: `managedservice.ListJQResourceResolver`
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

Invariants:
1. `Get` MUST return only the raw persisted payload for one logical resource from `resource.<ext>` and MUST NOT merge metadata defaults into that repository response.
2. `Save` MUST persist only the raw repository payload for `resource.<ext>` and MUST NOT write metadata defaults artifacts or flatten metadata-resolved defaults into repository-owned payload discovery.

### Interface: `repository.ResourceArtifactStore`
Responsibilities:
1. Persist sidecar files for one logical resource alongside the canonical payload file.
2. Read sidecar files used by metadata-driven payload expansion.
3. Enforce resource-relative path safety for sidecar files.

Method families:
1. `SaveResourceWithArtifacts`.
2. `ReadResourceArtifact`.

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
2. Preserve raw persisted include placeholders for `resource.defaults`.

Method families:
1. `Get/Set/Unset`.

### Interface: `metadata.DefaultsArtifactStore`
Responsibilities:
1. Read and persist deterministic selector-local defaults artifacts referenced by `resource.defaults`.
2. Enforce defaults-artifact filename safety and path scoping.

Method families:
1. `ReadDefaultsArtifact/WriteDefaultsArtifact/DeleteDefaultsArtifact`.

Invariants:
1. Reads and writes MUST be scoped to the owning metadata selector directory and MUST reject traversal or arbitrary filenames.
2. Supported artifact names MUST be limited to deterministic defaults names such as `defaults.<ext>` and `defaults-<profile>.<ext>`.
3. Supported codecs MUST be `json|yaml|yml|properties`, and stored payloads MUST decode to structured objects.

### Interface: `metadata.MetadataResolver`
Responsibilities:
1. Resolve layered metadata for a logical path.
2. Expand include-backed `resource.defaults` entries before returning resolved metadata.

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

### Interface: `managedservice.ManagedServiceClient`
Responsibilities:
1. Execute remote CRUD/list operations.
2. Execute request HTTP operations against managed service endpoints.
3. Resolve OpenAPI hints for operations.
4. Expose typed transport failures.
5. Honor list-operation `jq` resolver context when list transforms call `resource("<logical-path>")`.

Method families:
1. `Get/Create/Update/Delete/List/Exists`.
2. `Request`.
3. `GetOpenAPISpec`.

Optional capability:
1. providers MAY additionally implement `managedservice.AccessTokenProvider` for token inspection workflows.

### Interface: `managedservice.AccessTokenProvider`
Responsibilities:
1. Fetch a managed service access token using configured auth settings.

Method families:
1. `GetAccessToken`.

Corner case example:
1. Callers MUST treat missing `managedservice.AccessTokenProvider` capability as a validation/configuration error, not a transport error.

Context helper contract:
1. `managedservice.WithListJQResourceResolver` MUST attach one resolver per request context and preserve deterministic cache/cycle-guard state for nested resolution calls.
2. `managedservice.ResolveListJQResource` MUST return `(value, resolved=true, err=nil)` on success, `(nil, resolved=false, err=nil)` when no resolver is attached, and `(nil, resolved=true, err!=nil)` when resolution fails.

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
1. `secrets.SecretProvider` composes `secrets.SecretStore`, `secrets.PayloadProcessor`, and `secrets.Detector`.

### Interface: `secrets.SecretStore`
Responsibilities:
1. Store lifecycle and key/value secret operations.

Method families:
1. `Init`.
2. `Store/Get/Delete/List`.

### Interface: `secrets.PayloadProcessor`
Responsibilities:
1. Mask, resolve, and normalize secret placeholders in payloads.

Method families:
1. `MaskPayload/ResolvePayload`.
2. `NormalizeSecretPlaceholders`.

### Interface: `secrets.Detector`
Responsibilities:
1. Detect likely plaintext secret candidates in payloads.

Method families:
1. `DetectSecretCandidates`.

### Interface: `orchestrator.Orchestrator`
Responsibilities:
1. Orchestrate repository-store, metadata, managed-service, and secret-provider workflows.
2. Apply desired state to remote systems.
3. Refresh local state from remote systems.
4. Compute explain/diff/list outputs.

Method families:
1. `GetLocal/GetRemote/Request/GetOpenAPISpec/Save/Apply(policy)/ApplyWithValue(policy)/Create/Update/Delete`.
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
4. `AuthError`: authn/authz failure for repository, managed service, or secret store.
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
5. `resource.<ext>` directive placeholders MUST be exact-string matches (`{{secret ...}}`, `{{payload_type .}}`, `{{payload_media_type .}}`, `{{payload_extension .}}`); embedded interpolation text remains unsupported unless a caller documents broader templating.

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
1. A `orchestrator.Orchestrator.Apply` call resolves metadata, reads remote state, compares using metadata compare rules, and then creates/updates remote state only when needed unless `ApplyPolicy.Force=true`.
2. A `secrets.SecretProvider.MaskPayload` call stores extracted values and replaces them with placeholders before `repository.ResourceStore.Save`.
3. A `config.ContextService.ResolveContext` call merges persisted context and environment overrides, then validates and returns one resolved `config.Context`.
