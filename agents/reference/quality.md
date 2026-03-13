# Testing, Quality, and Security

## Purpose
Define quality gates and security invariants so behavior changes are verifiable and safe.

## In Scope
1. Test strategy by risk level.
2. Security and safety controls.
3. Required regression/acceptance coverage.
4. Release-readiness checks.

## Out of Scope
1. CI vendor configuration.
2. Runtime observability platform setup.
3. UI style concerns.

## Normative Rules
1. Every behavior change MUST add tests at the lowest effective layer.
2. High-risk orchestration or integration changes MUST include integration coverage.
3. CLI contract changes MUST include command-level success and failure tests.
4. Security-sensitive flows MUST include negative tests.
5. Deterministic ordering guarantees MUST be asserted.
6. Path traversal protections MUST be tested for repository and metadata access.
7. Secret values MUST never appear in logs, errors, or snapshots.
8. New normative rules SHOULD include an explicit matching test expectation.
9. Kubernetes operator behavior changes MUST include controller-level coverage for CRD validation/status transitions plus webhook authentication/event-filtering paths.
10. Changes to persisted metadata or context contracts MUST update the corresponding `schemas/*.json` artifacts and include a low-level check that those schema files remain valid JSON and keep their expected top-level wiring.

## Data Contracts
Test layers:
1. Unit: pure transforms, normalization, metadata layering/template rendering, secret placeholder normalization.
2. Integration: orchestrator workflows with fake providers and conflict handling.
3. E2E: CLI workflows using representative stacks and fixture trees.

Acceptance contracts:
1. Orchestrator idempotency for repeated apply.
2. Stable diff ordering for equivalent inputs.
3. Typed error categories for all major failure classes.

## Required Scenario Coverage

### Core Semantics
1. Metadata precedence: wildcard/literal collisions and template resolution (including relative references).
2. Repository safety: traversal rejection and deterministic list/delete recursion behavior.
3. Identity handling: alias/ID divergence and ambiguity conflict detection.
4. Secrets lifecycle: detect/mask/resolve/normalize behavior and non-disclosure guarantees.
5. Compare/diff semantics: suppression/filter rules, stable ordering, structured diff/explain `resource.DiffEntry` path shape (`ResourcePath` + RFC 6901 `Path` pointer with `Path=""` for root replace), and stable text-output rendering for `resource diff --output text`.

### CLI Behavior
6. CLI execution footer: resource mutation commands (`resource save|apply|create|update|delete`) and state-changing HTTP request commands (`resource request post|put|patch|delete|connect`) emit deterministic `[OK|ERROR] ...` status lines to stderr unless `--no-status` is set, interactive terminals apply bold color tags, `--no-color`/`NO_COLOR` disable ANSI color tags, nil payload outputs stay empty, state-changing commands suppress payload output by default, and `--verbose` restores that payload output.
7. CLI safeguards: validation errors, conflicting path inputs, and destructive-operation protections.
8. Metadata edit CLI: `metadata edit` opens YAML metadata content, seeds empty metadata for missing overrides, rejects invalid edited YAML, and persists only validated metadata updates.
9. Global flag env defaults: `DECLAREST_CONTEXT`, `DECLAREST_OUTPUT`, `DECLAREST_VERBOSE`, `DECLAREST_VERBOSE_INSECURE`, `DECLAREST_NO_STATUS`, `DECLAREST_NO_COLOR`, and `NO_COLOR` honor `flag > env > built-in default` precedence, and explicit `--no-status=false` or `--no-color=false` override env-backed defaults.

### Context and Config
9. Context config: strict decode after legacy-alias normalization, one-of validation (including repository-only and remote-only contexts), legacy `repository.resource-format` migration to `preferences.preferredFormat`, overrides precedence, and missing-catalog behavior.
10. Context defaults: omitted `repository.git.remote.autoSync` enables automatic push behavior, and omitted `managedServer.http.healthCheck` probes the normalized `managedServer.http.baseURL`.
11. Runtime `${ENV_VAR}` resolution: CLI context catalogs preserve placeholder text on disk, resolve exact-match placeholders before runtime validation/defaulting, and operator CR validation/runtime paths apply the same expansion semantics before dependency and overlap checks.

