# Testing, Quality, and Security

## Purpose
Define quality gates, security invariants, and acceptance criteria for changes across the system.

## In Scope
1. Test strategy and coverage expectations.
2. Security controls and safety checks.
3. Regression and acceptance criteria.
4. Release readiness checklist.

## Out of Scope
1. CI vendor-specific setup files.
2. Runtime infrastructure monitoring implementation.
3. UI design concerns.

## Normative Rules
1. Every behavioral change MUST include tests at the lowest effective layer.
2. High-risk orchestration changes MUST include integration coverage.
3. CLI contract changes MUST include command-level tests for success and failure paths.
4. Security-sensitive flows MUST include negative tests.
5. Deterministic ordering guarantees MUST be asserted in tests.
6. Path traversal protections MUST be tested for repository and metadata access.
7. Secret values MUST never appear in logs or snapshots.
8. Merge requests SHOULD fail if required test categories are missing.

## Data Contracts
Test layers:
1. Unit: pure transforms, path normalization, metadata merge/render, secret placeholder normalization.
2. Integration: reconciler with fake providers and conflict handling.
3. E2E: CLI workflows with representative contexts and fixtures.
4. E2E profiles:
   `basic`: all `main` cases with matched capabilities.
   `full`: all `main` + `corner` cases with matched capabilities.
   `manual`: environment bring-up and interactive handoff without automated assertions.

Acceptance contracts:
1. Reconciler idempotency for repeated apply.
2. Stable diff ordering.
3. Typed error categories for all failure classes.

## Required Scenario Coverage
1. Metadata precedence with wildcard/literal collisions.
2. Relative template resolution across nested paths.
3. Alias/ID divergence handling in `resource.Resource`.
4. Secret placeholder behavior for valid and invalid scopes.
5. Path traversal rejection in repository operations.
6. Deterministic diff/compare with suppression/filter semantics.
7. CLI validation errors and destructive-operation safeguards.
8. Reconciler idempotency under repeated apply.
9. Context config precedence and invalid override handling.
10. OpenAPI-assisted request construction with safe fallback behavior.
11. Repository sync conflict classes and actionable outcomes.
12. File-organization policy scenarios for split vs cohesive files.
13. CLI package boundary checks rejecting direct provider implementation imports.
14. Repository list/delete recursion policy behavior and deterministic ordering.
15. `repo status` output contract for auto text and structured JSON/YAML modes.
16. Missing context catalog behavior (`list` empty vs `current/resolve` `NotFoundError`) is covered.
17. `config create/update/validate` strict input decoding rejects unknown JSON/YAML fields.
18. Interactive config flows (`create/use/rename/delete`) are covered for TTY success paths and non-TTY validation failures.
19. `config show` is covered for `--context` selection, interactive fallback selection, non-interactive validation failure, and YAML output contract.
20. Context persistence compacts `metadata.base-dir` when equal to repository base-dir, and `ResolveContext` restores the default value.
21. E2E profile selection (`basic|full|manual`) maps to expected workload scope and step flow.
22. Manual profile rejects remote-only selections and emits a temporary context config for manual interaction.
23. E2E runtime output reports grouped progress steps with deterministic status transitions (`RUNNING`, `OK`, `FAIL`, `SKIP`).
24. Case requirement filtering skips unsupported cases unless explicitly mandatory for the selected stack.

## Failure Modes
1. Tests pass locally but rely on non-deterministic ordering.
2. Missing regression tests for changed metadata behavior.
3. Security checks bypassed by direct provider calls.
4. Snapshot tests containing secrets.

## Edge Cases
1. Empty payload comparison after suppression removes all fields.
2. Secret normalization across equivalent placeholders.
3. Non-unique alias conflict during apply.
4. Refresh flow with partially configured server context.

## Examples
1. Unit test verifies metadata merge order across wildcard and literal candidates.
2. Integration test asserts `ConflictError` category when push detects divergence.
3. E2E test validates `resource delete` requires explicit force for collection targets.
