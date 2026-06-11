# Testing, Quality, and Security

## Purpose
Define the test strategy and the cross-cutting verification and security gates that make behavior changes verifiable and safe.

## Normative Rules

### Test Layers
1. Unit tests MUST cover pure transforms, normalization, metadata layering/template rendering, and secret placeholder normalization.
2. Integration tests MUST cover orchestrator workflows with fake providers, including conflict handling.
3. E2E tests MUST cover CLI/operator workflows against representative stacks and fixture trees.

### Coverage Tracing
4. Every normative rule in a domain reference file MUST have a matching test at the lowest effective layer; tests trace to the owning file's rules, not to this file. Owning files are defined in `AGENTS.md`.
5. Each behavior change MUST add or update tests in the same change. High-risk orchestration or integration changes MUST add integration coverage; CLI contract changes MUST add command-level success and failure tests.

### Risk-Based Verification Scope
6. Doc-only changes MUST verify markdown/reference consistency; no code tests required.
7. Low-risk changes (isolated pure logic) MUST run the affected unit tests.
8. Medium-risk changes (cross-component behavior, CLI surface) MUST run unit plus integration coverage for the touched contract.
9. High-risk changes (orchestration, security, operator, persisted contracts, release packaging) MUST run unit, integration, and the relevant E2E or packaging gate before handoff.

### Security Invariants
10. Secret values MUST NOT appear in logs, errors, or snapshots; secret non-disclosure MUST have negative-test coverage. Secret semantics are owned by `secrets.md`.
11. Path-traversal protections for repository and metadata access MUST have negative tests (rejection asserts). Path safety is owned by `resource-repo.md`.
12. Auth-required and destructive-operation flows MUST have negative tests (rejection without valid credentials/confirmation). Auth modes are owned by `managed-service.md`; CLI safeguards by `cli.md`.
13. Deterministic ordering guarantees (diff, list, completion, summaries) MUST be asserted with stable-output tests.

### Cross-Cutting Gates
14. Changes to persisted metadata, context, or bundle contracts MUST update the corresponding `schemas/*.json` artifacts in the same change, and a low-level test MUST assert those schema files remain valid JSON and keep their expected top-level wiring.
15. OLM packaging changes (`config/manifests/`, `config/olm/`, `bundle/`, `catalog/`, `bundle.Dockerfile`, `catalog.Dockerfile`, and release workflows publishing bundle/catalog images) MUST pass `make verify-bundle` before handoff, and CI MUST run `make verify-bundle` path-filtered to those OLM-relevant changes. OLM packaging boundaries are owned by `k8s-operator.md` and `architecture.md`.
16. Workflows that publish release artifacts or container images MUST pin external actions to full commit SHAs, grant the least `GITHUB_TOKEN` permissions per job, validate the release config (`.goreleaser.yaml`), and publish provenance plus SBOM attestations for released CLI artifacts and pushed container images.
17. Kubernetes operator behavior changes MUST add controller coverage (CRD validation/status transitions) plus webhook coverage (authentication, signature/event filtering). Operator contracts are owned by `k8s-operator.md`.

## Failure Modes
1. Tests pass locally while hiding non-determinism (ordering, timing).
2. Changed behavior ships without regression coverage at any layer.
3. Security-sensitive paths bypass required negative tests.
4. Snapshot or log artifacts leak secret values.
5. Persisted-contract change merges without the matching `schemas/*.json` update.

## Examples
1. Unit test asserts deterministic metadata merge order across wildcard and literal layers.
2. Integration test asserts `ConflictError` when repository sync detects divergence.
3. Negative test asserts repository read rejects a path-traversal segment and produces no leaked secret in the error.
4. E2E test asserts collection-delete safety gates fire and the run summary output is deterministic.
5. Low-level test asserts `schemas/context.schema.json`, `schemas/contexts.schema.json`, and `schemas/metadata.schema.json` stay valid JSON with their expected top-level wiring.
6. Packaging test asserts `make verify-bundle` succeeds and a release workflow test asserts full-SHA action pins plus provenance/SBOM attestation markers.