### Metadata and Bundles
10. Metadata bundles: manifest validation (`bundle.yaml` required fields), shorthand name/version contract checks, secure tar.gz extraction safeguards, and deterministic cache reuse behavior.
11. Bundle OpenAPI wiring: `managedServer.http.openapi` precedence over bundle hints, fallback to `bundle.yaml declarest.openapi`, bundle-root/metadata-root OpenAPI files (`openapi.yaml|openapi.yml|openapi.json`), recursive bundle OpenAPI file discovery, and cross-origin OpenAPI fetches without leaked auth headers.
12. Managed-server OpenAPI version compatibility: `openapi: 3.x` and `swagger: 2.0` specs provide equivalent method-support checks, media fallback defaults, and `validate.schemaRef=openapi:request-body` behavior for body-bearing operations.
13. Remote operation construction: OpenAPI-assisted defaults with explicit metadata override precedence, including inferred metadata `resource.defaultFormat` and `defaultFormat: any` when mixed payload formats are advertised.

### Repository
14. Repository sync: conflict classes, actionable outcomes, `repository status` output contract, and verbose worktree-detail coverage for `repository status --verbose`.
15. Repository defaults-sidecar semantics: metadata-root `defaults.<ext>` merge/compaction rules, raw defaults-sidecar read/write capability, supported merge-capable payload types (`json|yaml|ini|properties`), JSON/YAML cross-compatibility, new-sidecar format selection that prefers the collection resource payload type, rejection of legacy per-resource `defaults.<ext>` locations, unsupported payload rejection (`xml|hcl|text|octet-stream|opaque suffixes`), preservation of explicit empty override files for fully-defaulted resources, and deterministic behavior when only collection defaults exist without a child resource override.

### E2E Harness
15. E2E profiles: `cli-basic|cli-full|cli-manual|operator-manual|operator-basic|operator-full` workload behavior, `smoke|main|operator-main|corner` case selection, `CASE_PROFILES` family filtering, deterministic step statuses, manual-profile managed-server access handoff details, and operator-profile installation/handoff/automation validation.
16. E2E runtime UX: grouped step reporting (`RUNNING|OK|FAIL|SKIP`), live `SPAN` elapsed-time updates for running steps, actionable failure log pointers, and final-summary execution parameters that distinguish explicit selections from defaulted values (including profile defaults, component-elected defaults, and env-backed container engine selection).
17. Resource-server fixtures: metadata `resource` identity mapping (`id`/`alias` identity templates) and intermediary `/_/` expansion for nested trees.
18. E2E component orchestration: dependency-aware hook ordering, parallel ready-batch execution, and cycle/missing-dependency failures.

### Authentication
19. OAuth2 component auth: `client_credentials` token issuance, managed-server auth rejection when oauth2 config is missing or invalid, and bootstrap warnings when `managedServer.http.auth.oauth2.tokenURL` uses plain HTTP.
20. mTLS component auth: only configured client certificates are accepted when mTLS is enabled.
21. Basic-auth component auth: requests fail without valid credentials when basic auth is selected and succeed with configured username/password.
22. Managed-server auth redaction: debug logging masks `Authorization` and custom auth header names defined by `managedServer.http.auth.customHeaders` unless `--verbose-insecure` is enabled, while non-auth headers remain visible.

### HTTP Request CLI
23. HTTP request CLI routing: `resource request <method>` maps to managed-server requests with positional/flag path validation, metadata-rendered header propagation, payload decoding from `--payload <path|->` or stdin, inline decoding only for structured payload formats, `application/octet-stream` for opaque payloads, and `--accept-type` / `--content-type` overrides for raw non-metadata endpoints.
24. mTLS trust reload: updating `simple-api-server` trusted client-cert files at runtime changes access behavior for new connections without service restart, including empty trusted-cert sets denying all access.

### Secret Handling
24. Resource save secret safeguard: `resource save` fails on potential plaintext secrets that are not metadata-declared unless `--allow-plaintext` is provided, metadata-declared attribute candidates are automatically stored/masked before persistence, metadata `resource.secret: true` automatically triggers whole-resource secret persistence for default single-resource saves, `resource save --secret-attributes[=<comma-separated-json-pointers>]` handles selected structured-payload candidates and rejects non-structured payloads, stores deterministic path-scoped secret keys, writes `{{secret .}}` placeholders, updates metadata, skips group items that do not contain requested attributes, and fails with warning when non-metadata-declared candidates remain unhandled, and `resource save --secret` stores the whole encoded payload under `<logical-path>:.`, preserves the repository payload suffix, persists `resource.secret: true`, writes only the exact root placeholder, and rejects incompatible secret/list flags.
25. Secret detect metadata fix flow: `secret detect` scans repository scope when no payload input is provided (default scope `/`), `secret detect --fix` merges detected JSON Pointer attributes into metadata `resource.secretAttributes`, and `--secret-attribute` filtering has negative validation coverage for payload and repository-scan modes.
26. Secret-candidate false-positive guard: detection and save-time checks ignore numeric-only and boolean-like policy/toggle values for secret-like keys/attributes (for example action-token lifespan maps and token-claim toggles) while preserving detection for real plaintext secret strings.

