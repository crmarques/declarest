# Testing Review

Re-runnable review of DeclaREST's test suite (unit / integration / E2E) plus the Bash E2E harness, then design/implement tests that enforce the specs. Tests must enforce the specs, not mirror current buggy behavior. You may refactor tests and harness and make minimal production changes needed for testability.

The authoritative specs are `agents/reference/*`. `quality.md` defines the test strategy (layers, risk-based scope, security invariants); every normative rule in a domain file should trace to a test at the lowest effective layer.

## Method
1. **Spec → test traceability** — list key behaviors/invariants from the reference specs; map each to the test(s) that verify it; surface gaps as a table.
2. **Suite assessment** — enumerate tests by type; evaluate assertion strength (strong vs weak), determinism/flakiness, setup/teardown hygiene, runtime, parallelization, and CI reproducibility.
3. **E2E harness** (`test/e2e/`, Bash, pluggable components called by a generic orchestrator) — review lifecycle phases (init/start/health/configure-auth/context/stop), dependency-aware batching, error propagation, diagnostics/log capture, per-run isolation under `.runs/<run-id>/`, and the component contract (`component.env`, required hook scripts, `--validate-components`). Propose improvements that keep the pluggable design but make it safer and easier to extend.
4. **Layered strategy** — unit (pure logic, metadata layering/templates, secret placeholder normalization, table-driven/property tests); integration (orchestrator with fake providers, conflict handling); E2E (user-visible CLI/operator behavior, negative cases, idempotency, ordering, partial failure, concurrency).

## What to enforce with new/changed tests
- Happy paths plus failure modes: invalid input, partial failure, timeouts/retries, concurrency, idempotency (apply twice = no-op), stable ordering, state transitions, version/format compatibility.
- Security invariants: secret non-disclosure, path-traversal rejection, auth negatives.
- E2E component-contract test suite validating every component implements the required standard.
- Schema guards: `schemas/*.json` stay valid JSON with expected top-level wiring when contracts change.

## Test design requirements
Deterministic (explicit waits/polling with timeouts, no wall-clock dependence); hermetic (own temp dirs/resources, no host-state reliance); observable (actionable diagnostics + preserved artifacts on failure); portable (Linux CI, single entrypoint); fast feedback (quick unit tests; staged smoke vs full E2E). For Bash: `set -euo pipefail`, standardized function/exit-code/output conventions, structured logging with case IDs, isolated per-test temp dirs, reusable assertion helpers.

## Deliverables
- **A. Current state** — what exists, what it covers, main gaps vs specs.
- **B. Harness review** — orchestrator + component-contract issues and concrete improvements.
- **C. Test matrix** — spec area → unit → integration → E2E → corner cases, with P0/P1/P2 priority.
- **D. Plan** — PR-sized steps: scope, tests added/changed, coverage gain, risk, acceptance criteria (commands + expected output).
- **E. Delivered changes** (when asked to implement) — tests pass locally and in CI; docs updated on how to run unit/E2E and add new components/tests.

Prefer black-box assertions (CLI/API output, exit codes, side effects) over internal inspection. Avoid heavy test dependencies unless justified.
