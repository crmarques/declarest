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

## Examples

### Example 1: Apply With Metadata and Secrets
Goal: apply one local resource with masked credentials.

Inputs:
1. Path `/customers/acme`.
2. Local payload includes `apiToken` placeholder.
3. Metadata defines `update.path` and compare suppression.

Execution:
1. `Reconciler` loads resource and resolved metadata.
2. `SecretManager` resolves placeholders.
3. `ResourceServerManager` executes update request.
4. `ResourceRepositoryManager` saves normalized payload with masked placeholders.

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
2. `Reconciler` runs bounded fallback via list operation.
3. Candidate matched by alias resolves correct remote ID.
4. `ResourceInfo` updated with stable alias and remote path.

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
