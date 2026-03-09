# Metadata Semantics, Layering, and Templates

## Purpose
Define deterministic metadata behavior for operation routing, transform rules, and template rendering.

## In Scope
1. Metadata structure and supported directives.
2. Layering and precedence algorithm.
3. Template rendering and context resolution.
4. Metadata inference from OpenAPI hints.

## Out of Scope
1. HTTP transport implementation internals.
2. Secret store backend behavior.
3. CLI output formatting specifics.

## Normative Rules
1. Metadata resolution MUST be deterministic and reproducible.
2. Precedence MUST be: defaults, inherited collection layers, wildcard layers, literal layers, resource layer.
3. At the same depth, wildcard candidates MUST be applied before literal candidates when establishing defaults.
4. At the same precedence level, merge order MUST be stable and lexicographic.
5. Arrays MUST replace existing arrays and MUST NOT deep-merge.
6. Resource-level directives MUST override inherited directives.
7. Template rendering failures MUST return typed validation errors with path context.
8. Compare transforms MUST run before diff generation.
9. Inference from OpenAPI SHOULD propose method/path defaults but MUST NOT overwrite explicit user metadata unless explicitly requested.
10. Metadata persistence MUST omit nil directive fields from stored files instead of writing `null`.
11. Metadata persistence MUST preserve explicit empty arrays/maps when they are provided for replacement semantics.
12. Metadata persistence MUST read selector sidecars from `metadata.yaml` or `metadata.json`, MUST prefer `metadata.yaml` when both exist, and SHOULD write `metadata.yaml` by default while removing a superseded sibling sidecar after successful persistence.
13. Inference MUST accept metadata selector paths containing intermediary `_` segments and trailing collection markers (for example `/admin/realms/_/clients/`).
14. Selector-path inference SHOULD use OpenAPI path templates when available to infer operation paths and identity attributes, and non-template-safe OpenAPI parameter names MUST fall back to deterministic placeholder names from fallback inference.
15. Inference output SHOULD omit directives that are equal to deterministic fallback defaults so CLI responses focus on meaningful overrides.
16. Until recursive metadata inference traversal is implemented, inference requests with `recursive=true` MUST return a typed validation error and MUST NOT persist metadata changes.
17. `resource.collectionPath` templates MUST support indirection by resolving template fields from the handled logical path when payload attributes are absent.
18. Operation paths starting with `.` (for example `.` or `./{{.id}}`) MUST resolve relative to the rendered effective collection path.
19. When an operation path is omitted, defaults MUST be `.` for `create` and `list`, and `./{{.id}}` for `get`, `update`, `delete`, and `compare`.
20. List-operation `transforms[*].jqExpression` entries MAY call `resource("<logical-path>")`; when used, resolution MUST target the same active source as the primary list workflow and return normalized JSON payload.
21. Metadata template-rendered string fields MUST support `{{payload_type .}}`, `{{payload_media_type .}}`, and `{{payload_extension .}}`, which resolve from the active resource payload descriptor.
22. Metadata template scopes MUST expose `contentType` as the active payload media type when the payload itself does not already define `contentType`, so media-aware templates work for raw text or octet-stream payloads.
23. Metadata defaults MUST leave media header selection to payload-type-aware request building unless explicit metadata overrides are present.
24. Operation validation directives (`validate.requiredAttributes`, `validate.assertions`, `validate.schemaRef`) MUST be preserved through metadata merge/render/serialization and MUST remain operation-scoped.
25. OpenAPI-backed inference SHOULD populate `operations.create/update.validate.schemaRef` as `openapi:request-body` when request-body schemas exist and MAY populate `validate.requiredAttributes` from deterministic schema `required` fields.
24. `resource.payloadType` MAY override filename-derived payload inference for one resource or collection scope and MUST support `json`, `yaml`, `xml`, `hcl`, `ini`, `properties`, `text`, and `octet-stream`.
25. `resource.secret: true` MAY declare one whole-resource secret save/read contract for that scope, MUST remain distinct from attribute-scoped secret masking, and MUST be mutually exclusive with `resource.secretAttributes`.
26. Metadata attribute references (`resource.idAttribute`, `resource.aliasAttribute`, `resource.secretAttributes[*]`, `resource.externalizedAttributes[*].path`, `transforms[*].selectAttributes`, `transforms[*].excludeAttributes`, and `validate.requiredAttributes[*]`) MUST use RFC 6901 JSON Pointer strings.
27. When `resource.payloadType` resolves to a non-structured payload type, `resource.idAttribute`, `resource.aliasAttribute`, `resource.secretAttributes`, and `resource.externalizedAttributes` MUST fail validation because they depend on structured payload traversal.
28. `resource.externalizedAttributes` MUST default unspecified `template|mode|saveBehavior|renderBehavior|enabled` fields deterministically and MUST validate duplicate enabled `path` or `file` entries before persistence or workflow use.
29. Enabled `resource.externalizedAttributes` entries MUST treat `path` as one JSON Pointer traversal path, MUST traverse object keys by pointer token, MUST traverse arrays only through zero-based numeric tokens or `*` wildcard tokens, MUST reject empty paths/files and repository-escaping relative files, MUST externalize only text/string payload values in MVP scope, and MUST leave disabled entries inert.
30. When an enabled externalized-attribute `path` uses one or more `*` wildcard array tokens, repository workflows MUST materialize concrete artifact file names deterministically by appending matched wildcard indices before the configured file extension (for example `script.sh` -> `script-0.sh`), and placeholder rendering/expansion MUST use that concrete file path.
31. Repository-backed payload workflows (`save`, `apply`, `create`, `update`, `diff`) MUST replace configured include placeholders with sidecar file contents before downstream payload transforms or identity resolution when the stored payload value matches the configured placeholder template exactly.

