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
12. Inference MUST accept metadata selector paths containing intermediary `_` segments and trailing collection markers (for example `/admin/realms/_/clients/`).
13. Selector-path inference SHOULD use OpenAPI path templates when available to infer operation paths and identity attributes, and non-template-safe OpenAPI parameter names MUST fall back to deterministic placeholder names from fallback inference.
14. Inference output SHOULD omit directives that are equal to deterministic fallback defaults so CLI responses focus on meaningful overrides.
15. Until recursive metadata inference traversal is implemented, inference requests with `recursive=true` MUST return a typed validation error and MUST NOT persist metadata changes.
16. `resourceInfo.collectionPath` templates MUST support indirection by resolving template fields from the handled logical path when payload attributes are absent.
17. Operation paths starting with `.` (for example `.` or `./{{.id}}`) MUST resolve relative to the rendered effective collection path.
18. When an operation path is omitted, defaults MUST be `.` for `create` and `list`, and `./{{.id}}` for `get`, `update`, `delete`, and `compare`.
19. Metadata decoding SHOULD accept `operationInfo.<operation>.url.path` as a compatibility alias for `operationInfo.<operation>.path`.
20. List-operation `jq` expressions MAY call `resource("<logical-path>")`; when used, resolution MUST target the same active source as the primary list workflow and return normalized JSON payload.
21. Metadata template-rendered string fields MUST support `{{resource_format .}}`, which resolves to the active repository resource format (`json` or `yaml`) and defaults to `json` when the configured format is empty.
22. Default metadata operation media directives SHOULD use repository-format-aware templates in `httpHeaders` entries (`Accept: application/{{resource_format .}}` for all default operations and `Content-Type: application/{{resource_format .}}` for `create|update`).
23. Operation validation directives (`validate.requiredAttributes`, `validate.assertions`, `validate.schemaRef`) MUST be preserved through metadata merge/render/serialization and MUST remain operation-scoped.
24. OpenAPI-backed inference SHOULD populate `operationInfo.createResource/updateResource.validate.schemaRef` as `openapi:request-body` when request-body schemas exist and MAY populate `validate.requiredAttributes` from deterministic schema `required` fields.

## Data Contracts
Supported metadata groups:
1. `resourceInfo`: identity, secret-attribute, and collection directives.
2. `operationInfo.createResource/updateResource/deleteResource/getResource/compareResources/listCollection`: operation-specific directives.
3. `operationInfo.defaults`: shared transform defaults applied before operation-specific overrides.
4. Operation wire fields: `path`, `httpMethod`, `query`, `httpHeaders`, `body` (including media headers such as `Accept` and `Content-Type` as `httpHeaders` entries).
5. Transform wire fields: `payload.filterAttributes`, `payload.suppressAttributes`, `payload.jqExpression` (with compare-specific top-level fields `compareResources.filterAttributes|suppressAttributes|jqExpression`).
6. Operation validation wire fields: `validate.requiredAttributes`, `validate.assertions[*].message`, `validate.assertions[*].jq`, `validate.schemaRef`.
7. Resource-level secret detection fields: `secretInAttributes`.

Operation selector contract:
1. API boundaries MUST use typed `metadata.Operation` values.
2. Allowed operation values are `get`, `create`, `update`, `delete`, `list`, and `compare`.
3. Operation path fields MAY be provided as either `path` or compatibility `url.path`.
4. Metadata decoding SHOULD accept compatibility aliases `method`, `headers`, `filter`, `suppress`, and `jq` while canonical persistence uses `httpMethod`, `httpHeaders`, and `payload.*` transform fields.

Infer options contract:
1. `apply`: whether inferred directives are persisted.
2. `recursive`: reserved for future traversal support; current behavior MUST reject `true` with a validation error.

Template context contract:
1. Current resource payload fields.
2. Ancestor resource payload fields.
3. Context attributes: logical path, collection path, alias, remote ID.
4. Relative references allowed with `../` traversal semantics bound to ancestor levels.
5. Helper function `resource_format` with root-scope call form `{{resource_format .}}`.

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
5. `secretInAttributes` points to missing payload fields and SHOULD not fail metadata resolution.
6. Metadata update writes from CLI commands remove nil keys while keeping explicit empty arrays/maps.
7. Selector-path inference without OpenAPI data still returns deterministic fallback metadata hints.
8. Collection-path indirection uses selector/logical-path-derived attributes (for example `{{.realm}}`) even when the payload omits those attributes.
9. Relative operation paths resolve against rendered collection paths and keep compatibility for non-relative legacy values (for example `customers` => `/customers`).
10. `resource("<logical-path>")` lookups used by list `jq` can resolve parent resources through metadata-aware alias/id fallback and then filter candidates deterministically by referenced fields.
11. Invalid metadata template helper usage (for example `{{resource_format "yaml"}}`) returns a typed validation error.

## Examples
1. `/customers/_` defines `operationInfo.getResource.path: /api/customers/{{.id}}`; `/customers/acme/metadata` overrides only `operationInfo.getResource.httpHeaders`.
2. `operationInfo.compareResources.suppressAttributes` includes `/updatedAt` and `/version`; diff output excludes these fields.
3. `operationInfo.listCollection.path` inferred from OpenAPI, then manually overridden with custom query defaults.
4. Inference for `/admin/realms/_/clients/` can propose `resourceInfo.idFromAttribute: id`, `resourceInfo.aliasFromAttribute: clientId`, and templated operation paths from OpenAPI selectors.
5. For selector `/admin/realms/_/user-registry` with `resourceInfo.collectionPath: /admin/realms/{{.realm}}/components` and `operationInfo.getResource.path: ./{{.id}}`, rendering `/admin/realms/platform/user-registry` with `id=123456` resolves to `/admin/realms/platform/components/123456`.
6. For selector `/admin/realms/_/user-registry/_/mappers/`, `operationInfo.listCollection.payload.jqExpression` MAY use `resource("/admin/realms/{{.realm}}/user-registry/{{.provider}}/")` and compare mapper `parentId` with the resolved parent `.id`.
7. `metadata infer /admin/realms/ --recursive` MUST fail with a validation error and MUST NOT write metadata files until recursive traversal is implemented.
8. `metadata get` resolves `{{resource_format .}}` tokens in metadata string fields to `application/json` or `application/yaml` based on repository format while preserving unrelated templates such as `{{.id}}`.
9. `operationInfo.createResource.validate.requiredAttributes: ["realm"]` is satisfied for `/admin/realms/platform/...` when `realm` is derived from the logical path template context.
