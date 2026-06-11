# Managed Service, HTTP, and OpenAPI

## Purpose
Define remote server interaction contracts: request construction, auth, OpenAPI/Swagger normalization, throttling, media defaults, request-time enforcement of metadata directives, and the list-jq resolver.

## Scope
Identity-template (`resource.id`/`resource.alias`) and required-attribute semantics are DEFINED in domain.md and metadata.md; this file ENFORCES them at request time. Secret placeholder resolution is defined in secrets.md. Local repository layout is owned by resource-repo.md.

## Normative Rules
1. Remote operations MUST execute only through `managedservice.ManagedServiceClient`.
2. Request method, path, query, and headers MUST derive from resolved metadata plus explicit overrides.
3. Auth mode precedence MUST be deterministic and documented. Auth modes: OAuth2, custom-headers, basic.
4. TLS configuration errors MUST fail fast during initialization.
5. HTTP response errors MUST preserve status code and response body context.
6. List responses MUST normalize into deterministic `resource.Resource` ordering.

### Debug redaction and transport warnings
7. Managed-service debug output MUST redact `Authorization` and every header name configured under `managedService.http.auth.customHeaders`, unless `--verbose-insecure` is enabled.
8. Bootstrap wiring MUST warn (but still allow) when `managedService.http.url` or `managedService.http.auth.oauth2.tokenURL` use plain HTTP, because credentials may transit in cleartext.

### Media defaults
9. When metadata does not set `Accept`, requests MUST default it from the effective concrete payload descriptor or one unique OpenAPI media type when available; concrete metadata `resource.format` MAY supply that descriptor when no explicit payload or repository descriptor exists; `resource.format: any` MUST NOT force one fixed media type.
10. Body-bearing operations (`create|update`) MUST apply rule 9 to `ContentType` when unset.
11. Raw request execution MUST use metadata-rendered `Accept`/`Content-Type` rather than JSON-only defaults.

### OpenAPI / Swagger
12. OpenAPI-derived defaults SHOULD improve request correctness and SHOULD infer metadata `resource.format` from advertised payload formats (using `any` when multiple deterministic formats are advertised; an `application/octet-stream` media type or a schema `format: binary` keyword infers `octet-stream`), but MUST NOT override explicit metadata unless requested.
13. Managed-service OpenAPI sources MUST accept OpenAPI 3.x (`openapi`) and Swagger 2.0 (`swagger`) documents. Swagger 2.0 operations MUST be normalized so media-default inference and `validate.schemaRef=openapi:request-body` behave identically: `parameters[in=body].schema`/`consumes`/`produces`/response `schema` MUST expose equivalent `requestBody.content` and response `content` semantics.
14. OpenAPI document URLs MAY be cross-origin relative to `managedService.http.url`, but authentication headers MUST attach only for same-origin OpenAPI fetches.

### Request-time validation (enforcement only)
15. Before body transforms or remote execution, structured body-bearing mutation workflows MUST validate the effective required-attribute set against the structured source payload (set composition is defined in metadata.md/domain.md: includes `resource.alias` pointers always, `resource.id` pointers for non-create mutations, create MAY omit metadata-derived `resource.id`). Non-structured mutation payloads MAY skip this presence check because JSON Pointer traversal is unavailable.
16. Before sending body-bearing requests, operation validation directives (`validate.requiredAttributes` as RFC 6901 pointers, `validate.assertions`, `validate.schemaRef`) MUST be evaluated against the outgoing payload for structured payloads only, and MUST fail fast with `ValidationError` for `octet-stream`.
17. Payload validation context MUST include path-derived template fields (e.g. `/realm` from `/admin/realms/<realm>/...`) without mutating the outgoing request body.

### Payload handling
18. Body-bearing payload mutation (`transforms`) MUST run only for structured payloads; raw text or `octet-stream` request bodies MUST bypass structured transforms and preserve original bytes/text.
19. `application/octet-stream` responses MUST decode to `resource.BinaryValue`; auto/text CLI output for one binary payload MUST write raw bytes without a trailing newline.

