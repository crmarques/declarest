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
1. The harness MUST expose repository entrypoint `run-e2e.sh` and delegate orchestration to `test/e2e/run-e2e.sh`.
2. Supported profiles MUST be `basic`, `full`, and `manual`; default is `basic`.
3. Default component selections MUST be `resource-server=simple-api-server`, `repo-type=filesystem`, and `secret-provider=file`.
4. `basic` MUST run `main` cases only; `full` MUST run `main` + `corner` cases; both run only requirement-compatible cases.
5. `manual` MUST start selected local-instantiable components, generate temporary context config, generate shell setup/reset scripts for handoff, and skip automated cases.
6. `manual` MUST seed the selected context repository directory with the selected resource-server `repo-template` tree when `resource-server != none`.
7. `manual` MUST reject remote-only connection selections during initialization with actionable validation output.
8. Runtime lifecycle MUST be profile-specific: `basic`/`full` use seven steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Running Workload`, `Finalizing`); `manual` uses five steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`).
9. Step statuses MUST be `RUNNING`, `OK`, `FAIL`, `SKIP`.
10. Non-TTY mode MUST emit deterministic plain logs; TTY mode MAY use live spinner/color output.
11. Final summary MUST include step outcomes, case counters, duration, context file path, and logs path.
12. Each component under `test/e2e/components/<type>/<name>/` MUST provide `component.env`, `scripts/init.sh`, `scripts/configure-auth.sh`, and `scripts/context.sh`.
13. Local compose-backed components MUST also provide `compose.yaml` and `scripts/health.sh`.
14. `component.env` MUST declare `COMPONENT_RUNTIME_KIND` and `COMPONENT_DEPENDS_ON` explicitly.
15. The runner MUST execute component hooks through one generic hook orchestration path (`init`, `start`, `health`, `configure-auth`, `context`, `stop`) rather than per-component ad hoc branching.
16. Hook orchestration MUST be dependency-aware using `COMPONENT_DEPENDS_ON` and MUST run ready batches in parallel when no dependency edge blocks them.
17. Missing dependency targets and dependency cycles MUST fail initialization or hook execution with actionable output.
18. Component scripts MUST be ShellCheck-friendly Bash and publish generated runtime values through the component state file.
19. Resource-server components MUST ship fixture trees under `repo-template/` with collection metadata at `<logical-collection>/_/metadata.json` and resource payloads at `<logical-resource>/resource.json`.
20. Resource-server fixture metadata MUST model API-facing identifiers via `idFromAttribute` and `aliasFromAttribute` (for example, keycloak realms use `realm`).
21. The loader MUST expand intermediary `/_/` metadata placeholders into concrete collection targets before invoking `metadata set`.
22. Cases MUST define `CASE_ID`, `CASE_SCOPE`, `CASE_REQUIRES`, and `case_run`.
23. Missing requirements default to `SKIP`; they become `FAIL` when tied to explicitly requested capabilities/selections.
24. Runtime artifacts MUST be written under `test/e2e/.runs/<run-id>/` (logs, state, context, per-case workdirs).
25. User-facing E2E env vars MUST use `DECLAREST_E2E_*`; container engine selection MUST support `podman` or `docker` via `DECLAREST_E2E_CONTAINER_ENGINE` (default `podman`).
26. The runner MUST maintain one live execution log file and print its path at startup.
27. Cleanup mode flags (`--clean`, `--clean-all`) MUST short-circuit workload execution, stop referenced runner processes, and remove execution artifacts plus compose-backed runtime resources associated with each run.
28. Components MAY implement optional `scripts/manual-info.sh`; in `manual` profile, the runner MUST execute this hook for selected components after `Configuring Access` and print its output to terminal.
29. Runner security selection flags MUST include `--resource-server-basic-auth`, `--resource-server-oauth2`, and `--resource-server-mtls` with defaults `false`, `true`, and `false`, respectively.
30. `--resource-server-basic-auth` and `--resource-server-oauth2` MUST NOT both be `true` in the same run because `managed-server.http.auth` is one-of.
31. `resource-server` components MUST declare security capabilities in `component.env`; runner selection MUST fail when requested security features are unsupported or required features are disabled.
32. `simple-api-server` mTLS trust MUST be reloaded from configured client-certificate sources for new connections without process restart.
33. `simple-api-server` mTLS mode MUST allow an empty trusted-certificate set and deny all client API access until trusted certificates are added.

