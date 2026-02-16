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
1. `reconciler.ResourceReconciler` loads resource and resolved metadata.
2. `secrets.SecretProvider` resolves placeholders.
3. `server.ResourceServerManager` executes update.
4. `repository.ResourceRepositoryManager` persists normalized masked state.

Expected outputs:
1. Remote update succeeds.
2. Local file remains masked.
3. Immediate diff reports no drift.

### Example 2: 404 Direct Path With Alias Fallback
Goal: fetch remote resource when direct path is stale.

Inputs:
1. Path `/customers/acme`.
2. Resolved `get.path` targets stale remote identifier.

Execution:
1. Direct get returns 404.
2. Reconciler performs bounded alias/list fallback.
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
2. CLI detects mismatch and stops before reconciler call.

Expected outputs:
1. Command fails with `ValidationError`.

### Example 6: Repo Status Contract
Goal: ensure `repo status` is non-mutating and output-stable.

Inputs:
1. `declarest repo status` with `--output auto|json|yaml`.

Execution:
1. CLI calls `reconciler.ResourceReconciler.RepoStatus`.
2. Reconciler calls `repository.ResourceRepositoryManager.SyncStatus`.

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
3. Runner prints follow-up commands and waits for user exit.

Expected outputs:
1. Temporary context config is usable.
2. Remote selections fail validation with actionable guidance.

### Example 8: Resource-Server Fixture Identity and Metadata Expansion
Goal: validate fixture-tree sync against API-facing identifiers and nested metadata placeholders.

Inputs:
1. Selected resource-server fixture tree under `repo-template/`.
2. Metadata using `idFromAttribute`/`aliasFromAttribute` and intermediary placeholder paths (for example `/x/_/y/_/_`).

Execution:
1. Loader expands metadata placeholders into concrete collection targets.
2. Reconciler operations resolve remote paths using API-facing identifiers.

Expected outputs:
1. Expanded targets are deterministic and contain no unresolved intermediary placeholders.
2. Apply/diff cycles remain idempotent.

Failure expectation:
1. Misconfigured route identifier mapping fails with typed validation/transport error and actionable path context.