### Resource Operations
27. Resource collection-target scope: `resource apply|create|update|delete|diff` execute direct-child collection targets by default; `apply|create|update|delete` include descendants with `--recursive`; `resource diff` flattens per-resource compare output into one deterministic list, and `resource apply|create|update` accept explicit payload input for single-target mutations while repository-driven mode loads payloads for resolved targets when explicit input is absent; explicit-payload mutations MUST include CLI coverage for collection-path child inference from metadata alias/id plus the equivalent explicit resource path.
28. Resource metadata HTTP-method override: `resource get|list|apply|create|update|delete --http-method <METHOD>` overrides metadata operation `method` for remote operations, rejects repository-only source selections where no remote request is executed, and keeps `resource apply` remote read checks on default GET semantics while overriding only create/update writes.
29. Apply drift/force contract: `resource apply` reads remote state, creates only on `NotFound`, compares using metadata compare transforms, skips update when drift is empty by default, and executes update when `--force` (CLI) or `spec.sync.force` (operator) is enabled.
30. Resource save list identity fallback: when list entries omit metadata-defined alias/id attributes, `resource save` falls back to common identity attributes (`clientId`, `id`, `name`, `alias`) before returning alias-resolution errors, preserves per-entry payload descriptors when they are available, and `resource.defaultFormat: any` prevents one collection-level descriptor from coercing every saved child to the same repository format.
31. Repository defaults-sidecar contract: repository-backed `resource get|apply|create|update|diff|template|edit|copy|save` use the effective payload merged from metadata-root `defaults.<ext>` and `resource.<ext>` for merge-capable object resources, while repository writes compact back into `resource.<ext>` so unchanged defaulted fields remain only in `defaults.<ext>`.
32. Resource defaults CLI contract: `resource defaults get|edit|infer` command-level coverage MUST verify raw defaults reads from metadata selector directories, empty-object behavior when the sidecar is absent, editor persistence/clear behavior, `--save` persistence including defaults-sidecar codec selection, `--check` success/mismatch behavior, the `--managed-server` safety gate requiring `--yes`, `--wait` validation plus parsing of bare-integer seconds vs explicit durations, and managed-server inference isolation from sibling/defaults-sidecar value selection.
33. Defaults pruning contract: `resource get --prune-defaults` MUST compact repository and managed-server outputs against raw metadata-root defaults sidecars while preserving empty-object output when all fields are defaulted and pruning fields covered by stable empty-object defaults, and `resource save --prune-defaults` MUST compact single-resource and per-item list saves before persistence while keeping explicit empty override files for fully-defaulted saved resources.

### Read Fallback
32. Metadata-aware read fallback: `resource get` and `resource save` (no input) attempt literal remote read first and then collection list/filter fallback by metadata alias/id when `NotFound`; `resource request get` mirrors this behavior after a `NotFound` literal request; and managed-server read fallback MUST also cover complex alias templates whose `operations.get.path` cannot be rendered from the requested logical segment alone by resolving one unique matched parent-collection candidate payload.
33. Resource copy source fallback: `resource copy` reads the source from the local repository first and retries from the remote server only when the local read returns `NotFoundError`, then preserves the resolved effective payload for save/override validation.
34. Resource edit source resolution: `resource edit` resolves existing local resources through the same metadata-aware bounded fallback used by repository-backed single-resource workflows, preserves the canonical local repository path when that fallback resolves a different stored path, and falls back to a remote read only when local resolution returns `NotFoundError`.
35. Metadata-aware path fallback breadth: repository-backed single-resource workflows (`resource get --source repository`, `resource apply`, `resource update`, `resource diff`, `resource explain`) use literal local lookup then bounded metadata `resource.id` fallback with reverse matching limited to simple single-pointer templates; remote delete retries with metadata-aware identity after literal `NotFound`, and delete/request-get-request-delete coverage MUST include complex alias templates that require matched list-item payload fields to rerender the remote path.
36. Resource secret placeholder resolution: remote workflows resolve `{{secret .}}` to logical-path scoped JSON Pointer keys for attribute placeholders, resolve an exact whole-resource `{{secret .}}` payload to `<logical-path>:.`, and continue to support `{{secret <custom-key>}}` overrides.