## Data Contracts
Supported metadata groups:
1. `resource`: identity, payload-type, whole-resource-secret, secret-attribute, and collection directives.
2. `resource.externalizedAttributes[*]`: sidecar payload directives (`path`, `file`, optional `template|mode|saveBehavior|renderBehavior|enabled`), where `path` is one JSON Pointer string and arrays use numeric or `*` wildcard tokens.
3. `operations.create/update/delete/get/compare/list`: operation-specific directives.
4. `operations.defaults.transforms`: shared ordered transform pipeline applied before operation-specific pipelines.
5. Operation wire fields: `path`, `method`, `query`, `headers`, `body` (including media headers such as `Accept` and `Content-Type` as `headers` entries).
6. Transform wire fields: `transforms[*].selectAttributes`, `transforms[*].excludeAttributes`, `transforms[*].jqExpression`.
7. Operation validation wire fields: `validate.requiredAttributes`, `validate.assertions[*].message`, `validate.assertions[*].jq`, `validate.schemaRef`.
8. Resource-level secret detection fields: `secret`, `secretAttributes`.

Operation selector contract:
1. API boundaries MUST use typed `metadata.Operation` values.
2. Allowed operation values are `get`, `create`, `update`, `delete`, `list`, and `compare`.

Infer options contract:
1. `apply`: whether inferred directives are persisted.
2. `recursive`: reserved for future traversal support; current behavior MUST reject `true` with a validation error.

Template context contract:
1. Current resource payload fields.
2. Ancestor resource payload fields.
3. Context attributes: logical path, collection path, alias, remote ID.
4. Relative references allowed with `../` traversal semantics bound to ancestor levels.
5. Helper functions `payload_type`, `payload_media_type`, and `payload_extension` with root-scope call form `{{... .}}`.
6. Compatibility alias `contentType` populated from the active payload media type when the payload map does not already define `contentType`.

## Layering Algorithm
1. Start with engine defaults.
2. Collect matching collection metadata candidates from root to target collection.
3. For each depth, apply wildcard then literal candidate in stable order.
4. Apply target resource metadata last.
5. Normalize merged result and validate required fields for requested operation.

## Failure Modes
1. Missing required operation path after all layers resolve.
2. Invalid template variables or unresolved relative references.
3. Transform expressions that produce invalid payload shapes.
4. Conflicting metadata causing ambiguous identity resolution.

