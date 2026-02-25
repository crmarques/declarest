# Use Cases and Corner Cases

## Purpose
Provide implementation-ready scenarios that make expected behavior and failure handling unambiguous.

## In Scope
1. End-to-end workflow examples.
2. High-risk corner cases.
3. Acceptance expectations by scenario.

## Out of Scope
1. Adapter implementation details.
2. Full CLI help text.
3. Non-essential narrative context.

## Normative Rules
1. New capabilities MUST include at least one normal scenario and one corner-case scenario.
2. Each scenario MUST define source of truth, mutation scope, and expected outputs.
3. Execution steps MUST map to interfaces in `agents/reference/interfaces.md`.
4. Failure paths MUST name expected error category.

## Data Contracts
Scenario template:
1. Name.
2. Goal.
3. Inputs.
4. Preconditions.
5. Execution steps.
6. Expected outputs.
7. Failure expectations.

## Failure Modes
1. Scenario omits mutation scope, hiding side effects.
2. Scenario references behavior not defined in interfaces/contracts.
3. Scenario has non-deterministic expected output.

## Edge Cases
1. Metadata inheritance conflict with alias collision.
2. Secret masking for nested payloads with mixed sensitivity.
3. Refresh after remote deletion with stale local alias.
4. OpenAPI mismatch with explicit metadata override.
5. CLI receives conflicting positional/flag path inputs.

## Examples

### Example 1: Apply With Metadata and Secrets
Goal: apply one local resource that contains masked credentials.

Inputs:
1. Path `/customers/acme`.
2. Local payload with secret placeholders.
3. Metadata defining update path and compare suppression.

Execution:
1. `orchestrator.Orchestrator` loads resource and resolved metadata.
2. `secrets.SecretProvider` resolves placeholders.
3. `server.ResourceServer` executes update.
4. Orchestrator returns normalized remote mutation output without implicit local persistence.

Expected outputs:
1. Remote update succeeds.
2. Local file remains masked.
3. Immediate diff reports no drift.

### Example 2: 404 Direct Path With Alias Fallback
Goal: fetch remote resource when direct path is stale.

Inputs:
1. Path `/customers/acme`.
2. Resolved `operationInfo.getResource.path` targets stale remote identifier.

Execution:
1. Direct get returns 404.
2. Orchestrator performs bounded alias/list fallback.
3. Matching candidate updates `resource.Resource` identity fields.

Expected outputs:
1. Fetch succeeds deterministically on repeated runs.

Failure expectation:
1. Multiple alias candidates return `ConflictError`.

### Example 3: Metadata Wildcard/Literal Precedence
Goal: verify deterministic layered metadata resolution.

Inputs:
1. Wildcard metadata at `/customers/*`.
2. Literal metadata at `/customers/acme`.

Execution:
1. Resolve metadata for `/customers/acme`.
2. Apply defaults, wildcard, literal, then resource directives.

Expected outputs:
1. Literal fields override wildcard fields.
2. Arrays replace inherited arrays.
3. Resolution order is stable.

### Example 4: Repository Traversal Rejection
Goal: prevent filesystem escape on save.

Inputs:
1. Path `/customers/../../etc/passwd`.

Execution:
1. Repository normalizes path and validates safe-join before IO.

Expected outputs:
1. Operation fails with `ValidationError`.
2. No filesystem mutation occurs.

### Example 5: CLI Path Conflict
Goal: reject ambiguous path target selection.

Inputs:
1. `declarest resource get /customers/acme --path /customers/other`.

Execution:
1. CLI parses positional and flag path values.
2. CLI detects mismatch and stops before orchestrator call.

Expected outputs:
1. Command fails with `ValidationError`.

### Example 6: Repo Status Contract
Goal: ensure `repo status` is non-mutating and output-stable.

Inputs:
1. `declarest repo status` with `--output auto|json|yaml`.

Execution:
1. CLI resolves `repository.RepositorySync` from startup context.
2. CLI calls `repository.RepositorySync.SyncStatus`.