### Metadata Persistence
36. Metadata persistence normalization: metadata writes omit nil fields (no `null` serialization) while preserving explicit empty arrays/maps used for merge replacement behavior.

### Externalized Attributes
37. Externalized-attribute contract: `resource.externalizedAttributes` defaults omitted directive fields deterministically, `resource save` externalizes configured string attributes into sibling files plus configured placeholders, JSON Pointer `path` values support numeric and `*` wildcard traversal with deterministic indexed sidecar filenames, repository-backed apply/create/update/diff expand matching placeholders from sidecar files before identity/diff/mutation logic, disabled entries are ignored, and duplicate path/file, traversal, non-string save values, and missing referenced files fail with `ValidationError`.
38. Resource save override guard: saving to an already-present logical path must fail without `--force` and succeed when `--force` is provided so repository state cannot be overwritten accidentally.

### Wildcard and Collection
39. Resource save wildcard expansion: `resource save` must expand `_` path segments via remote direct-child list traversal, persist all matched collection/resource targets, reject wildcard+payload input, and return `NotFound` when no concrete matches are resolved.

### Completion
40. CLI path completion: completion merges repository paths, remote paths, and OpenAPI paths with command-aware source priority (remote-first defaults for `resource get|save|list|delete` and `resource request <method>`, repository-first defaults for `resource apply|create|update|diff|explain|template`, `metadata *`, and path-aware `secret` commands; each uses fallback to the secondary source when the preferred source has no candidates); templated OpenAPI segments resolve concrete values through source-prioritized collection listings, placeholder segments (`{...}`) are excluded from rendered suggestions, alias-aware segments prefer rendered metadata `resource.alias` values over IDs when available, rendered suggestions use canonical absolute paths that remain prefix-compatible with shell token filtering, collection-prefix suggestions preserve deterministic trailing-`/` semantics, scoped completion queries prefer parent-path direct listing with bounded recursive/root fallback only when needed, metadata selector-only child branches (for example intermediary `/_/` templates not represented in OpenAPI) are surfaced as canonical logical child suggestions when selectors match the typed path, and path completions emit `NoSpace` so accepted completions do not auto-append a trailing space.

### Bootstrap and Context CLI
41. CLI startup bootstrap gating: `version` and context-catalog commands (`context print-template|add|edit|update|delete|rename|list|use|show|current|resolve|validate`) execute without active-context resolution, while runtime commands (`resource/*` including `resource request <method>`, `server/*`, `repository/*`, `metadata/*`, `secret/*`, and `context check`) still fail fast when no active context is available.
42. Context add input contract: `context add` accepts context name from positional arg or global `--context`, decodes JSON/YAML from `--content-type` or payload file extension, defaults extension-less input to JSON, skips interactive name prompt when provided, and returns `ValidationError` when both names differ.
43. Context add interactive schema coverage: wizard prompts require managed-server capture, allow optional-section skipping for optional blocks, support full context attribute capture across repository/managed-server/secret-store/preference blocks, and enforce one-of prompt branching so only the selected auth/key-source/provider branch is collected.
44. Context template output contract: `context print-template` emits a stable commented YAML template containing all supported context options, explicitly documents mutually-exclusive sections, accepts no positional args, and runs without active-context resolution.
45. Context validation contract: context-catalog mutation and resolve flows fail with `ValidationError` when both `repository` and `managedServer` are missing, preserve repository-only and remote-only contexts as valid shapes, normalize documented legacy alias keys before strict decode, map legacy `repository.resource-format` to `preferences.preferredFormat`, allow sparse proxy override blocks while still rejecting incomplete proxy auth or invalid proxy URLs, interactive `context add` still prompts managed-server configuration, and `context edit` rejects invalid edited catalog/context YAML without persisting partial changes.

### Repository CLI
46. Repository command repository-type awareness: `repository push` and `repository commit` fail fast with `ValidationError` on filesystem contexts, `repository status` default text output differs by repository type while preserving stable structured (`json|yaml`) sync fields, `repository status --verbose` emits deterministic git-worktree details for git contexts, and `repository tree` returns a deterministic local directory-only tree view.
47. Repository commit CLI contract: `repository commit --message|-m` validates non-empty messages, creates one local git commit when the worktree is dirty, and returns deterministic success output when no changes are available to commit.
48. Repository clean contract: `repository clean` invokes repository cleanup, removes tracked and untracked uncommitted changes in git repositories, and succeeds as a no-op for filesystem repositories.