### List-jq resolver
20. Resolved `operations.list.transforms[*].jqExpression` steps MUST execute against the decoded list payload before list-shape extraction and identity mapping.
21. List-operation `jq` MAY call `resource("<logical-path>")`; resolution MUST use the context-provided logical-path resolver when available, and MUST fail with `ValidationError` when no resolver is provided.
22. Within one `jq` evaluation, repeated `resource("<logical-path>")` calls MUST be cached by path; invalid arguments or cyclic resolver dependencies MUST fail with `ValidationError`.

### Throttling
23. When `managedService.http.requestThrottling` is configured (`max-concurrent-requests`, `queue-size`, `requests-per-second`, `burst`), execution MUST enforce bounded in-flight concurrency and queue capacity, MUST reject overflow with `ConflictError`, and SHOULD share throttling scope across identical managed-service identities.

### Collection-fallback rendering
24. When a single-resource `get`/`delete` path cannot be rendered from the requested logical segment alone (e.g. a complex alias such as `{{/name}} - {{/version}}` requires payload fields only available from a collection item), resolution MUST attempt one parent-collection list, find a unique metadata alias/id match, and rerender the operation from that matched candidate payload/identity before returning the original validation error. A non-unique alias/id match MUST fail with `ConflictError`.

## Data Contracts
Request spec adds beyond interfaces.md: `Method`, `Path`, `Query` map, `Headers` map, `Accept`, `ContentType`, `Body` payload, optional `Validate` directives. Server operations: `Get/Create/Update/Delete/List/Exists`, `Request`, `GetOpenAPISpec`.

## Failure Modes
1. Missing/invalid credentials -> auth failure.
2. Timeout or transport interruption.
3. OpenAPI/Swagger document unavailable or invalid; request-body/schema pointer unresolvable -> `ValidationError`.
4. Metadata path template renders an invalid URI.
5. Queue full -> `ConflictError`.
6. Server returns non-JSON payload for a JSON-configured operation; OpenAPI path exists but method unsupported for the operation type.

## Examples
1. `List` hydrates `resource.Resource` per item with inferred alias and remote ID; its `jq` can filter by parent reference, e.g. `.parentId == (resource("/admin/realms/platform/user-registry/ldap-test") | .id)`.
2. Throttling `max-concurrent-requests=1`, `queue-size=1`: a third concurrent request fails fast with `ConflictError` while two earlier requests are in-flight/queued; two concurrent syncs to the same service share one throttle scope/queue budget.
3. Binary `get` of a certificate endpoint returns `resource.BinaryValue` when the server responds `application/octet-stream`; inferred `resource.format` becomes `octet-stream`.
4. `Update` of a Rundeck private key stored as `resource.key` sends raw bytes, preserves the body, and resolves `ContentType` to `application/octet-stream` even though the local payload is not a JSON object.
5. Swagger 2.0 `consumes`/`produces` plus body `parameters` provide `Create` `ContentType`/`Accept` fallbacks and `openapi:request-body` validation without OpenAPI 3 syntax; an operation omitting `parameters[in=body].schema` makes `validate.schemaRef=openapi:request-body` fail with `ValidationError`.
6. `Create` with `resource.requiredAttributes: ["/realm"]` (satisfied implicitly from `/admin/realms/<realm>/...`) and `resource.alias: "{{/clientId}}"` fails before remote execution when the structured source omits `/clientId`, even if `operations.create.transforms` would later remove `/clientId` from the body.
7. Server-assigned ID: `resource.id: "{{/id}}"`, `resource.alias: "{{/clientId}}"`, `resource.requiredAttributes: ["/realm"]` lets `Create` succeed with `{realm, clientId}`, but `Update` for that scope fails fast until `/id` is supplied or validated another way.
8. Collection fallback: for logical path `/apis/orders - v1` with `resource.alias: "{{/name}} - {{/version}}"`, `resource.id: "/id"`, `operations.list.path: /api/apis`, `operations.get.path: /api/apis/{{/name}}/{{/version}}`, resolution lists `/api/apis`, matches alias `orders - v1`, and rerenders the operation from the matched payload so `.name`/`.version` resolve.
9. Bootstrap with `oauth2.tokenURL: http://auth.local/oauth/token` emits a plaintext-transport warning before requests start; custom auth header names other than `Authorization` are still debug-redacted when sourced from `customHeaders`.