## Data Contracts
Runner flags:
1. Workload: `--profile`.
2. Component selection: `--resource-server`, `--repo-type`, `--git-provider`, `--secret-provider`.
3. Resource-server security selection: `--resource-server-basic-auth`, `--resource-server-oauth2`, `--resource-server-mtls`.
4. Connection selection: `--resource-server-connection`, `--git-provider-connection`, `--secret-provider-connection`.
5. Runtime controls: `--list-components`, `--keep-runtime`, `--verbose`.
6. Cleanup controls: `--clean`, `--clean-all`.

`component.env` fields:
1. `COMPONENT_TYPE`, `COMPONENT_NAME`.
2. `SUPPORTED_CONNECTIONS`, `DEFAULT_CONNECTION`.
3. `REQUIRES_DOCKER`.
4. `COMPONENT_RUNTIME_KIND` (`native|compose`).
5. `COMPONENT_DEPENDS_ON` (space-separated dependency selectors using `<type>:<name>` or `<type>:*`).
6. `DESCRIPTION`.
7. `SUPPORTED_SECURITY_FEATURES` (`resource-server` only): whitespace-separated subset of `basic-auth oauth2 mtls`.
8. `REQUIRED_SECURITY_FEATURES` (`resource-server` optional): whitespace-separated subset of `SUPPORTED_SECURITY_FEATURES`.

Optional component hook:
1. `scripts/manual-info.sh` may emit plain-text operator access details for `manual` profile output.
2. `scripts/start.sh` and `scripts/stop.sh` may override built-in compose runtime lifecycle adapters.

Case requirements (`CASE_REQUIRES`):
1. Selector format: `key=value`.
2. Capability format: symbolic value such as `has-secret-provider`.
3. All requirements are AND-combined.

Case discovery order:
1. Global cases first: `test/e2e/cases/<scope>/`.
2. Selected component cases next: `test/e2e/components/<type>/<name>/cases/<scope>/`.
3. Discovery MUST be deterministic and deduplicated.

Manual handoff:
1. Emit temporary context catalog path.
2. Emit setup/reset shell script paths; setup script MUST export runtime vars and define alias `declarest-e2e` to the run-local binary, and reset script MUST unset those vars and remove the alias.
3. Print concrete follow-up `declarest-e2e` commands.
4. Exit after startup and keep runtime resources available until explicit `--clean`/`--clean-all`.

## Failure Modes
1. Manual profile accepts unsupported remote selections.
2. Component hook failures are swallowed and execution continues with invalid state.
3. Partial startup passes without health checks.
4. Requirement filtering hides explicitly requested mandatory coverage.
5. Summary output omits actionable failing-step log pointers.
6. Component dependency selectors reference non-discovered components and fail late.
7. Hook dependency cycles deadlock startup sequencing.

## Edge Cases
1. `--list-components` short-circuits runtime startup but still yields deterministic summary.
2. `manual` with explicit subset flags starts only selected local-instantiable components.
3. `full` runs validation/auth corner cases without marking run unstable when assertions pass.
4. Remote-capable selections with missing env credentials fail fast with guidance.
5. Nested fixture metadata patterns (for example `/x/_/y/_/_`) expand deterministically into concrete collection targets.
6. Cleanup for unknown run ids returns actionable output and still attempts runner/compose teardown using deterministic project naming.
7. Dependency selector wildcard (for example `git-provider:*`) resolves exactly one selected provider in some runs and multiple in others without changing correctness.
8. Local `simple-api-server` with `ENABLE_MTLS=true` generates or consumes mounted cert material from a host directory and fails fast when required TLS files are missing.
9. Selecting both `--resource-server-basic-auth=true` and `--resource-server-oauth2=true` fails fast before startup with actionable validation output.
10. Local `simple-api-server` mTLS trust directory can transition from non-empty to empty and back during runtime; API access follows the current trust set for new connections.

## Examples
1. `./run-e2e.sh --profile basic --repo-type filesystem --resource-server none --secret-provider none` runs compatible main cases and reports deterministic summary.
2. `./run-e2e.sh --profile manual --repo-type git --git-provider github --git-provider-connection remote` fails initialization because manual mode is local-instantiable only.
3. `./run-e2e.sh --profile basic --repo-type git --git-provider gitlab --resource-server simple-api-server` runs dependency-aware parallel hooks while ensuring `repo-type:git` waits for `git-provider:*` initialization.
4. `./run-e2e.sh --resource-server keycloak --resource-server-oauth2 false` fails selection because keycloak requires oauth2 security support.
5. `./run-e2e.sh --profile full --resource-server simple-api-server --resource-server-mtls true` validates runtime mTLS trust reload by removing and re-adding trusted client certificates without restarting the server.
