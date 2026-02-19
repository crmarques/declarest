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
13. Selector-path inference SHOULD use OpenAPI path templates when available to infer operation paths and identity attributes.

## Data Contracts
Supported metadata groups:
1. `resourceInfo`: identity, secret-attribute, and collection directives.
2. `operations.get/create/update/delete/list/compare`: operation-specific directives.
3. Operation fields: `path`, `method`, `query`, `headers`, `contentType`, `accept`.
4. Transform fields: `filter`, `suppress`, `jq`.
5. Resource-level secret detection fields: `secretsFromAttributes`.

Operation selector contract:
1. API boundaries MUST use typed `metadata.Operation` values.
2. Allowed operation values are `get`, `create`, `update`, `delete`, `list`, and `compare`.

Infer options contract:
1. `apply`: whether inferred directives are persisted.
2. `recursive`: whether inference traverses descendants.

Template context contract:
1. Current resource payload fields.
2. Ancestor resource payload fields.
3. Context attributes: logical path, collection path, alias, remote ID.
4. Relative references allowed with `../` traversal semantics bound to ancestor levels.

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
5. `secretsFromAttributes` points to missing payload fields and SHOULD not fail metadata resolution.
6. Metadata update writes from CLI commands remove nil keys while keeping explicit empty arrays/maps.
7. Selector-path inference without OpenAPI data still returns deterministic fallback metadata hints.

## Examples
1. `/customers/_` defines `operations.get.path: /api/customers/{{.id}}`; `/customers/acme/metadata` overrides only headers.
2. `operations.compare.suppress` includes `/updatedAt` and `/version`; diff output excludes these fields.
3. `operations.list.path` inferred from OpenAPI, then manually overridden with custom query defaults.
4. Inference for `/admin/realms/_/clients/` can propose `idFromAttribute: id`, `aliasFromAttribute: clientId`, and templated operation paths from OpenAPI selectors.
