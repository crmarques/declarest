---
name: quality-gate
description: Select and execute the smallest verification set that protects changed contracts while keeping feedback fast.
---

# Quality Gate

## Workflow
1. Classify change impact as `doc`, `low`, `medium`, or `high`.
2. Map impact to verification scope using the matrix below.
3. Run the fastest meaningful checks first and stop on first failure.
4. Expand scope only when changed contracts, orchestration risk, or security risk requires it.
5. Report commands run, outcomes, and any intentional coverage gaps.

## Impact Matrix
1. `doc`: spec/comment/instruction-only edits with no behavior change; tests are optional unless contracts changed.
2. `low`: pure transforms in one package, no I/O/auth/path-safety changes; run targeted package tests.
3. `medium`: CLI wiring, metadata behavior, repository semantics, or provider contract changes; run targeted tests plus repository-wide tests.
4. `high`: orchestration, auth/secrets, path safety, destructive operations, or E2E harness changes; run repository-wide tests and relevant E2E coverage.

## Command Guidance
1. Prefer package-scoped checks first: `go test ./<package>/...`.
2. Use repository-wide regression gate when needed: `go test ./...`.
3. Use `make check` when formatting/lint/tests all need reconfirmation.
4. Use focused E2E runs before full profiles: `./test/e2e/run-e2e.sh --profile basic ...` (or `make e2e E2E_FLAGS='...'`).
5. Avoid redundant reruns when unchanged areas are already validated.

## Guardrails
1. Do not claim coverage for checks that were not executed.
2. If a required check cannot run, report the blocker and residual risk.
3. Security-sensitive and destructive workflows require negative-test evidence.
4. Keep verification proportional; avoid full E2E suites for low-impact edits.
