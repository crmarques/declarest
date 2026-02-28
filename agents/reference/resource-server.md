# Resource Server, HTTP, and OpenAPI

## Purpose
Define remote server interaction contracts, request generation rules, and OpenAPI-assisted behavior.

## In Scope
1. Remote CRUD/list/existence semantics.
2. Request construction from metadata.
3. Auth and TLS configuration contract.
4. OpenAPI-based defaults and validation support.

## Out of Scope
1. Local repository persistence details.
2. Secret masking internals.
3. CLI completion internals.

## Normative Rules
1. Remote operations MUST be executed through `server.ResourceServer` only.
2. Request method, path, query, and headers MUST derive from resolved metadata plus explicit overrides.
3. Auth mode precedence MUST be deterministic and documented.
4. TLS configuration errors MUST fail fast during initialization.
5. HTTP response errors MUST preserve status code and response body context.
6. OpenAPI-derived defaults SHOULD improve request correctness but MUST NOT override explicit metadata unless requested.
7. List responses MUST be normalized into deterministic `resource.Resource` ordering.
8. `operationInfo.listCollection.jq` (or resolved list-operation `jq`) MUST be executed against decoded list payload before list-shape extraction and identity mapping.
9. List-operation `jq` expressions MAY call `resource("<logical-path>")`; resolution MUST use a context-provided logical-path resolver when available.
10. When no logical-path resolver is provided, `resource("<logical-path>")` MUST fail with a validation error.
11. Within one `jq` evaluation, repeated `resource("<logical-path>")` calls MUST be cached by path, and invalid arguments or cyclic resolver dependencies MUST fail with validation errors.
12. When metadata does not explicitly set `Accept`, remote operation requests MUST default to `application/<repository.resource-format>` (`json` when omitted); body-bearing operations (`create|update`) MUST apply the same default for `ContentType` when unset.
13. Before sending body-bearing requests, operation validation directives (`validate.requiredAttributes`, `validate.assertions`, `validate.schemaRef`) MUST be evaluated against the outgoing payload.
14. Payload validation context MUST include path-derived template fields (for example `realm` from `/admin/realms/<realm>/...`) without mutating the outgoing request body.

## Data Contracts
Request spec fields:
1. `Method`.
2. `Path`.
3. `Query` map.
4. `Headers` map.
5. `Accept`.
6. `ContentType`.
7. `Body` resource payload.
8. Optional `Validate` operation validation directives.

Server interface operations:
1. `Get/Create/Update/Delete/List/Exists`.
2. `GetOpenAPISpec`.

Auth modes:
1. OAuth2.
2. Custom header token.
3. Bearer token.
4. Basic auth.

## Failure Modes
1. Auth failure due to missing or invalid credentials.
2. Timeout or transport interruption.
3. OpenAPI document unavailable or invalid.
4. Metadata path template resolves to invalid URI.

## Edge Cases
1. 404 on direct path with viable alias/list fallback strategy.
2. Non-unique alias returned by list operation.
3. Server returns non-JSON payload for configured JSON operation.
4. OpenAPI path exists but method unsupported for operation type.
5. Validation schema reference is configured but OpenAPI request-body/schema pointer cannot be resolved.

## Examples
1. `Get` operation uses `operationInfo.getResource.path` plus default `Accept: application/<repository.resource-format>` (for example `application/yaml` in YAML repositories).
2. `Update` operation resolves `ContentType` from metadata or defaults to `application/<repository.resource-format>` and sends normalized payload body.
3. `List` operation hydrates `resource.Resource` for each item with inferred alias and remote ID.
4. `List` operation `jq` can filter by parent references (for example `.parentId == (resource("/admin/realms/platform/user-registry/ldap-test") | .id)`).
5. `Create` operation validation can require `realm` while resolving `realm` implicitly from `/admin/realms/<realm>/...` logical paths.
