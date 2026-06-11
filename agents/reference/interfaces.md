# Canonical Interfaces and Contracts

**Quick navigation:** [Data Contracts (Types)](#data-contracts) | [Interface Contracts](#interface-contracts) | [Error Taxonomy](#error-taxonomy-and-propagation) | [Determinism](#determinism-requirements)

## Purpose
Define stable contracts for shared interfaces, types, determinism, error handling, and IO expectations across bounded-context packages.

## Normative Rules
1. Modules MUST depend on interfaces from owner packages, not concrete implementations.
2. `resource.Resource` MUST be the canonical resource descriptor.
3. Interface outcomes MUST be deterministic for identical inputs and state.
4. Errors MUST use the [taxonomy](#error-taxonomy-and-propagation) and preserve root-cause context.
5. Cross-package data transfer MUST use the declared contracts below only.
6. Contract changes MUST be documented here before implementation changes.

## Data Contracts

### Type: `config.ContextSelection`
Context-resolution inputs.
1. `Name`: explicit context name, optional.
2. `Overrides`: runtime key-value overrides.

### Type: `bootstrap.Session`
Client-facing application state assembled at startup.

Required fields:
1. `Contexts`: `config.ContextService`.
2. `Orchestrator`: `orchestrator.Orchestrator`.
3. `Services`: `orchestrator.ServiceAccessor` — exposes `RepositoryStore()`, `RepositorySync()`, `MetadataService()`, `SecretProvider()`, `ManagedServiceClient()`.

Invariants:
1. Fields MUST reference interfaces, not provider concrete types.
2. Clients MUST consume `bootstrap.Session` as their primary dependency entrypoint, and MUST access sub-services via `Services` rather than instantiating providers directly.

Factory contract (`bootstrap.NewSession`):
1. MUST assemble default provider implementations; clients MUST NOT instantiate providers directly.
2. MUST resolve the selected or current context at startup, returning an error when resolution or provider wiring fails.
3. When `metadata.bundle`/`metadata.bundleFile` is configured and `managedService.http.openapi` is empty, wiring MUST use bundle-provided OpenAPI source hints when available.
4. When a provider satisfies `repository.ResourceStore` but not `repository.RepositorySync`, MUST return `InternalError`.

### Type: `bootstrap.BootstrapConfig`
Startup wiring inputs.
1. `ContextCatalogPath`: optional explicit context catalog path.

### Type: `orchestrator.DefaultOrchestrator`
Default concrete orchestrator assembled by the composition root.
1. `Repository`: `repository.ResourceStore`.
2. `Metadata`: `metadata.MetadataService`.
3. `Server`: optional `managedservice.ManagedServiceClient`.
4. `Secrets`: optional `secrets.SecretProvider`.

### Type: `config.Context`
Persisted context configuration. Full schema/semantics owned by context-config.md.

Required: `Name`.

Optional: `Repository`, `ManagedService`, `SecretStore`, `Metadata` typed objects; `Preferences` map; `managedService.http.healthCheck` probe target used by `server check`.

JSON Pointer / placeholder forms:
1. Metadata pointer fields (`requiredAttributes[*]`, `secretAttributes[*]`, `externalizedAttributes[*].path`, transform `selectAttributes`/`excludeAttributes`, compare exclude/select fields, `validate.requiredAttributes[*]`) MUST use RFC 6901 JSON Pointer strings.
2. `resource.id` and `resource.alias` are identity templates that MAY embed JSON Pointer expressions and MUST accept raw shorthand such as `/id`.
3. Metadata string-template fields rendering payload/context attributes MUST accept canonical placeholder form (`{{/id}}`) plus one-level shorthand (`{{id}}`).

User-config key contract:
1. Persisted keys MUST use camelCase.
2. Catalog readers MUST reject legacy aliases and non-canonical persisted keys; unknown keys MUST fail strict decoding.

One-of / cardinality invariants (enforcement here; semantics owned by context-config.md):
1. `config.Context` MUST define at least one of `repository` or `managedService`.
2. `repository` MUST define exactly one of `git` or `filesystem` when configured.
3. `managedService.http.auth` MUST define exactly one of `oauth2`, `basic`, or `customHeaders` when `managedService.http` is configured.
4. `secretStore` MUST define exactly one of `file` or `vault`.
5. `secretStore.file` MUST define exactly one of `key`, `keyFile`, `passphrase`, `passphraseFile`.
6. `secretStore.vault.auth` MUST define exactly one of `token`, `password`, or `appRole`.
7. `metadata` MUST define at most one of `baseDir`, `bundle`, or `bundleFile`.
8. `managedService.http.proxy` MAY define any subset of `http`, `https`, `noProxy`, `auth`; an empty block disables inherited/environment proxy resolution; effective proxying requires at least one resolved proxy URL after environment merge.
9. `managedService.http.proxy.auth` MUST define `basic.credentialsRef` when configured.
10. `repository.git.remote.auth` MUST define exactly one of `basic`, `ssh`, or `accessKey`.
11. `managedService.http.requestThrottling` MUST define at least one of `maxConcurrentRequests` or `requestsPerSecond` when configured.
12. `requestThrottling.queueSize` MUST NOT be set unless `maxConcurrentRequests` is set.
13. `requestThrottling.burst` MUST NOT be set unless `requestsPerSecond` is set.

### Type: `config.Credential`
Reusable username/password definition at catalog scope.
1. `Name`: unique credential identifier.
2. `Username`: `config.CredentialValue`.
3. `Password`: `config.CredentialValue`.

Invariants:
1. Credential names MUST be unique within one catalog.
2. Persisted context components MUST reference catalog credentials by `credentialsRef.name` instead of inlining reusable pairs.

### Type: `config.CredentialValue`
One credential attribute that is either literal or prompted at runtime.
1. Literal string value, or
2. Prompt object `{prompt: true, persistInSession?: bool}`.

Invariants:
1. `persistInSession: true` MUST reuse the value for later `declarest` commands only within one shell session that exported `DECLAREST_PROMPT_AUTH_SESSION_ID`.
2. New prompt-auth session cache files MUST be written only under `XDG_RUNTIME_DIR/declarest/prompt-auth/`.
3. When `XDG_RUNTIME_DIR` or `DECLAREST_PROMPT_AUTH_SESSION_ID` is unavailable, prompted values MAY be reused only inside the current process and MUST NOT be persisted under the home directory.

### Type: `config.CredentialsRef`
Placeholder to inject a named catalog credential into one auth block.
1. `Name`: referenced catalog credential name.

Invariants:
1. When a component defines `credentialsRef`, runtime MUST inject the referenced credential object there while omitting its `name` field.
2. Referenced prompt-backed attributes MUST prompt only when the owning component first needs that value at runtime.
3. Non-interactive execution MUST fail when a required prompt-backed attribute has no cached session value.

### Type: `config.ContextCatalog`
Persisted context catalog in one YAML file.

Required:
1. `Contexts`: list of full `config.Context` objects.
2. `CurrentContext`: active context name (persisted key `currentContext`).

Optional:
1. `DefaultEditor` (persisted key `defaultEditor`).
2. `Credentials`: catalog-scoped credentials referenced by `credentialsRef`.

Invariants:
1. Context names MUST be unique and non-empty.
2. `CurrentContext` MUST reference an existing context when contexts are present.
3. Every resolved `credentialsRef.name` MUST reference an entry in `Credentials`.

### Type: `resource.Value`
Structured or opaque resource content. Allowed shapes: map object; array; scalar (`string`/`number`/`bool`/`null`); `resource.BinaryValue`.

Invariants:
1. Serialization MUST be deterministic and MUST avoid implicit numeric precision loss.
2. Opaque binary payloads MUST use `resource.BinaryValue`, not raw `[]byte`.
3. Exact placeholder directives (`{{secret .}}`, `{{payload_type .}}`, `{{payload_media_type .}}`, `{{payload_extension .}}`, metadata externalized-attribute include placeholders) MUST remain string values until workflow-specific resolution.
4. When a whole-resource payload is persisted as `{{secret .}}`, resolution MUST treat `.` as the root payload scope for that logical path.
5. Defaults-merge invariant (semantics owned by metadata.md/domain.md; enforced here): orchestrator/CLI workflows MAY compose one effective object payload from resolved `resource.defaults` plus raw `resource.<ext>` payloads. Defaults artifacts MUST be referenced only through exact include placeholders (`{{include defaults.yaml}}`, `{{include defaults-prod.properties}}`); arrays MUST replace; explicit `resource.<ext>` values MUST win; supported codecs are `json|yaml|yml|properties`; non-object or unsupported payloads MUST fail defaults validation rather than merge implicitly.

### Type: `resource.BinaryValue`
Opaque binary payload content. Required: `Bytes`.

Invariants:
1. `Bytes` MUST be treated as opaque and MUST NOT assume UTF-8 text semantics.
2. Structured CLI output MUST serialize this as a base64 wrapper object, not raw byte arrays.

### Type: `resource.PayloadDescriptor`
Describes how a payload is encoded. Fields: `PayloadType`, `MediaType`, `Extension`.

### Type: `resource.Content`
Canonical payload-plus-descriptor pair exchanged across orchestrator read/write boundaries. Fields: `Value` (`resource.Value`), `Descriptor` (`resource.PayloadDescriptor`). `resource.Value` is the bare payload element; `resource.Content` couples it with its descriptor so encoding survives transport.

### Type: `metadata.ResourceMetadata`
Behavior directives for a resource or collection. Structure/precedence owned by metadata.md; this lists the contract groups.
1. `selector.descendants`: persisted collection-selector scope; accepted only on collection metadata; MUST NOT be a mergeable resolved directive.
2. `resource` identity: `id`, `alias`, `requiredAttributes`; optional `remoteCollectionPath`; optional `format` (supported payload formats plus `any`, drives default media behavior when concrete). When `format` is omitted, `id` and `alias` default to `/id`.
3. `resource` secret: `secret`, `secretAttributes`.
4. `resource` defaults: `defaults.{mode,useProfiles,value,profiles}`; `defaults.value` and `defaults.profiles[*]` MUST accept either one structured object or one exact include placeholder pointing to deterministic selector-local files `defaults.<ext>` / `defaults-<profile>.<ext>`.
5. `resource` externalized attributes: `externalizedAttributes[*].{path,file,template,mode,saveBehavior,renderBehavior,enabled}`.
6. `operations` directives: `create`, `update`, `delete`, `get`, `compare`, `list`.
7. Operation wire fields: `path`, `method`, `query`, `headers`, `body`, `transforms[*].{selectAttributes,excludeAttributes,jqExpression}`, `validate.requiredAttributes`, `validate.assertions[*].{message,jq}`, `validate.schemaRef`; attribute refs use RFC 6901 pointers; media uses `headers` entries (e.g. `Accept`, `Content-Type`).
8. `operations.defaults.transforms`: ordered pipeline applied before operation-specific `transforms`.
9. Template helpers: `{{payload_type .}}`, `{{payload_media_type .}}`, `{{payload_extension .}}`, plus descendant-scope `{{/descendantPath}}` and `{{/descendantCollectionPath}}` when a descendant-enabled collection selector matched. `resource.id`/`resource.alias` remain segment-safe and MUST NOT resolve to slashful values.

### Type: `bundlemetadata.BundleManifest`
Decoded `bundle.yaml` for `metadata.bundle`/`metadata.bundleFile` consumers. Full normative shape owned by metadata-bundle.md.

Required: `APIVersion` (`declarest.io/v1alpha1`); `Kind` (`MetadataBundle`); `Name`; `Version` (semver-2, optional `v` prefix); `Description`; `Declarest.MetadataRoot`.

Optional: `Deprecated`; `Declarest.OpenAPI` (repo-relative path or `http`/`https` URL); `Declarest.CompatibleDeclarest` (Masterminds/semver constraint); `Declarest.CompatibleManagedService.{Product,Versions}` (paired); `Distribution.ArtifactTemplate` (`<name>-{version}.tar.gz`).

Invariants:
1. MUST decode strictly; unknown YAML keys MUST fail with `ValidationError`.
2. `CompatibleDeclarest` MUST be enforced at bundle resolution against the running binary version; the `dev` build MUST bypass the gate.
3. `CompatibleManagedService` MUST be syntactically validated at decode time; runtime evaluation against an actual managed-service version is deferred.

### Type: `bundlemetadata.BundleResolution`
Resolved bundle returned by `bundlemetadata.ResolveBundle`. See metadata-bundle.md.

Required: `MetadataDir` (absolute path); `Manifest`; `Shorthand` (bool).
Optional: `OpenAPI` (resolved source path/URL); `DeprecatedWarning` (set when `Manifest.Deprecated` is `true`).

### Type: `bundlemetadata.RegistryCredential`
Static OCI registry credential consumed by `bundlemetadata.WithRegistryCredentials`.

Required: `Registry` (host or `host:port`, compared case-insensitively); `Username`; `Password`.

Invariant: hosts MUST match the reference's `host[:port]` exactly after lower-casing; unknown hosts resolve to anonymous access.

### Type: `metadata.OperationSpec`
Resolved operation request intent.

Required: `Method`, `Path`, `Query`, `Headers`, `Accept`, `ContentType`, `Body`.
Optional: `Validate` (`metadata.OperationValidationSpec`).

### Type: `managedservice.RequestSpec`
One concrete managed-service HTTP request.

Required: `Method`, `Path`.
Optional: `Query`, `Headers`, `Accept`, `ContentType`, `Body`.

### Type: `metadata.OperationValidationSpec`
Operation payload validation directives.
1. `RequiredAttributes`: required top-level payload attributes.
2. `Assertions`: jq assertions that must evaluate truthy.
3. `SchemaRef`: OpenAPI schema reference (`openapi:request-body` or `openapi:#/...`).

### Type: `metadata.ValidationAssertion`
One jq-based payload assertion: `Message`, `JQ`.

### Type: `metadata.ResourceOperationSpecInput`
Provider-safe resource descriptor for operation-spec rendering without `resource.Resource`.

Required: `LogicalPath`, `CollectionPath`, `LocalAlias`, `RemoteID`, `PayloadDescriptor`, `Metadata`, `Payload`.

### Type: `metadata.Operation`
Supported operation identifiers: `get`, `create`, `update`, `delete`, `list`, `compare`.

### Type: `metadata.InferenceRequest`
Inference behavior options: `Apply`, `Recursive`.

### Type: `repository.ResetPolicy`
Reset behavior options: `Hard`.

### Type: `repository.PushPolicy`
Push behavior options: `Force`.

### Type: `repository.ListPolicy`
List behavior options: `Recursive`.

### Type: `repository.DeletePolicy`
Delete behavior options: `Recursive`.

### Type: `repository.SyncReport`
Local/remote sync status. Required: `State`, `Ahead`, `Behind`, `HasUncommitted`.

### Type: `repository.WorktreeStatusEntry`
One file-level local worktree change for verbose status output.

Required: `Path`; `Staging` (git-style index code, e.g. `M`/`?`); `Worktree` (git-style worktree code, e.g. `M`/`?`).

### Type: `repository.RepositoryStatusDetailsReader`
Optional capability for verbose local worktree status inspection.

Method contract (`WorktreeStatus(ctx)`):
1. MUST return deterministic path ordering for identical repository state.
2. MUST return only local worktree/index change details and MUST NOT mutate repository state.

### Type: `repository.RepositoryTreeReader`
Optional capability for directory-tree inspection of local repository layout.

Method contract (`Tree(ctx)`):
1. MUST return deterministic lexicographically sorted slash-delimited directory paths relative to the repository root.
2. MUST return directories only (no files) and MUST exclude the repository root itself.
3. MUST exclude hidden control directories (e.g. `.git`) and the reserved metadata namespace directory `_`.

### Type: `repository.HistoryFilter`
Local VCS history query filters.

Fields: `MaxCount`, `Author`, `Grep`, `Since`, `Until`, `Paths`, `Reverse`.

Invariants:
1. `Since`/`Until` MAY be nil for open-ended ranges.
2. `Paths` entries MUST be interpreted as repository-relative logical prefixes by history-capable providers.

### Type: `repository.HistoryEntry`
One local VCS commit entry.

Required: `Hash`, `Author`, `Email`, `Date`, `Subject`.
Optional: `Body`.

### Type: `orchestrator.DeletePolicy`
Type alias for `repository.DeletePolicy` — orchestrator and repository share one canonical recursive-scope directive.

### Type: `orchestrator.ListPolicy`
Type alias for `repository.ListPolicy` — orchestrator and repository share one canonical recursive-scope directive.

### Type: `orchestrator.ApplyPolicy`
Apply behavior options: `Force`.

### Type: `resource.Resource`
Canonical descriptor for resource state and routing.

Required:
1. `LogicalPath`: normalized repository path.
2. `CollectionPath`: normalized parent collection path.
3. `LocalAlias`: alias resolved for local storage.
4. `RemoteID`: identity used for remote path resolution.
5. `ResolvedRemotePath`: concrete remote operation path.
6. `Payload`: `resource.Value` content.
7. `PayloadDescriptor`: `resource.PayloadDescriptor` describing the payload type, media type, and extension.

Invariants:
1. Logical path MUST be absolute and normalized.
2. Alias and remote-ID resolution MUST be explicit and reproducible.
3. Resource identity MUST NOT rely on implicit directory naming alone.

### Type: `resource.DiffEntry`
One deterministic compare output item.

Required: `ResourcePath`, `Path`, `Operation`, `Local`, `Remote`.

Field semantics:
1. `ResourcePath` MUST be the normalized absolute logical resource path.
2. `Path` MUST be an RFC 6901 JSON Pointer relative to the resource payload root.
3. `Path == ""` MUST represent a root payload replacement.

Example: when the entire payload for `/customers/acme` differs (e.g. remote missing), the entry uses `ResourcePath="/customers/acme"`, `Path=""`.

### Type: `managedservice.ListJQResourceResolver`
Logical-path resource resolution callback for list-operation `jq` `resource("<logical-path>")` calls.

Signature: `func(ctx context.Context, logicalPath string) (resource.Value, error)`.

Invariants:
1. Identical `(logicalPath, state)` MUST return deterministic output.
2. Implementations MUST treat `logicalPath` as a normalized absolute path.

## Interface Contracts

### Interface: `config.ContextService`
Responsibilities: manage context definitions; resolve active context; apply environment/runtime overrides; validate configuration.

Method families: `Create/Update/Delete/Rename/List`; `SetCurrent/GetCurrent`; `ResolveContext`; `Validate`.

Composition: composes `config.ContextCatalogWriter`, `config.ContextCatalogReader`, `config.ContextResolver`, `config.ContextValidator`.

### Interface: `config.ContextCatalogWriter`
Responsibilities: mutate persisted catalog entries and active selection.
Method families: `Create/Update/Delete/Rename`; `SetCurrent`.

### Interface: `config.ContextCatalogReader`
Responsibilities: read persisted catalog entries and the current selection.
Method families: `List`; `GetCurrent`.

### Interface: `config.ContextCatalogEditor`
Responsibilities: read and replace the persisted full catalog as one validated document.
Method families: `GetCatalog`; `ReplaceCatalog`.

### Interface: `config.ContextResolver`
Responsibilities: resolve one effective context from catalog state plus runtime/environment overrides.
Method families: `ResolveContext`.

### Interface: `config.ContextValidator`
Responsibilities: validate context configuration semantics and one-of invariants.
Method families: `Validate`.

### Interface: `repository.ResourceStore`
Responsibilities: persist resources by logical path; read/list/delete resources; enforce path safety and layout invariants.

Method families: `Save/Get/Delete(policy)/List(policy)/Exists`.

Invariants:
1. `Get` MUST return only the raw persisted payload from `resource.<ext>` and MUST NOT merge metadata defaults into the response.
2. `Save` MUST persist only the raw repository payload for `resource.<ext>` and MUST NOT write defaults artifacts or flatten metadata-resolved defaults into repository-owned discovery.

### Interface: `repository.ResourceArtifactStore`
Responsibilities: persist and read sidecar files for one logical resource alongside the canonical payload file; enforce resource-relative path safety for sidecars.
Method families: `SaveResourceWithArtifacts`; `ReadResourceArtifact`.

### Interface: `repository.RepositoryCommitter`
Responsibilities: create a local VCS commit for repository mutations when the active backend supports it.
Method families: `Commit(message)`.
Invariant: `Commit` MUST return `(false, nil)` when there are no local changes to commit.

### Interface: `repository.RepositoryHistoryReader`
Responsibilities: read local VCS commit history with deterministic filtering when supported.
Method families: `History(filter)`.

### Interface: `repository.RepositoryTreeReader`
Responsibilities: expose a deterministic local directory tree view for CLI inspection.
Method families: `Tree`.

### Interface: `repository.RepositorySync`
Responsibilities: manage repository lifecycle/synchronization; expose deterministic sync status; expose destructive local cleanup of uncommitted changes.
Method families: lifecycle `Init/Refresh/Clean/Reset/Check`; sync `Push/SyncStatus`.

### Interface: `metadata.MetadataService`
Responsibilities: read/write metadata; resolve layered metadata; render templates and derive operation directives.

Method families: `Get/Set/Unset`; `ResolveForPath`; `RenderOperationSpec`.

Composition (required): composes `metadata.MetadataStore`, `metadata.MetadataResolver`, `metadata.OperationSpecRenderer`. Providers MAY additionally implement the optional capabilities `metadata.ResourceOperationSpecRenderer`, `metadata.DefaultsArtifactStore`, `metadata.CollectionChildrenResolver`, and `metadata.CollectionWildcardResolver`; callers discover these by type assertion, not through `MetadataService`.

### Interface: `metadata.MetadataStore`
Responsibilities: read/persist metadata overrides by logical path; preserve raw persisted include placeholders for `resource.defaults`.
Method families: `Get/Set/Unset`.

### Interface: `metadata.DefaultsArtifactStore`
Responsibilities: read/persist deterministic selector-local defaults artifacts referenced by `resource.defaults`; enforce filename safety and path scoping.
Method families: `ReadDefaultsArtifact/WriteDefaultsArtifact/DeleteDefaultsArtifact`.

Invariants:
1. Reads/writes MUST be scoped to the owning metadata selector directory and MUST reject traversal or arbitrary filenames.
2. Artifact names MUST be limited to deterministic defaults names (`defaults.<ext>`, `defaults-<profile>.<ext>`).
3. Codecs MUST be `json|yaml|yml|properties`, and stored payloads MUST decode to structured objects.

### Interface: `metadata.MetadataResolver`
Responsibilities: resolve layered metadata for a logical path; expand include-backed `resource.defaults` entries before returning resolved metadata.
Method families: `ResolveForPath`.

### Interface: `metadata.OperationSpecRenderer`
Responsibilities: render operation specs from metadata using a resource scope.
Method families: `RenderOperationSpec`.

### Interface: `metadata.ResourceOperationSpecRenderer`
Responsibilities: render operation specs using `metadata.ResourceOperationSpecInput` as the resource descriptor.
Method families: `RenderOperationSpecForResource`.

### Interface: `managedservice.ManagedServiceClient`
Responsibilities: execute remote CRUD/list operations; execute request HTTP operations against managed-service endpoints; resolve OpenAPI hints; expose typed transport failures; honor the list-operation `jq` resolver context when list transforms call `resource("<logical-path>")`.

Method families: `Get/Create/Update/Delete/List/Exists`; `Request`; `GetOpenAPISpec`.

Optional capability: providers MAY additionally implement `managedservice.AccessTokenProvider` for token-inspection workflows.

Context helper contract:
1. `managedservice.WithListJQResourceResolver` MUST attach one resolver per request context and preserve deterministic cache/cycle-guard state for nested resolution.
2. `managedservice.ResolveListJQResource` MUST return `(value, resolved=true, err=nil)` on success, `(nil, false, nil)` when no resolver is attached, and `(nil, true, err!=nil)` when resolution fails.

### Interface: `managedservice.AccessTokenProvider`
Responsibilities: fetch a managed-service access token using configured auth settings.
Method families: `GetAccessToken`.
Corner case: callers MUST treat a missing `AccessTokenProvider` capability as a validation/configuration error, not a transport error.

### Interface: `secrets.SecretProvider`
Responsibilities: store/retrieve secrets; mask/unmask payload values using placeholders; detect likely secret candidates.

Method families: `Init`; `Store/Get/Delete/List`; `MaskPayload/ResolvePayload`; `NormalizeSecretPlaceholders`; `DetectSecretCandidates`.

Composition: composes `secrets.SecretStore`, `secrets.PayloadProcessor`, `secrets.Detector`.

### Interface: `secrets.SecretStore`
Responsibilities: store lifecycle and key/value secret operations.
Method families: `Init`; `Store/Get/Delete/List`.

### Interface: `secrets.PayloadProcessor`
Responsibilities: mask, resolve, and normalize secret placeholders in payloads.
Method families: `MaskPayload/ResolvePayload`; `NormalizeSecretPlaceholders`.

### Interface: `secrets.Detector`
Responsibilities: detect likely plaintext secret candidates in payloads.
Method families: `DetectSecretCandidates`.

### Interface: `orchestrator.Orchestrator`
Responsibilities: orchestrate repository-store, metadata, managed-service, and secret-provider workflows; apply desired state to remote; refresh local state from remote; compute explain/diff/list outputs.

Method families (read/write payloads use `resource.Content`):
1. `GetLocal/GetRemote/Request/GetOpenAPISpec/Save/Apply(policy)/ApplyWithContent(content, policy)/Create/Update/Delete`.
2. `ListLocal(policy)/ListRemote(policy)`.
3. `Diff/Template`.

Composition: composes `orchestrator.LocalReader`, `orchestrator.RemoteReader`, `orchestrator.OpenAPISpecReader`, `orchestrator.ManagedRequestExecutor`, `orchestrator.ResourceSaver`, `orchestrator.ResourceApplier`, `orchestrator.ResourceDiffer`, `orchestrator.TemplateRenderer`.

### Interface: `orchestrator.CompletionService`
Responsibilities: provide read-only local/remote/OpenAPI capabilities for CLI path completion and template-path expansion.
Method families: `GetLocal/ListLocal`; `GetRemote/ListRemote`; `GetOpenAPISpec`.

### Interface: `orchestrator.RemoteReader`
Responsibilities: provide remote read/list operations without mutation behavior.
Method families: `GetRemote/ListRemote`.

## Error Taxonomy and Propagation
Categories:
1. `ValidationError`: invalid input, shape, path, or config.
2. `NotFoundError`: missing local or remote resource.
3. `ConflictError`: divergence, non-unique identity, or write collision.
4. `AuthError`: authn/authz failure for repository, managed service, or secret store.
5. `TransportError`: network, TLS, timeout, and protocol issues.
6. `InternalError`: unexpected invariant violations.

Propagation:
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
4. Irreversible operations MUST support dry-run explanation mode at the orchestrator boundary.
5. `resource.<ext>` directive placeholders MUST be exact-string matches (`{{secret ...}}`, `{{payload_type .}}`, `{{payload_media_type .}}`, `{{payload_extension .}}`); embedded interpolation text remains unsupported unless a caller documents broader templating.

## Failure Modes
1. Context resolution failure (missing/invalid config) -> `ValidationError`/`NotFoundError`.
2. Path traversal attempt -> `ValidationError`.
3. Unresolved template variable on render -> `ValidationError`.
4. Missing secret key or unavailable store -> `NotFoundError`/`AuthError`.
5. Remote operation transport/auth failure -> `TransportError`/`AuthError`.

## Edge Cases
1. Alias collision across sibling resources in one collection.
2. Metadata directives present at collection scope but missing at resource scope.
3. Payloads with mixed numeric representations.
4. Partial context configuration with optional managers absent.

## Examples
1. `orchestrator.Orchestrator.Apply` resolves metadata, reads remote state, compares using metadata compare rules, then creates/updates remote state only when needed unless `ApplyPolicy.Force=true`.
2. `secrets.SecretProvider.MaskPayload` stores extracted values and replaces them with placeholders before `repository.ResourceStore.Save`.
3. `config.ContextService.ResolveContext` merges persisted context and environment overrides, then validates and returns one resolved `config.Context`.