Expected outputs:
1. `auto` prints deterministic text summary.
2. Structured modes expose stable keys: `state`, `ahead`, `behind`, `hasUncommitted`.
3. Operation performs no repository mutation.

### Example 7: Manual E2E Handoff
Goal: start local stack and hand control to the user.

Inputs:
1. `run-e2e.sh --profile manual`.

Execution:
1. Runner validates selected stack is local-instantiable.
2. Runner starts components and emits temporary context catalog.
3. Runner executes optional component `manual-info` hooks and prints access details.
4. Runner copies selected resource-server `repo-template` into the context repository directory.
5. Runner generates setup/reset shell scripts and prints follow-up `declarest-e2e` commands, exits, and keeps runtime resources.

Expected outputs:
1. Temporary context config is usable after sourcing the generated setup script.
2. Access output includes component-specific details (for example, local keycloak admin console URL and credentials).
3. Context repository directory contains seeded template resources and collection metadata.
4. Setup script exports runtime env vars and defines alias `declarest-e2e`; reset script unsets these vars and removes the alias.
5. Output includes cleanup commands (`--clean`, `--clean-all`) for explicit teardown.
6. Remote selections fail validation with actionable guidance.

### Example 8: Resource-Server Fixture Identity and Metadata Expansion
Goal: validate fixture-tree sync against API-facing identifiers and nested metadata placeholders.

Inputs:
1. Selected resource-server fixture tree under `repo-template/`.
2. Metadata using `idFromAttribute`/`aliasFromAttribute` and intermediary placeholder paths (for example `/x/_/y/_/_`).

Execution:
1. Loader expands metadata placeholders into concrete collection targets.
2. Orchestrator operations resolve remote paths using API-facing identifiers.

Expected outputs:
1. Expanded targets are deterministic and contain no unresolved intermediary placeholders.
2. Apply/diff cycles remain idempotent.

Failure expectation:
1. Misconfigured route identifier mapping fails with typed validation/transport error and actionable path context.

### Example 9: Resource Get Source Selection
Goal: read either remote observed state or local desired state deterministically.

Inputs:
1. Path `/customers/acme`.
2. CLI source flag `--source` with values `repository` or `remote-server`.

Execution:
1. `declarest resource get /customers/acme` runs without source flags.
2. `declarest resource get /customers/acme --source repository` runs with repository override.
3. `declarest resource get /customers/acme --source repository --remote-server` runs with conflicting compatibility-era inputs.

Expected outputs:
1. Step 1 reads from remote source by default.
2. Step 2 reads from repository local source.

Failure expectation:
1. Step 3 fails with `ValidationError` before orchestrator execution.

### Example 10: Resource Save List Fanout
Goal: persist collection payloads as one file per resource by default.

Inputs:
1. Path `/customers`.
2. Input payload list (array or object containing `items` array).
3. Optional flags `--as-items` and `--as-one-resource`.

Execution:
1. `declarest resource save /customers` receives a list payload.
2. CLI resolves item aliases using metadata identity attributes.
3. CLI saves one resource file per resolved item path.
4. `declarest resource save /customers --as-one-resource` receives the same list payload.

Expected outputs:
1. Step 1 produces one local file per list item under `/customers/<alias>`.
2. Step 4 persists the payload as a single resource file at `/customers`.

Failure expectation:
1. `--as-items` with non-list input fails with `ValidationError`.
2. `--as-items --as-one-resource` fails with `ValidationError`.

### Example 11: E2E Dependency-Aware Parallel Component Hooks
Goal: run component hooks in parallel when possible without violating dependency constraints.

Inputs:
1. Selected stack with `repo-type=git`, `git-provider=gitlab`, `resource-server=simple-api-server`, `secret-provider=file`.
2. Component metadata where `repo-type:git` declares `COMPONENT_DEPENDS_ON="git-provider:*"`.

Execution:
1. `run-e2e.sh` executes `init` hooks using dependency-aware batching.
2. `git-provider:gitlab` and `resource-server:simple-api-server` initialize in parallel.
3. `repo-type:git` initializes only after `git-provider:*` completion.
4. Runner executes `configure-auth` and `context` hooks through the same dependency graph.