## Edge Cases
1. Wildcard metadata matches multiple nested collections with partial overrides.
2. Relative template references exceed available ancestor depth.
3. Compare suppression removes all fields and yields empty normalized payload.
4. Explicit null directive intended to clear inherited field.
5. `secretAttributes` points to missing payload fields and SHOULD not fail metadata resolution.
6. Metadata update writes from CLI commands remove nil keys while keeping explicit empty arrays/maps, default to `metadata.yaml`, and still read legacy `metadata.json` sidecars.
7. Selector-path inference without OpenAPI data still returns deterministic fallback metadata hints.
8. Collection-path indirection uses selector/logical-path-derived attributes (for example `{{.realm}}`) even when the payload omits those attributes.
9. `resource("<logical-path>")` lookups used by list `jq` can resolve parent resources through metadata-aware alias/id fallback and then filter candidates deterministically by referenced fields.
10. Invalid metadata template helper usage (for example `{{payload_type "yaml"}}`) returns a typed validation error.
11. Raw octet-stream or text payloads can still render templates that read `contentType`; when the payload is not a map object, that alias resolves from the active payload descriptor instead of failing on a missing key.
12. `payloadType: octet-stream` disables structured payload transforms and validation rules for that scope.
13. `payloadType: text` or `payloadType: octet-stream` can still use `resource.secret: true`, but `resource.secretAttributes`, identity attributes, and externalized attributes fail validation for that scope.
14. Externalized-attribute file paths containing `../` or duplicate enabled `file`/`path` entries fail metadata validation deterministically before repository IO.
15. Wildcard array externalization can skip elements that do not contain the targeted attribute while still materializing indexed sidecars for matching siblings only.
16. Repository payloads MAY keep inline values for configured externalized attributes; expansion only occurs when the stored value matches the configured placeholder string exactly.

## Examples
1. `/customers/_` defines `operations.get.path: /api/customers/{{.id}}`; `/customers/acme/metadata` overrides only `operations.get.headers`.
2. `operations.compare.transforms: [{excludeAttributes:["/updatedAt","/version"]}]` excludes those fields from diff output.
3. `operations.list.path` inferred from OpenAPI, then manually overridden with custom query defaults.
4. Inference for `/admin/realms/_/clients/` can propose `resource.idAttribute: /id`, `resource.aliasAttribute: /clientId`, and templated operation paths from OpenAPI selectors.
5. For selector `/admin/realms/_/user-registry` with `resource.collectionPath: /admin/realms/{{.realm}}/components` and `operations.get.path: ./{{.id}}`, rendering `/admin/realms/platform/user-registry` with `id=123456` resolves to `/admin/realms/platform/components/123456`.
6. For selector `/admin/realms/_/user-registry/_/mappers/`, `operations.list.transforms: [{jqExpression:"..."}]` MAY use `resource("/admin/realms/{{.realm}}/user-registry/{{.provider}}/")` and compare mapper `parentId` with the resolved parent `.id`.
7. `metadata infer /admin/realms/ --recursive` MUST fail with a validation error and MUST NOT write metadata files until recursive traversal is implemented.
8. `metadata get` resolves payload-aware helper tokens in metadata string fields while preserving unrelated templates such as `{{.id}}`.
9. `operations.create.validate.requiredAttributes: ["/realm"]` is satisfied for `/admin/realms/platform/...` when `realm` is derived from the logical path template context.
10. OpenAPI inference for an endpoint with `application/octet-stream` request or response media infers `resource.payloadType: octet-stream` when explicit metadata is absent.
10. `resource.externalizedAttributes: [{path:"/script", file:"script.sh"}]` plus `resource.yaml script: "{{include script.sh}}"` stores script content in a sibling `script.sh` file and expands that file back into the effective payload for apply/diff.
11. `resource.externalizedAttributes: [{path:"/sequence/commands/*/script", file:"script.sh"}]` plus a payload with script commands stores placeholders such as `{{include script-0.sh}}` and `{{include script-2.sh}}` for the matching array elements only.
12. When `resource.yaml` contains `script: "{{include script.sh}}"` but `script.sh` is absent, repository-backed mutation workflows fail with a typed validation error before remote HTTP execution.
13. `resource.payloadType: text` plus `resource.secret: true` is valid for a whole-file secret, while `resource.secretAttributes: ["/password"]` at that same scope fails validation because text payloads are not structured.
13. Rundeck-style metadata can render `{{index . "contentType"}}` for a raw `resource.key` payload because the template scope injects `contentType: application/octet-stream` from the active descriptor.