### Metadata CLI
49. Metadata selector-path contract: `metadata infer|render|get` accept intermediary selector paths (for example `/admin/realms/_/clients/`), `metadata render` defaults operation by target kind and retries `list` when defaulted `get` is missing a path, infer uses OpenAPI hints when available (including fallback placeholder normalization for non-template-safe OpenAPI parameter names), `metadata infer` exposes only supported flags (no hidden recursive mode), structured metadata output omits nil directive fields, and infer output omits directives equal to deterministic fallback defaults.

### Secret Output
50. Resource get secret-output contract: `resource get` redacts metadata-declared secret attributes to `{{secret .}}` for `--source repository` and `--source managed-server` by default, and `--show-secrets` restores plaintext output for those attributes.
51. Whole-resource secret output contract: `resource get --source repository --show-secrets` resolves exact whole-resource `{{secret .}}` placeholders back to descriptor-typed payloads (including octet-stream files), while the default repository read keeps the placeholder-only file content masked.

### Collection and Exclusion
52. Collection exclude contract: `resource get|list|save --exclude <item[,item...]>` exclude matching collection items by alias, ID, or direct child path segment for managed-server and repository collection reads, and `resource save --exclude` rejects `--mode single` with `ValidationError`.

### Metadata Output
53. CLI completion flag contract: shell completion output avoids duplicate option suggestions that differ only by `=` suffix (for example `--output` and `--output=`).
54. Metadata get output contract: `metadata get` returns the full canonical nested metadata schema by default with explicit default values for unset attributes, `metadata get --overrides-only` returns the compact override object without expanded defaults, fallback inference remains endpoint-gated (OpenAPI or remote discovery), and commands still return `NotFoundError` when neither source resolves the endpoint.
55. Metadata infer apply persistence contract: `metadata infer --apply` persists compacted metadata only (no default-equivalent directives), matching the command output contract, and JSON output/persisted JSON metadata end with one trailing newline.

### Secret CLI
56. Secret CLI contract: `secret set|get|delete` accept raw-key, path+key, and composite `<path>:<key>` target forms; `secret list [path]` returns deterministic key-only output (with structured output support for automation); `secret list <path> --recursive` includes descendant path-scoped secret entries rendered as the full relative path from `<path>` (including whole-resource `:.` keys); `secret get <path>` without a key fails with `ValidationError` that points users to `secret list`; and single-secret reads print raw plaintext values without JSON quoting.
57. Inline text payload contract: explicit non-structured `--content-type` values such as `text`, `txt`, or `text/plain` keep inline mutation payloads literal instead of parsing `key=value` or `/pointer=value` shorthand, and whole-resource `resource save --secret` preserves that text payload under `<logical-path>:.` while writing `resource.txt` placeholders.

### Remote Read Fallback
57. Remote collection `NotFound` fallback: `resource get`/`resource save` remote reads treat `404` as an empty list only when collection intent is confirmed by repository structure hints or OpenAPI inference and the collection path is not a misclassified concrete resource path; nested collection reads MUST preserve `NotFound` when the parent resource is also `NotFound`; single-resource paths still use metadata alias/id fallback and preserve `NotFound` when no match exists.
58. Remote read fallback error contract: when single-resource parent-collection fallback receives a non-list validation payload (for example object/array-shape mismatch), commands preserve the original resource `NotFound` instead of surfacing list-decoding validation output.

### Metadata Path and Transform
59. Metadata path indirection contract: rendered operation specs resolve `resource.remoteCollectionPath` templates from handled logical-path context (for example intermediary `/_/` selectors), default the remote collection path to the handled logical collection path when omitted, keep `logicalCollectionPath` and `remoteCollectionPath` template values distinct, accept canonical pointer placeholders such as `{{/id}}` plus one-level shorthand such as `{{id}}`, preserve legacy `{{.id}}` compatibility, and treat `.`-prefixed operation paths as collection-relative with omitted operation paths defaulting to `.` for `create|list` and `./{{/id}}` for `get|update|delete|compare`.
60. List jq transform contract: list workflows execute resolved list-operation `jq` expressions before list-shape extraction; valid filters constrain candidate resources deterministically and invalid jq expressions fail with `ValidationError`.
61. Remote metadata singleton fallback contract: when metadata list filtering (`jq`) yields exactly one candidate for a `NotFound` single-resource read, fallback resolves that candidate deterministically (including canonical-ID retry path) only at selector depth and preserves `NotFound` for explicit child identity segments.
62. List jq resource-reference contract: `jq` expressions can call `resource("<logical-path>")`, resolution uses the active workflow source through context resolver wiring, missing resolver context fails with `ValidationError`, repeated lookups are cached per expression evaluation, and invalid arguments/cyclic resolver dependencies fail with `ValidationError`.