Expected outputs:
1. Hook batches run in parallel where no dependency edge exists.
2. Repository component reads provider state without race conditions.
3. Final context assembly succeeds deterministically.

Failure expectation:
1. Dependency selector referencing a non-selected component fails with actionable dependency error.
2. Cyclic dependencies fail fast with an explicit cycle message before workload execution.

### Example 12: Simple API OAuth2 Guardrail (Corner)
Goal: ensure `simple-api-server` denies resource operations without a valid bearer token.

Inputs:
1. Local stack with `resource-server=simple-api-server`.
2. Client credentials configured in component state.

Execution:
1. Call a non-token endpoint (for example `GET /api/projects`) without `Authorization: Bearer`.
2. Call `/token` with valid `grant_type=client_credentials` and configured client credentials.
3. Retry `GET /api/projects` with the issued bearer token.

Expected outputs:
1. Step 1 fails with HTTP `401` and OAuth2 `invalid_token`.
2. Step 2 returns JSON containing `access_token` and `token_type=Bearer`.
3. Step 3 succeeds and returns deterministic JSON (`[]` or list payload, based on stored resources).

Failure expectation:
1. Invalid client credentials at `/token` fail with OAuth2 `invalid_client` and HTTP `401`.

### Example 13: Simple API mTLS Client Certificate Allowlist
Goal: ensure `simple-api-server` accepts only configured client certificates during TLS handshake when mTLS is enabled.

Inputs:
1. `resource-server=simple-api-server`.
2. `ENABLE_MTLS=true`.
3. One or more allowed client public certificates mounted into the configured cert directory.

Execution:
1. Start the component with `DECLAREST_E2E_SIMPLE_API_ENABLE_MTLS=true`.
2. Run one request with the configured client cert/key and CA trust settings.
3. Run one request with an untrusted client certificate.
4. Remove all trusted cert files from the configured allowlist directory while the server is running and retry with the previously trusted client certificate.
5. Add the trusted client cert file back to the allowlist directory and retry again.

Expected outputs:
1. Step 2 succeeds and the request reaches normal API handling.
2. Step 3 fails during TLS handshake before API request processing.
3. Step 4 fails without restarting the service because no client certificates are currently trusted.
4. Step 5 succeeds again without restarting the service.

Failure expectation:
1. Missing TLS server cert/key material causes startup/configuration failure with actionable error.

### Example 14: Simple API Basic Auth Guardrail
Goal: ensure `simple-api-server` rejects unauthenticated requests and accepts configured basic-auth credentials when basic-auth mode is selected.

Inputs:
1. `resource-server=simple-api-server`.
2. `--resource-server-basic-auth=true`.
3. Basic auth username/password configured in component state.

Execution:
1. Call `GET /health` without `Authorization`.
2. Call `GET /health` with wrong basic credentials.
3. Call `GET /health` with configured basic credentials.

Expected outputs:
1. Step 1 fails with HTTP `401`.
2. Step 2 fails with HTTP `401`.
3. Step 3 succeeds with HTTP `200`.

Failure expectation:
1. Selecting both `--resource-server-basic-auth=true` and `--resource-server-oauth2=true` fails run selection before startup.

### Example 15: Secret Detect Metadata Autofix
Goal: detect secret-like attributes from repository resources or input payload and persist them into metadata.

Inputs:
1. Local repository resources under `/customers` containing potential secret-like attributes.
2. Optional payload input with detected keys (for example `apiToken`, `password`).
3. `declarest secret detect` (repository-scan mode, whole repo scope).
4. `declarest secret detect /customers --fix` (repository-scan scoped metadata autofix).
5. `declarest secret detect /customers/acme --fix < payload.json` (payload mode metadata autofix).
6. Optional `--secret-attribute password`.

