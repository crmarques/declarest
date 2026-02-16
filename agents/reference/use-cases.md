# Use Cases and Corner Cases

## Purpose
Provide implementation-oriented scenarios that make expected behavior and edge cases unambiguous.

## In Scope
1. End-to-end workflow examples.
2. Corner-case walkthroughs.
3. Acceptance expectations per scenario.

## Out of Scope
1. Adapter implementation code.
2. Full CLI help text.
3. Non-essential narrative context.

## Normative Rules
1. Every new capability MUST add at least one normal and one corner-case scenario.
2. Each scenario MUST identify source of truth, mutation scope, and expected outputs.
3. Scenario steps MUST map to interfaces in `interfaces.md`.
4. Scenario outcomes MUST reference expected error category for failure paths.

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
1. Scenario omits mutation scope and hides side effects.
2. Scenario references undefined interface behavior.
3. Scenario output expectations are non-deterministic.

## Edge Cases
1. Metadata inheritance conflict plus alias collision.
2. Secret masking on nested map with mixed key sensitivity.
3. Refresh after remote deletion and local stale alias.
4. OpenAPI mismatch with explicit metadata override.
5. CLI path provided both positionally and via `--path` with different values.

## Examples

### Example 1: Apply With Metadata and Secrets
Goal: apply one local resource with masked credentials.

Inputs:
1. Path `/customers/acme`.
2. Local payload includes `apiToken` placeholder.
3. Metadata defines `update.path` and compare suppression.

Execution:
1. `reconciler.ResourceReconciler` loads resource and resolved metadata.
2. `secrets.SecretProvider` resolves placeholders.
3. `server.ResourceServerManager` executes update request.
4. `repository.ResourceRepositoryManager` saves normalized payload with masked placeholders.

Expected outputs:
1. Remote update succeeds.
2. Local file remains masked.
3. Diff after apply reports no drift.

### Example 2: 404 Direct Path With Alias Fallback
Goal: fetch remote resource when direct path fails.

Inputs:
1. Path `/customers/acme`.
2. `operations.get.path` initially resolves to stale ID.

Execution:
1. Direct `get` returns 404.
2. `reconciler.ResourceReconciler` runs bounded fallback via list operation.
3. Candidate matched by alias resolves correct remote ID.
4. `resource.Resource` updated with stable alias and remote path.

Expected outputs:
1. Fetch succeeds without manual intervention.
2. Operation remains deterministic for repeated calls.

Failure expectation:
1. If multiple candidates share alias, return `ConflictError`.

### Example 3: Metadata Wildcard Precedence
Goal: confirm deterministic merge of wildcard and literal directives.

Inputs:
1. Wildcard metadata at `/customers/*`.
2. Literal metadata at `/customers/acme`.

Execution:
1. Resolve layered metadata for `/customers/acme`.
2. Apply defaults, wildcard, literal, then resource directives.

Expected outputs:
1. Literal overrides wildcard where fields collide.
2. Arrays replace parent arrays.
3. Merge order remains stable across runs.

### Example 4: Traversal Rejection
Goal: prevent filesystem escape during save.

Inputs:
1. Path `/customers/../../etc/passwd`.

Execution:
1. Repository path normalization and safe-join validation run before IO.

Expected outputs:
1. Operation fails with `ValidationError`.
2. No filesystem write occurs.

### Example 5: CLI Path Input Conflict
Goal: avoid ambiguous target selection when both path inputs are provided.

Inputs:
1. Command `declarest resource get /customers/acme --path /customers/other`.

Execution:
1. CLI parses both path inputs.
2. CLI detects mismatch between positional and flag path.

Expected outputs:
1. Command fails before execution with `ValidationError`.
2. No reconciler call occurs.

### Example 6: Collection List/Delete Recursion Policy
Goal: enforce non-recursive defaults and explicit recursive behavior.

Inputs:
1. Collection path `/customers`.
2. Resources in `/customers/acme`, `/customers/east/zen`, `/customers/west/zen`.
3. List policy and delete policy toggled between `Recursive=false` and `Recursive=true`.

Execution:
1. `repository.ResourceRepositoryManager.List` is called with `Recursive=false`.
2. `repository.ResourceRepositoryManager.Delete` is called with `Recursive=false`.
3. Repeat list/delete with `Recursive=true`.

Expected outputs:
1. Non-recursive list returns only direct collection children in deterministic order.
2. Non-recursive delete removes only direct resources and preserves nested collections.
3. Recursive list and delete include full descendants deterministically.

### Example 7: Repo Status Output Contract
Goal: ensure repository sync status is non-mutating and output-stable.

Inputs:
1. Command `declarest repo status`.
2. Output mode `auto`, then `json`, then `yaml`.

Execution:
1. CLI calls `reconciler.ResourceReconciler.RepoStatus`.
2. Reconciler calls `repository.ResourceRepositoryManager.SyncStatus`.
3. CLI formats text for `auto` and structured output for `json`/`yaml`.

Expected outputs:
1. `auto` prints deterministic text summary.
2. `json` and `yaml` include stable `state`, `ahead`, `behind`, `hasUncommitted`.
3. Operation does not mutate repository state.

### Example 8: Profile-Scoped E2E Execution
Goal: ensure profile selection controls case scope without overriding component stack flags.

Inputs:
1. Command `run-e2e.sh --profile basic --repo-type filesystem`.
2. Command `run-e2e.sh --profile full --repo-type filesystem`.

Execution:
1. `basic` resolves and runs only `main` cases whose requirements match the selected stack.
2. `full` resolves and runs `main` plus `corner` cases whose requirements match the selected stack.
3. Runner reports grouped steps with deterministic status transitions.

Expected outputs:
1. `basic` excludes `corner` cases.
2. `full` includes `corner` cases.
3. Both commands keep explicit component selections unchanged.

### Example 9: Manual Profile Interactive Handoff
Goal: start components and expose a temporary context catalog for user-driven verification.

Inputs:
1. Command `run-e2e.sh --profile manual`.
2. Optional explicit local component flags.

Execution:
1. Runner initializes selected local-instantiable components.
2. Runner generates temporary `contexts.yaml` under run artifacts.
3. Runner prints export and sample CLI commands, then waits for user exit.

Expected outputs:
1. Temporary context config exists and is usable by CLI commands.
2. Remote selections are rejected for manual profile with actionable validation output.
3. Teardown runs only after manual session exits (unless keep-runtime is enabled).