### Resource Get Collection
63. Resource get explicit collection-marker contract: `resource get <path>/` with remote source resolves `<path>` as a normalized collection list target first; when the list attempt fails with list-shape validation (`list response ...` or `list payload ...`), command flow falls back to one normalized single-resource remote read for `<path>`.
64. Completion space-token contract: shell completion scripts preserve path candidates containing spaces as a single completion token (for example alias segment `AD PRD`) instead of truncating at the first whitespace boundary.

### Server CLI
65. Resource-server utility CLI contract: `server get base-url|token-url|access-token` print plaintext values from the active managed-server configuration/auth flow, `get token-url|access-token` fail with `ValidationError` when OAuth2 auth is not configured, and `server check` performs a GET probe against `managedServer.http.healthCheck` when configured (otherwise the normalized `managedServer.http.baseURL` path) and succeeds only on successful HTTP responses.

### Payload Type
66. Payload-type directive and media-default contract: metadata template rendering supports canonical pointer placeholders such as `{{/id}}`, one-level shorthand such as `{{id}}`, compatibility legacy `{{.id}}`, plus `{{payload_type .}}`, `{{payload_media_type .}}`, and `{{payload_extension .}}`; template scopes inject `contentType` from the active payload descriptor when the payload omits it, `metadata get` preserves those helper tokens in printed metadata while `resource get --show-metadata` resolves them against the active payload descriptor, exact payload-type placeholders resolve during template/remote workflows without forcing secret resolution, unknown media types normalize to `application/octet-stream`, and remote request defaults derive `Accept` / `ContentType` from the resolved payload descriptor when metadata leaves them unset.

### Repository Payload
67. Mixed payload repository contract: repository-backed read/write/list/delete flows resolve `resource.*` files by discovered suffix or metadata `payloadType`, reject duplicate payload files for one logical path, read `metadata.yaml` or `metadata.json` sidecars with YAML precedence, write `metadata.yaml` sidecars by default while removing superseded sibling sidecars, preserve opaque input-file suffixes such as `.key` for octet-stream saves, and fall back to `.bin` only when no suffix or stronger payload hint is available.

### Binary Payload
68. Binary payload contract: `application/octet-stream` request and response bodies use `resource.BinaryValue`, `--output auto|text` writes raw bytes for one binary payload without a trailing newline, structured output wraps binary content as base64 with media metadata, multi-item binary auto/text output is rejected, and `resource edit` rejects binary payloads.
69. Binary metadata-mutation contract: body-bearing metadata `transforms` steps run only for structured payloads, while raw text or octet-stream request bodies remain unmodified after metadata rendering.

### E2E Component Contracts
70. E2E component contract validation mode: `run-e2e.sh --validate-components` validates component manifest contract versioning, hook-script syntax, dependency catalog consistency, and managed-server fixture identity metadata (`resource.id` / `resource.alias`) before workload execution, while fast Bash harness tests validate hook semantic contracts for state publication, deterministic context fragments, and repeated hook execution.
71. Plain-text-only CLI commands (`secret get`, `repository tree`, `server get *`, `server check`, shell `completion` subcommands, and `context print-template`) reject `--output json|yaml` with `ValidationError` instead of silently ignoring structured output requests.

### Exit Codes and Output Format
72. CLI exit-code mapping contract: typed errors map to deterministic non-zero process exit codes (`Validation`, `NotFound`, `Auth`, `Conflict`, `Transport`) while untyped/internal errors retain the generic failure code.
73. Repository history CLI contract: `repository history` returns a deterministic not-supported text message for filesystem contexts, supports git-only local history filters (`max-count`, `author`, `grep`, `since`, `until`, repeatable `path`, `reverse`, `oneline`), and forwards parsed filters to the repository history capability.
74. Repository tree CLI contract: `repository tree` prints a deterministic tree-style directory listing, omits files, omits hidden control directories and reserved `_/` metadata directories, and preserves spaces in directory names.
75. Resource list text output contract: `resource list --output auto|text` prints deterministic `<alias> (<id>)` lines, prefers metadata-derived alias/id values, and falls back when metadata identity attributes are absent.
76. Resource mutation inline payload grammar: `resource apply|create|save|update` accept explicit `--payload` forms for structured formats (file path, `-` stdin, inline JSON/YAML object text, JSON Pointer assignments) with negative coverage for malformed inline payloads, while binary payloads are file-or-stdin only.

