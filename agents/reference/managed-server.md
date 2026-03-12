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
4. Managed-server debug output MUST redact `Authorization` plus every header name configured under `managedServer.http.auth.customHeaders` unless `--verbose-insecure` is enabled.
5. TLS configuration errors MUST fail fast during initialization.
6. Bootstrap wiring MUST warn when `managedServer.http.baseURL` or `managedServer.http.auth.oauth2.tokenURL` use plain HTTP because credentials may be transmitted in cleartext.
7. HTTP response errors MUST preserve status code and response body context.
8. OpenAPI-derived defaults SHOULD improve request correctness but MUST NOT override explicit metadata unless requested.
9. List responses MUST be normalized into deterministic `resource.Resource` ordering.
10. Resolved `operations.list.transforms[*].jqExpression` steps MUST be executed against decoded list payload before list-shape extraction and identity mapping.
11. List-operation `jq` expressions MAY call `resource("<logical-path>")`; resolution MUST use a context-provided logical-path resolver when available.
12. When no logical-path resolver is provided, `resource("<logical-path>")` MUST fail with a validation error.
13. Within one `jq` evaluation, repeated `resource("<logical-path>")` calls MUST be cached by path, and invalid arguments or cyclic resolver dependencies MUST fail with validation errors.
14. When metadata does not explicitly set `Accept`, remote operation requests MUST default to the resolved payload descriptor media mapping (`application/octet-stream` for unknown or octet-stream payloads, `text/plain` for text-like codecs, and the managed-server/OpenAPI/default descriptor when available); body-bearing operations (`create|update`) MUST apply the same default for `ContentType` when unset.
15. Before body transforms or remote HTTP execution, body-bearing mutation workflows MUST validate `resource.requiredAttributes` against the structured source payload; every JSON Pointer referenced by configured `resource.alias` MUST count as required even when `resource.requiredAttributes` omits it; `update` and other non-create mutation flows MUST also require every JSON Pointer referenced by configured `resource.id`; `create` MAY omit metadata-derived `resource.id` pointers unless they are explicitly declared by `resource.requiredAttributes` or operation validation; non-structured mutation payloads MAY skip this attribute-presence check because JSON Pointer traversal is unavailable.
16. Before sending body-bearing requests, operation validation directives (`validate.requiredAttributes`, `validate.assertions`, `validate.schemaRef`) MUST be evaluated against the outgoing payload only for structured payloads and MUST fail fast with `ValidationError` for `octet-stream`; `validate.requiredAttributes[*]` MUST use RFC 6901 JSON Pointer strings.
17. Body-bearing operation payload mutation steps MUST run only for structured payloads; raw text or octet-stream request bodies MUST bypass structured `transforms` processing and preserve the original payload bytes/text.
18. Payload validation context MUST include path-derived template fields (for example `/realm` from `/admin/realms/<realm>/...`) without mutating the outgoing request body.
19. OpenAPI document URLs MAY be cross-origin relative to `managedServer.http.baseURL`, but authentication headers MUST only be attached for same-origin OpenAPI fetches.
20. Managed-server OpenAPI sources MUST accept OpenAPI 3.x (`openapi`) and Swagger 2.0 (`swagger`) documents; Swagger 2.0 operations MUST be normalized for media default inference and `validate.schemaRef=openapi:request-body` compatibility.
21. When `managedServer.http.requestThrottling` is configured, request execution MUST enforce bounded in-flight concurrency and queue capacity, MUST reject overflow with typed conflict errors, and SHOULD share throttling scope for identical managed-server identities.
22. `application/octet-stream` responses MUST decode to `resource.BinaryValue`, and auto/text CLI output for one binary payload MUST write raw bytes without a trailing newline.
23. When a single-resource `get` or `delete` operation path cannot be rendered from the requested logical path alone because metadata templates require payload fields that are only available from a collection item (for example complex aliases such as `{{/name}} - {{/version}}`), managed-server resolution MUST attempt one parent-collection list, find a unique metadata alias/id match, and rerender the resource operation using that matched candidate payload and identity before returning the original validation error.

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
10. Metadata-rendered update operations can target raw `resource.key` payloads, preserve the raw request body, and still resolve `Content-Type` from the active payload descriptor.
11. Structured mutation validation can require `/clientId` from `resource.alias: "{{/clientId}}"` even when an operation transform removes `/clientId` from the outgoing request body afterward.
12. Custom auth header names other than `Authorization` still require debug-log redaction when they come from `managedServer.http.auth.customHeaders`.
13. A direct resource `get` or `delete` path that cannot render `operations.get.path` or `operations.delete.path` from the requested logical segment alone can still succeed when the parent collection list yields exactly one candidate whose rendered alias matches that segment and whose payload supplies the missing template fields.
14. Plain HTTP OAuth2 token URLs emit a bootstrap warning but remain allowed for explicitly insecure local-development contexts.
15. Structured create payloads can omit metadata-derived `/id` when the remote server assigns the resource ID, while update for the same scope still fails fast if `/id` is missing.

## Examples
1. `Get` operation uses `operations.get.path` plus payload-type-aware default `Accept`.
2. `Update` operation resolves `ContentType` from metadata or payload-type defaults and sends structured or opaque payload body accordingly.
3. `List` operation hydrates `resource.Resource` for each item with inferred alias and remote ID.
4. `List` operation `jq` can filter by parent references (for example `.parentId == (resource("/admin/realms/platform/user-registry/ldap-test") | .id)`).
5. `Create` operation validation can require `/realm` while resolving `/realm` implicitly from `/admin/realms/<realm>/...` logical paths.
6. Swagger 2.0 `consumes/produces` plus body `parameters` support `Create` fallback defaults (`ContentType`/`Accept`) and `openapi:request-body` validation without requiring OpenAPI 3 syntax.
7. With `requestThrottling.max-concurrent-requests=1` and `queue-size=1`, a third concurrent request fails fast with `ConflictError` while two earlier requests are in-flight/queued.
8. `Get` of a binary certificate endpoint returns `resource.BinaryValue` when the server responds `application/octet-stream`.
9. `Update` of a Rundeck private key stored as `resource.key` sends raw bytes and resolves `ContentType` to `application/octet-stream` even when the local payload is not a JSON object.
10. `Create` with `resource.requiredAttributes: ["/realm"]` and `resource.alias: "{{/clientId}}"` fails before remote HTTP execution when the structured source payload omits `/clientId`, even if `operations.create.transforms` would otherwise remove `/clientId` from the transmitted body.
11. `Get` or `Delete` for logical path `/apis/orders - v1` with `resource.alias: "{{/name}} - {{/version}}"`, `resource.id: "/id"`, `operations.list.path: /api/apis`, and `operations.get.path: /api/apis/{{/name}}/{{/version}}` first lists `/api/apis`, matches alias `orders - v1`, and then rerenders the resource operation with the matched payload so `.name` and `.version` resolve correctly.
12. Bootstrap with `managedServer.http.auth.oauth2.tokenURL: http://auth.local/oauth/token` emits a plaintext-transport warning before remote requests start.
13. `Create` with `resource.id: "{{/id}}"`, `resource.alias: "{{/clientId}}"`, and `resource.requiredAttributes: ["/realm"]` can succeed with payload `{realm, clientId}` when the server assigns the ID, but `Update` for that same metadata still fails fast until `/id` is supplied or explicitly validated another way.
