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
1. Remote operations MUST be executed through `ResourceServerManager` only.
2. Request method, path, query, and headers MUST derive from resolved metadata plus explicit overrides.
3. Auth mode precedence MUST be deterministic and documented.
4. TLS configuration errors MUST fail fast during initialization.
5. HTTP response errors MUST preserve status code and response body context.
6. OpenAPI-derived defaults SHOULD improve request correctness but MUST NOT override explicit metadata unless requested.
7. List responses MUST be normalized into deterministic `ResourceInfo` ordering.

## Data Contracts
Request spec fields:
1. `Method`.
2. `Path`.
3. `Query` map.
4. `Headers` map.
5. `Accept`.
6. `ContentType`.
7. `Body` resource payload.

Server manager operations:
1. `Get/Create/Update/Delete/List/Exists`.
2. `GetOpenAPISpec`.
3. `BuildRequestFromMetadata`.

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

## Examples
1. `Get` operation uses `operations.get.path` plus default `Accept: application/json`.
2. `Update` operation resolves `ContentType` from metadata and sends normalized payload body.
3. `List` operation hydrates `ResourceInfo` for each item with inferred alias and remote ID.