### Git Auto-commit
77. Resource git auto-commit contract: `resource save` and repository-backed `resource delete` on git contexts create local commits after successful repository mutation, treat `--message` as an override-only flag, preserve remote-only delete behavior (no repository commit), fail empty `--message` values with `ValidationError`, and `resource save --push` pushes the save commit even when `repository.git.remote.autoSync` is disabled while failing with `ValidationError` for filesystem/no-remote contexts.
78. Resource auto-commit worktree safeguard: `resource save|delete|edit` fail with `ValidationError` before mutation when git worktree has unrelated uncommitted changes.
79. Git repository auto-init contract: git-backed repository status/history/check and git-backed repository mutation commit/status flows initialize a missing local `.git/` repository automatically and continue with normal operation semantics (including empty-history handling for fresh repos).

### Payload Validation
80. Operation payload validation contract: `resource create|update` and metadata-resolved `resource request post|put|patch` enforce `resource.requiredAttributes` plus every JSON Pointer referenced by `resource.alias` against the structured pre-transform resource payload; non-create mutations also enforce every JSON Pointer referenced by `resource.id`, while create MAY omit metadata-derived `resource.id` pointers unless they are explicitly declared by `resource.requiredAttributes` or operation validation; the same workflows then enforce `validate.requiredAttributes`, jq `validate.assertions`, and OpenAPI-backed `validate.schemaRef` against the outgoing structured payload; path-derived template fields satisfy operation required attributes without mutating the transmitted body, validation failures short-circuit before remote HTTP execution, and raw text/octet-stream payloads skip resource-attribute presence checks while still rejecting structured operation validation rules.

### E2E Platform and Runtime
81. E2E metadata source contract: `run-e2e.sh` defaults `--metadata-source` to `bundle`; legacy `--metadata-type base-dir` normalizes to `dir`; `bundle` mode skips component-local `openapi.yaml` wiring and uses shorthand `metadata.bundle` when mapped; `dir` mode uses a run-scoped copy of the component-local `metadata/` tree as `metadata.base-dir` and keeps local OpenAPI wiring.
82. E2E platform contract: `run-e2e.sh` supports `--platform <compose|kubernetes>` with default `kubernetes`, rejects invalid platform values, and cleanup mode rejects mixed workload flags (including `--platform`) with deterministic parser errors.
83. E2E containerized artifact contract: compose-runtime components require `compose/compose.yaml` and `k8s/*.yaml`; native components remain valid without runtime artifact directories.
84. E2E kubernetes runtime contract: local containerized components on `--platform kubernetes` use run-scoped kind clusters, apply rendered manifests, create service-annotation-driven port-forwards, persist runtime/forward metadata for cleanup/manual handoff, and stop logic terminates recorded port-forward processes.
85. E2E kind provider compatibility contract: podman engine selection enforces `KIND_EXPERIMENTAL_PROVIDER=podman` for kind operations, preflight validates provider readiness, and cleanup deletes recorded run clusters for `--clean`/`--clean-all`.
86. E2E managed-server flag contract: runner selection flags use the `--managed-server*` namespace (`--managed-server`, `--managed-server-connection`, `--managed-server-auth-type`, `--managed-server-mtls`) and reject legacy `--managed-server*` variants.
87. E2E managed-server proxy contract: `--managed-server-proxy` toggles context proxy injection (`managedServer.http.proxy`) and fails validation when enabled without at least one configured proxy URL (`DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL` or `DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL`).

### Throttling and Operator
88. Managed-server request throttling contract: configured `max-concurrent-requests` and `queue-size` enforce bounded request admission, overflow returns typed `ConflictError`, shared throttling scopes apply across concurrent clients for one managed-server identity, and proxy environment variables merge with configured managed-server proxy fields unless an explicit empty proxy block disables them.
89. SyncPolicy overlap contract: multiple SyncPolicies MAY share repository/managed-server/secret-store references, but overlapping source path/subpath scopes MUST fail deterministically even when references differ.
90. Repository webhook security contract: git webhook receiver rejects invalid signatures/tokens and unsupported events, enforces payload size bounds plus bounded read/write/idle HTTP timeouts, and only authenticated push events patch repository webhook-receipt annotations.
91. Operator webhook integration contract: operator profiles configure provider webhooks (`gitea|gitlab`) and webhook-backed pushes trigger reconcile/update behavior before the repository poll interval in operator-main coverage.
93. Operator reconcile planning contract: sync planning deterministically selects `full` vs `incremental`, metadata/unknown-path diffs trigger safe fallback to full scope, collection metadata defaults diffs under `/_/defaults.<ext>` resolve to the affected collection apply target, unsupported per-resource `defaults.<ext>` diffs trigger safe full fallback, and secret-version-hash changes force reconcile even when repository revision is unchanged.

