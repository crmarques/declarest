# E2E Harness and Component Contracts

## Purpose
Define the contract for the Bash E2E harness: profile behavior, component onboarding, case filtering, runtime status model, and manual handoff.

## In Scope
1. Runner entrypoint and profile semantics.
2. Component directory/script contracts.
3. Case catalog and requirement filtering.
4. Runtime step lifecycle and summary output.
5. Manual profile behavior and temporary context generation.

## Out of Scope
1. Non-E2E unit/integration provider assertions.
2. CI vendor job definitions.
3. Host/container runtime tuning.

## Normative Rules
1. The harness MUST expose repository entrypoint `run-e2e.sh` and delegate orchestration to `e2e/run-e2e.sh`.
2. Supported profiles MUST be `basic`, `full`, and `manual`; default is `basic`.
3. `basic` MUST run `main` cases only; `full` MUST run `main` + `corner` cases; both run only requirement-compatible cases.
4. `manual` MUST start selected local-instantiable components, generate temporary context config, and skip automated cases.
5. `manual` MUST seed the selected context repository directory with the selected resource-server `repo-template` tree when `resource-server != none`.
6. `manual` MUST reject remote-only connection selections during initialization with actionable validation output.
7. Runtime lifecycle MUST be profile-specific: `basic`/`full` use seven steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Running Workload`, `Finalizing`); `manual` uses five steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`).
8. Step statuses MUST be `RUNNING`, `OK`, `FAIL`, `SKIP`.
9. Non-TTY mode MUST emit deterministic plain logs; TTY mode MAY use live spinner/color output.
10. Final summary MUST include step outcomes, case counters, duration, context file path, and logs path.
11. Each component under `e2e/components/<type>/<name>/` MUST provide `component.env`, `scripts/init.sh`, `scripts/configure-auth.sh`, and `scripts/context.sh`.
12. Local compose-backed components MUST also provide `compose.yaml` and `scripts/health.sh`.
13. Component scripts MUST be ShellCheck-friendly Bash and publish generated runtime values through the component state file.
14. Resource-server components MUST ship fixture trees under `repo-template/` with collection metadata at `<logical-collection>/_/metadata.json` and resource payloads at `<logical-resource>/resource.json`.
15. Resource-server fixture metadata MUST model API-facing identifiers via `idFromAttribute` and `aliasFromAttribute` (for example, keycloak realms use `realm`).
16. The loader MUST expand intermediary `/_/` metadata placeholders into concrete collection targets before invoking `metadata set`.
17. Cases MUST define `CASE_ID`, `CASE_SCOPE`, `CASE_REQUIRES`, and `case_run`.
18. Missing requirements default to `SKIP`; they become `FAIL` when tied to explicitly requested capabilities/selections.
19. Runtime artifacts MUST be written under `e2e/.runs/<run-id>/` (logs, state, context, per-case workdirs).
20. User-facing E2E env vars MUST use `DECLAREST_E2E_*`; container engine selection MUST support `podman` or `docker` via `DECLAREST_E2E_CONTAINER_ENGINE` (default `podman`).
21. The runner MUST maintain one live execution log file and print its path at startup.
22. Cleanup mode flags (`--clean`, `--clean-all`) MUST short-circuit workload execution, stop referenced runner processes, and remove execution artifacts plus compose-backed runtime resources associated with each run.

## Data Contracts
Runner flags:
1. Workload: `--profile`.
2. Component selection: `--resource-server`, `--repo-type`, `--git-provider`, `--secret-provider`.
3. Connection selection: `--resource-server-connection`, `--git-provider-connection`, `--secret-provider-connection`.
4. Runtime controls: `--list-components`, `--keep-runtime`, `--verbose`.
5. Cleanup controls: `--clean`, `--clean-all`.

`component.env` fields:
1. `COMPONENT_TYPE`, `COMPONENT_NAME`.
2. `SUPPORTED_CONNECTIONS`, `DEFAULT_CONNECTION`.
3. `REQUIRES_DOCKER`.
4. `DESCRIPTION`.

Case requirements (`CASE_REQUIRES`):
1. Selector format: `key=value`.
2. Capability format: symbolic value such as `has-secret-provider`.
3. All requirements are AND-combined.

Case discovery order:
1. Global cases first: `e2e/cases/<scope>/`.
2. Selected component cases next: `e2e/components/<type>/<name>/cases/<scope>/`.
3. Discovery MUST be deterministic and deduplicated.

Manual handoff:
1. Emit temporary context catalog path.
2. Print concrete follow-up `declarest` commands.
3. Exit after startup and keep runtime resources available until explicit `--clean`/`--clean-all`.

## Failure Modes
1. Manual profile accepts unsupported remote selections.
2. Component hook failures are swallowed and execution continues with invalid state.
3. Partial startup passes without health checks.
4. Requirement filtering hides explicitly requested mandatory coverage.
5. Summary output omits actionable failing-step log pointers.

## Edge Cases
1. `--list-components` short-circuits runtime startup but still yields deterministic summary.
2. `manual` with explicit subset flags starts only selected local-instantiable components.
3. `full` runs validation/auth corner cases without marking run unstable when assertions pass.
4. Remote-capable selections with missing env credentials fail fast with guidance.
5. Nested fixture metadata patterns (for example `/x/_/y/_/_`) expand deterministically into concrete collection targets.
6. Cleanup for unknown run ids returns actionable output and still attempts runner/compose teardown using deterministic project naming.

## Examples
1. `./run-e2e.sh --profile basic --repo-type filesystem --resource-server none --secret-provider none` runs compatible main cases and reports deterministic summary.
2. `./run-e2e.sh --profile manual --repo-type git --git-provider github --git-provider-connection remote` fails initialization because manual mode is local-instantiable only.
