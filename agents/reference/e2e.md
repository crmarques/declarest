# E2E Harness and Component Contracts

## Purpose
Define the canonical contract for the Bash e2e harness, including profile behavior, component onboarding interface, runtime step model, and case filtering semantics.

## In Scope
1. Runner entrypoint and profile semantics.
2. Component directory and script contracts.
3. Case catalog structure and requirement filtering.
4. Runtime step lifecycle, status model, and summary expectations.
5. Manual handoff behavior and temporary context artifact rules.

## Out of Scope
1. Provider-specific business assertions for non-e2e unit or integration tests.
2. CI pipeline vendor configuration and job orchestration.
3. Docker host tuning and external infrastructure provisioning.

## Normative Rules
1. The e2e harness MUST be Bash-based with repository entrypoint `run-e2e.sh` delegating orchestration to `e2e/run-e2e.sh`.
2. The runner MUST support `--profile <basic|full|manual>` and default to `basic`.
3. `basic` profile MUST execute `main` cases only, limited to cases whose requirements are satisfied by the selected stack.
4. `full` profile MUST execute `main` and `corner` cases, limited to cases whose requirements are satisfied by the selected stack.
5. `manual` profile MUST start selected local-instantiable components, generate a temporary context catalog, and MUST NOT execute automated case scripts.
6. Profile selection MUST define workload behavior only; component flags MUST define stack composition and connection modes.
7. `manual` with no explicit component flags MUST select the maximal local-instantiable default stack.
8. `manual` with any remote component connection selection MUST fail validation during Step 1 with an actionable message.
9. The runtime lifecycle MUST use exactly seven grouped steps: `Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Running Workload`, `Finalizing`.
10. Step state transitions MUST use `RUNNING`, `OK`, `FAIL`, and `SKIP`.
11. TTY runs SHOULD provide single-line live progress with spinner and color; non-TTY runs MUST provide deterministic plain step logs.
12. Final summary MUST include step outcomes, case counters (pass/fail/skip), duration, context file path, and logs path.
13. Every component under `e2e/components/<type>/<name>/` MUST provide `component.env`, `scripts/init.sh`, `scripts/configure-auth.sh`, and `scripts/context.sh`.
14. Components that are local and Docker-backed MUST additionally provide `compose.yaml` and `scripts/health.sh`.
15. Component scripts MUST be ShellCheck-friendly Bash with robust error handling and MUST communicate generated runtime values through the component state file.
16. Case files MUST live under `e2e/cases/main/` or `e2e/cases/corner/` and MUST define `CASE_ID`, `CASE_SCOPE`, `CASE_REQUIRES`, and `case_run`.
17. Missing case requirements MUST yield `SKIP` by default; if the missing requirement maps to an explicitly requested capability/selection, the case MUST be marked `FAIL`.
18. Runtime artifacts MUST be written under `e2e/.runs/<run-id>/` and include logs, state, context output, and per-case work directories.

## Data Contracts
Runner flag groups:
1. Workload: `--profile`.
2. Component selection: `--resource-server`, `--repo-type`, `--git-provider`, `--secret-provider`.
3. Connection type selection: `--resource-server-connection`, `--git-provider-connection`, `--secret-provider-connection`.
4. Runtime controls: `--list-components`, `--keep-runtime`, `--verbose`.

Component metadata contract (`component.env`):
1. `COMPONENT_TYPE`: one bounded class such as `resource-server`, `repo-type`, `git-provider`, or `secret-provider`.
2. `COMPONENT_NAME`: unique name within type.
3. `SUPPORTED_CONNECTIONS`: space-separated connection values supported by the component.
4. `DEFAULT_CONNECTION`: default connection value from the supported set.
5. `REQUIRES_DOCKER`: `true` or `false`.
6. `DESCRIPTION`: short human-readable description.

Case requirement contract (`CASE_REQUIRES`):
1. Selector requirement format: `key=value`.
2. Capability requirement format: symbolic capability such as `has-secret-provider`.
3. Requirements are space-separated and all listed requirements are AND-combined.

Manual handoff contract:
1. The runner MUST emit a usable temporary context catalog path.
2. The runner MUST print concrete follow-up commands for `declarest` usage.
3. The manual session MUST remain active until user exit/interrupt, then finalize teardown unless `--keep-runtime` is set.

## Failure Modes
1. Profile validation permits incompatible remote selections in `manual`.
2. Component hook errors are swallowed and execution continues with invalid runtime state.
3. Local Docker component startup succeeds partially and health checks are skipped.
4. Case requirement filtering silently omits explicitly requested mandatory capability coverage.
5. Summary output lacks failing step log pointer, making failures non-actionable.

## Edge Cases
1. `--list-components` short-circuits runtime setup and component startup while still producing deterministic step/summary output.
2. `manual` profile with explicit subset component flags starts only selected local-instantiable components.
3. `full` profile runs corner cases that intentionally assert validation/auth failures without marking the run unstable when assertions pass.
4. Remote credential environment variables are missing for remote-capable components and validation or setup fails with clear guidance.

## Examples
1. Normal scenario: `./run-e2e.sh --profile basic --repo-type filesystem --resource-server none --secret-provider none` runs main cases, skipping stack-incompatible ones, and exits with deterministic summary.
2. Corner scenario: `./run-e2e.sh --profile manual --repo-type git --git-provider github --git-provider-connection remote --resource-server none --secret-provider none` fails in Step 1 because manual mode is local-instantiable only.