## Failure Modes
1. Tests pass locally with hidden non-determinism.
2. Changed behavior lacks regression coverage.
3. Security-sensitive paths bypass required safeguards.
4. Snapshot/log artifacts leak secret values.

## Edge Cases
1. Suppression removes all comparable fields.
2. Equivalent secret placeholders normalize differently.
3. Non-unique alias appears during apply/refresh.
4. Remote workflow runs with partially configured context.

## Examples
1. Unit test verifies deterministic metadata merge order across wildcard and literal layers.
2. Integration test asserts `ConflictError` when repository sync detects divergence.
3. E2E test validates collection delete safety gates and deterministic summary output.
4. CLI test verifies `metadata render` fallback from defaulted `get` to `list` when `get` has no path and validates completion scripts do not emit duplicate `--flag`/`--flag=` suggestions.
5. CLI test verifies `metadata infer --apply` persists the same compact payload printed to output and that `metadata get` default full-shape output vs `--overrides-only` compact output (including endpoint-gated fallback inference) are preserved.
6. CLI + repository tests verify `repository tree` text output is deterministic, rejects structured output, omits files/hidden or reserved directories, and preserves directory names containing spaces.
7. CLI test verifies explicit payload collection-target inference (`resource create /admin/realms --payload realm=test`) resolves the concrete child path from metadata alias/id hints while the equivalent explicit resource path (`/admin/realms/test`) continues to pass identity validation.
8. Bundle metadata tests verify OpenAPI fallback discovery order when `managedServer.http.openapi` is empty: `declarest.openapi` hint, bundle-root/metadata-root OpenAPI files, then deterministic recursive bundle-file fallback.
9. E2E harness tests verify both platform paths (`--platform compose` and `--platform kubernetes`), including a corner case where `--platform kubernetes` with remote/native-only selections skips kind cluster creation.
10. Unit and harness tests verify metadata sidecars support `metadata.yaml` and `metadata.json`, prefer YAML when both exist, and surface selected managed-server access details in manual-profile handoff output even without a component `manual-info` hook.
11. E2E harness tests verify operator profile defaulting/validation (`repo-type=git`, `git-provider=gitea`) and in-cluster operator deployment lifecycle behavior (runtime details, handoff commands, automated reconcile cases, and cleanup/teardown).
12. E2E harness tests verify summary execution parameters report explicit selections, component-elected auth defaults, and operator profile defaults deterministically.
13. Unit + integration tests verify managed-server requestThrottling validation, shared-gate queue overflow behavior, and repository webhook authentication/annotation patch behavior for valid vs invalid payloads.
14. Operator unit tests verify sync-plan fallbacks, schedule computation (`syncInterval` vs `fullResyncCron`), and overlap validation behavior with shared vs distinct dependency references.
15. Unit + CLI + integration tests verify `resource save --secret` stores whole-resource payloads under `<logical-path>:.`, preserves repository suffixes and root placeholders (including octet-stream files), persists `resource.secret: true`, auto-reuses metadata-driven whole-resource secret handling on later saves, rejects incompatible flags or missing secret/metadata providers, lets repository reads plus remote apply/diff resolve the whole-resource placeholder back to the original payload, and rejects missing path-like `--payload` inputs instead of persisting the literal path text as opaque content.
16. Unit tests verify `schemas/context.schema.json`, `schemas/contexts.schema.json`, and `schemas/metadata.schema.json` stay valid JSON and preserve the expected top-level contract wiring for repository/context/metadata consumers.
17. Repository, orchestrator, CLI, and operator tests verify `defaults.<ext>` merge/compaction semantics, supported codec coverage (`json|yaml|ini|properties`), rejection of legacy per-resource defaults layouts, unsupported payload-type rejection, local edit/copy/save preservation of compact overrides, raw defaults CLI flows (`get|edit|infer --save|infer --check`) including codec selection and stable empty-object inference, `resource get|save --prune-defaults` behavior, managed-server probe cleanup, and incremental plan classification for add/modify/delete of defaults sidecars.
