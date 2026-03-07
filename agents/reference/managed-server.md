# Managed Server, HTTP, and OpenAPI

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
1. Remote operations MUST be executed through `managedserver.ManagedServerClient` only.
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
12. When metadata does not explicitly set `Accept`, remote operation requests MUST default to the resolved payload type media mapping (`application/octet-stream` for `octet-stream`, `text/plain` for text-like codecs, and repository-default mapping when payload type cannot be inferred); body-bearing operations (`create|update`) MUST apply the same default for `ContentType` when unset.
13. Before sending body-bearing requests, operation validation directives (`validate.requiredAttributes`, `validate.assertions`, `validate.schemaRef`) MUST be evaluated against the outgoing payload only for structured payloads and MUST fail fast with `ValidationError` for `octet-stream`.
14. Payload validation context MUST include path-derived template fields (for example `realm` from `/admin/realms/<realm>/...`) without mutating the outgoing request body.
15. OpenAPI document URLs MAY be cross-origin relative to `managed-server.http.base-url`, but authentication headers MUST only be attached for same-origin OpenAPI fetches.
16. Managed-server OpenAPI sources MUST accept OpenAPI 3.x (`openapi`) and Swagger 2.0 (`swagger`) documents; Swagger 2.0 operations MUST be normalized for media default inference and `validate.schemaRef=openapi:request-body` compatibility.
17. When `managed-server.http.request-throttling` is configured, request execution MUST enforce bounded in-flight concurrency and queue capacity, MUST reject overflow with typed conflict errors, and SHOULD share throttling scope for identical managed-server identities.
18. `application/octet-stream` responses MUST decode to `resource.BinaryValue`, and auto/text CLI output for one binary payload MUST write raw bytes without a trailing newline.

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
2. `Request`.
3. `GetOpenAPISpec`.

OpenAPI document compatibility:
1. `openapi: 3.x` documents use `requestBody.content` and response `content` directly.
2. `swagger: 2.0` documents use `parameters[in=body].schema`, `consumes`, `produces`, and response `schema`; runtime normalization MUST expose equivalent `requestBody.content` and response `content` semantics.

Auth modes:
1. OAuth2.
2. Custom headers list (`custom-headers`).
3. Basic auth.

Request throttling fields:
1. `max-concurrent-requests`.
2. `queue-size`.
3. `requests-per-second`.
4. `burst`.

## Failure Modes
1. Auth failure due to missing or invalid credentials.
2. Timeout or transport interruption.
3. OpenAPI document unavailable or invalid.
4. Metadata path template resolves to invalid URI.
5. Request queue is full and additional requests are rejected with `ConflictError`.

## Edge Cases
1. 404 on direct path with viable alias/list fallback strategy.
2. Non-unique alias returned by list operation.
3. Server returns non-JSON payload for configured JSON operation.
4. OpenAPI path exists but method unsupported for operation type.
5. Validation schema reference is configured but OpenAPI request-body/schema pointer cannot be resolved.
6. Swagger 2.0 operation omits `parameters[in=body].schema`; `validate.schemaRef=openapi:request-body` fails with `ValidationError`.
7. Two concurrent sync workflows targeting the same managed server share one throttle scope and one queue budget.
8. OpenAPI advertises `application/octet-stream` or `format: binary`, and the resolved payload type becomes `octet-stream`.
9. Raw request execution uses metadata-rendered `Accept` and `Content-Type` instead of falling back to JSON-only defaults.

## Examples
1. `Get` operation uses `operationInfo.getResource.path` plus payload-type-aware default `Accept`.
2. `Update` operation resolves `ContentType` from metadata or payload-type defaults and sends structured or opaque payload body accordingly.
3. `List` operation hydrates `resource.Resource` for each item with inferred alias and remote ID.
4. `List` operation `jq` can filter by parent references (for example `.parentId == (resource("/admin/realms/platform/user-registry/ldap-test") | .id)`).
5. `Create` operation validation can require `realm` while resolving `realm` implicitly from `/admin/realms/<realm>/...` logical paths.
6. Swagger 2.0 `consumes/produces` plus body `parameters` support `Create` fallback defaults (`ContentType`/`Accept`) and `openapi:request-body` validation without requiring OpenAPI 3 syntax.
7. With `request-throttling.max-concurrent-requests=1` and `queue-size=1`, a third concurrent request fails fast with `ConflictError` while two earlier requests are in-flight/queued.
8. `Get` of a binary certificate endpoint returns `resource.BinaryValue` when the server responds `application/octet-stream`.
