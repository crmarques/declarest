# Testing, Quality, and Security

## Purpose
Define quality gates and security invariants so behavior changes are verifiable and safe.

## In Scope
1. Test strategy by risk level.
2. Security and safety controls.
3. Required regression/acceptance coverage.
4. Release-readiness checks.

## Out of Scope
1. CI vendor configuration.
2. Runtime observability platform setup.
3. UI style concerns.

## Normative Rules
1. Every behavior change MUST add tests at the lowest effective layer.
2. High-risk orchestration or integration changes MUST include integration coverage.
3. CLI contract changes MUST include command-level success and failure tests.
4. Security-sensitive flows MUST include negative tests.
5. Deterministic ordering guarantees MUST be asserted.
6. Path traversal protections MUST be tested for repository and metadata access.
7. Secret values MUST never appear in logs, errors, or snapshots.
8. New normative rules SHOULD include an explicit matching test expectation.

## Data Contracts
Test layers:
1. Unit: pure transforms, normalization, metadata layering/template rendering, secret placeholder normalization.
2. Integration: reconciler workflows with fake providers and conflict handling.
3. E2E: CLI workflows using representative stacks and fixture trees.

Acceptance contracts:
1. Reconciler idempotency for repeated apply.
2. Stable diff ordering for equivalent inputs.
3. Typed error categories for all major failure classes.

## Required Scenario Coverage
1. Metadata precedence: wildcard/literal collisions and template resolution (including relative references).
2. Repository safety: traversal rejection and deterministic list/delete recursion behavior.
3. Identity handling: alias/ID divergence and ambiguity conflict detection.
4. Secrets lifecycle: detect/mask/resolve/normalize behavior and non-disclosure guarantees.
5. Compare/diff semantics: suppression/filter rules and stable output.
6. CLI safeguards: validation errors, conflicting path inputs, and destructive-operation protections.
7. Context config: strict decode, one-of validation, overrides precedence, and missing-catalog behavior.
8. Remote operation construction: OpenAPI-assisted defaults with explicit metadata override precedence.
9. Repository sync: conflict classes, actionable outcomes, and `repo status` output contract.
10. E2E profiles: `basic|full|manual` workload behavior, requirement filtering, and deterministic step statuses.
11. E2E runtime UX: grouped step reporting (`RUNNING|OK|FAIL|SKIP`) and actionable failure log pointers.
12. Resource-server fixtures: metadata identity mapping (`idFromAttribute`/`aliasFromAttribute`) and intermediary `/_/` expansion for nested trees.
13. E2E component orchestration: dependency-aware hook ordering, parallel ready-batch execution, and cycle/missing-dependency failures.
14. OAuth2 component auth: `client_credentials` token issuance and bearer-token rejection when auth is missing or invalid.
15. mTLS component auth: only configured client certificates are accepted when mTLS is enabled.
16. Basic-auth component auth: requests fail without valid credentials when basic auth is selected and succeed with configured username/password.
17. Ad-hoc CLI routing: `ad-hoc <method>` maps to managed-server requests with positional/flag path validation and payload decoding from `--file` or stdin, and `ad-hoc post|put --payload` inline decoding with source-conflict validation.
18. mTLS trust reload: updating `simple-api-server` trusted client-cert files at runtime changes access behavior for new connections without service restart, including empty trusted-cert sets denying all access.
19. Resource save secret safeguard: `resource save` fails on potential plaintext secrets unless `--ignore` is provided, and `resource save --handle-secrets[=<comma-separated-attributes>]` handles selected candidates, stores deterministic path-scoped secret keys, writes `{{secret .}}` placeholders, updates metadata, and fails with warning when unhandled candidates remain unless `--ignore` is set.
20. Secret detect metadata fix flow: `secret detect` scans repository scope when no payload input is provided (default scope `/`), `secret detect --fix` merges detected attributes into metadata `secretsFromAttributes`, and `--secret-attribute` filtering has negative validation coverage for payload and repository-scan modes.
21. Resource mutation scope: `resource apply|create|update|delete` execute direct-child collection targets by default and include descendants with `--recursive`; `resource create` accepts explicit payload input for single-target mutations and otherwise loads repository payloads for resolved targets.
22. Resource save list identity fallback: when list entries omit metadata-defined alias/id attributes, `resource save` falls back to common identity attributes (`clientId`, `id`, `name`, `alias`) before returning alias-resolution errors.
23. Metadata-aware read fallback: `resource get` and `resource save` (no input) attempt literal remote read first and then collection list/filter fallback by metadata alias/id when `NotFound`; `ad-hoc get` mirrors this behavior after a `NotFound` literal request.
24. Metadata-aware path fallback breadth: repository-backed single-resource workflows (`resource get --repository`, `resource apply`, `resource update`, `resource diff`, `resource explain`) use literal local lookup then bounded metadata `idFromAttribute` fallback; remote delete retries with metadata-aware identity after literal `NotFound`.
25. Resource secret placeholder resolution: remote workflows resolve `{{secret .}}` to logical-path scoped keys, resolve `{{secret <custom-key>}}` overrides, and remain compatible with legacy absolute key placeholders.
26. Metadata persistence normalization: metadata writes omit nil fields (no `null` serialization) while preserving explicit empty arrays/maps used for merge replacement behavior.
27. Resource save override guard: saving to an already-present logical path must fail without `--force` and succeed when `--force` is provided so repository state cannot be overwritten accidentally.
28. Resource save wildcard expansion: `resource save` must expand `_` path segments via remote direct-child list traversal, persist all matched collection/resource targets, reject wildcard+payload input, and return `NotFound` when no concrete matches are resolved.

## Failure Modes
1. Tests pass locally with hidden non-determinism.
2. Changed behavior lacks regression coverage.
3. Security-sensitive paths bypass required safeguards.
4. Snapshot/log artifacts leak secret values.

## Edge Cases
1. Suppression removes all comparable fields.
2. Equivalent secret placeholders normalize differently.
3. Non-unique alias appears during apply/refresh.
4. Remote workflow runs with partially configured context.

## Examples
1. Unit test verifies deterministic metadata merge order across wildcard and literal layers.
2. Integration test asserts `ConflictError` when repository sync detects divergence.
3. E2E test validates collection delete safety gates and deterministic summary output.