Execution:
1. Without payload input, CLI scans local repository resources recursively under requested path (or `/` when path omitted).
2. With payload input, CLI detects secret candidates from payload content.
3. When `--secret-attribute` is provided, CLI filters to exactly one detected attribute.
4. In `--fix` mode, CLI loads existing metadata for each target path (or initializes empty metadata when missing).
5. CLI merges filtered detected attributes into `resourceInfo.secretInAttributes` and persists metadata.

Expected outputs:
1. Repository-scan output groups detected attributes by logical resource path with deterministic ordering.
2. Metadata for fixed paths contains deterministic, deduplicated `resourceInfo.secretInAttributes`.
3. Existing metadata directives remain preserved.

Failure expectation:
1. `declarest secret detect --fix < payload.json` without path fails with `ValidationError`.
2. `declarest secret detect /customers --secret-attribute unknown` fails with `ValidationError`.

### Example 16: OpenAPI Context Propagation
Goal: surface a component's `openapi.yaml` (when present) to the generated context so metadata inference uses the stable API definition.

Inputs:
1. Component `keycloak` (or another) declares an `openapi.yaml` asset under its component directory.
2. `run-e2e.sh` is invoked with that component selected and a writable run directory.

Execution:
1. The runner copies each selected component's `openapi.yaml` to the run directory before context hooks execute.
2. The corresponding `context` hook reads the exported `E2E_COMPONENT_OPENAPI_SPEC` value and emits the appropriate key (for resource servers, `resource-server.http.openapi`) pointing at the run-scoped spec file.
3. `e2e_context_build` aggregates fragments, producing a context YAML that references the copied spec.

Expected outputs:
1. `test/e2e/.runs/<run-id>/contexts.yaml` contains `resource-server.http.openapi` pointing to `<run-id>/<component-name>-openapi.yaml`.
2. `declarest config show --context e2e-<profile>` succeeds and can use the spec for metadata inference or `metadata infer --openapi`.

Failure expectation:
1. If the runner cannot copy the declared spec, the context phase fails fast with an actionable error before `metadata` commands run.

### Example 17: Repository History by Backend Type (Corner)
Goal: expose local git history when available while keeping filesystem repos deterministic and non-mutating.

Inputs:
1. Context `dev-fs` with `repository.filesystem`.
2. Context `dev-git` with `repository.git` and a repository base dir that may exist without `.git/` initialized yet.
3. Optional filters `--max-count`, `--author`, `--grep`, `--since`, `--until`, `--path`, `--oneline`.

Execution:
1. Run `declarest --context dev-fs repo history`.
2. Run `declarest --context dev-git repo history --oneline --max-count 5 --author alice --grep fix --path customers`.

Expected outputs:
1. Step 1 prints a stable not-supported message for filesystem repositories and exits successfully.
2. Step 2 auto-initializes the local git repo when needed and prints filtered local git commit history (empty on a fresh repo) without additional unexpected mutations.

Failure expectation:
1. Invalid `--since` or `--until` date input fails with `ValidationError` before repository history lookup.

### Example 18: Git Auto-Commit for Repository Mutations
Goal: commit repository changes after local mutation commands while protecting against unrelated worktree changes.

Inputs:
1. Git repository context with clean worktree.
2. `resource save` or `resource delete --repository`.
3. Optional commit-message flags `--message` or `--message-override`.

Execution:
1. Run `declarest resource save /customers/acme --payload 'id=acme,name=Acme' --overwrite --message ticket-123`.
2. Run `declarest resource delete /customers/acme --confirm-delete --repository --message-override 'cleanup customer'`.
3. Re-run one command after creating an unrelated uncommitted change in the repo.
4. Run one command with both `--message` and `--message-override`.

Expected outputs:
1. Step 1 saves repository content and creates one local commit whose message appends `ticket-123` to the default save message.
2. Step 2 deletes local repository content and creates one local commit using the override message exactly.

Failure expectation:
1. Step 3 fails with `ValidationError` before mutation because auto-commit commands require a clean git worktree.
2. Step 4 fails with `ValidationError` because commit-message flags are mutually exclusive.
