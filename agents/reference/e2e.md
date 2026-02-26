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
6. `manual` MUST seed the selected context repository directory with the selected resource-server `repo-template` tree.
7. `manual` MUST reject remote-only connection selections during initialization with actionable validation output.
8. Runtime lifecycle MUST be profile-specific: `basic`/`full` use seven steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Running Test Cases`, `Finalizing`); `manual` uses five steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`).
9. Step statuses MUST be `RUNNING`, `OK`, `FAIL`, `SKIP` and rendered as bracketed labels such as `[RUNNING]`, `[OK]`, `[FAILED]`, `[SKIP]`.
10. Non-TTY mode MUST emit deterministic plain logs; TTY mode MAY use live spinner/color output.
11. Step output MUST resemble a table framed by divider lines above and below the header row `STEP | ACTION | SPAN | STATUS`, print that header once per run, center each header label within its column, render spinner glyphs inside the `ACTION` column while a step is running, populate `SPAN` only after the step finishes, and center `STEP`, `SPAN`, and `STATUS` values for every row.
12. Final summary MUST include step outcomes, case counters, duration, context file path, and logs path.
13. Each component under `test/e2e/components/<type>/<name>/` MUST provide `component.env`, `scripts/init.sh`, `scripts/configure-auth.sh`, and `scripts/context.sh`.
14. Local compose-backed components MUST also provide `compose.yaml` and `scripts/health.sh`.
15. `component.env` MUST declare `COMPONENT_CONTRACT_VERSION=1`, `COMPONENT_RUNTIME_KIND`, and `COMPONENT_DEPENDS_ON` explicitly.
16. The runner MUST expose `--validate-components` to validate all discovered component contracts and resource-server fixture metadata, then short-circuit workload execution with deterministic summary output.
17. The runner MUST reject `--resource-server none` with actionable validation output because resource-server selection is mandatory for E2E runs.
18. The runner MUST execute component hooks through one generic hook orchestration path (`init`, `start`, `health`, `configure-auth`, `context`, `stop`) rather than per-component ad hoc branching.
19. Hook orchestration MUST be dependency-aware using `COMPONENT_DEPENDS_ON` and MUST run ready batches in parallel when no dependency edge blocks them.
20. Missing dependency targets and dependency cycles MUST fail initialization or hook execution with actionable output.
21. Component scripts MUST be ShellCheck-friendly Bash and publish generated runtime values through the component state file.
22. Resource-server components MUST ship fixture trees under `repo-template/` with collection metadata at `<logical-collection>/_/metadata.json` and resource payloads at `<logical-resource>/resource.json`.
23. Resource-server fixture metadata MUST model API-facing identifiers via `idFromAttribute` and `aliasFromAttribute` (for example, keycloak realms use `realm`).
24. The runner `--validate-components` mode MUST reject resource-server fixture metadata files that omit `resourceInfo.idFromAttribute` or `resourceInfo.aliasFromAttribute`.
25. The loader MUST expand intermediary `/_/` metadata placeholders into concrete collection targets before invoking `metadata set`.
26. Cases MUST define `CASE_ID`, `CASE_SCOPE`, `CASE_REQUIRES`, and `case_run`.
27. Missing requirements default to `SKIP`; they become `FAIL` when tied to explicitly requested capabilities/selections.
28. Runtime artifacts MUST be written under `test/e2e/.runs/<run-id>/` (logs, state, context, per-case workdirs).
29. User-facing E2E env vars MUST use `DECLAREST_E2E_*`; container engine selection MUST support `podman` or `docker` via `DECLAREST_E2E_CONTAINER_ENGINE` (default `podman`).
30. The runner MUST maintain one live execution log file and print its path at startup.
31. Cleanup mode flags (`--clean`, `--clean-all`) MUST short-circuit workload execution, stop referenced runner processes, and remove execution artifacts plus compose-backed runtime resources associated with each run, and they MUST also drop any run-specific `PATH` entries (for example `<run-dir>/bin`) that the manual profile prepended so shells no longer reference cleaned runs.
32. Components MAY implement optional `scripts/manual-info.sh`; in `manual` profile, the runner MUST execute this hook for selected components after `Configuring Access` and print its output to terminal.
33. Runner security selection flags MUST include `--resource-server-auth-type <none|basic|oauth2|custom-header>` and `--resource-server-mtls`; `--resource-server-mtls` defaults to `false`, and auth type defaults MUST be elected by the selected resource-server component when the flag is omitted (preference order SHOULD be `oauth2`, then `custom-header`, then `basic`, then `none`).
34. `resource-server` components MUST declare security capabilities in `component.env`, including at least one auth-type capability token (`none|basic-auth|oauth2|custom-header`) and optional `mtls`; runner selection MUST fail when requested auth-type or mTLS features are unsupported or required features are disabled.
35. When a resource-server component cannot support a selected auth-type or mTLS combination for the chosen connection mode, its hooks MUST fail with actionable output if selection validation did not already reject the combination.
36. `simple-api-server` mTLS trust MUST be reloaded from configured client-certificate sources for new connections without process restart.
37. `simple-api-server` mTLS mode MUST allow an empty trusted-certificate set and deny all client API access until trusted certificates are added.
38. `manual` with `repo-type=git` MUST run `repo init` after context assembly so repository-dependent checks (`config check`, `repo status`) are immediately usable.
39. Components MAY include `openapi.yaml`; the runner copies any discovered spec into the run directory, exposes its path via `E2E_COMPONENT_OPENAPI_SPEC`, and lets the component's `context` hook echo the resulting value (for example `resource-server.http.openapi`) so metadata-aware commands can infer the API surface.
40. Resource-server components MAY ship a sibling `metadata/` directory; when present, the runner MUST mirror that directory under `<run-dir>/metadata`, set `E2E_METADATA_DIR` to the mirrored path, and ensure repository-type context fragments emit `metadata.base-dir` using `E2E_METADATA_DIR` (fallbacking to the repo base dir when unset) so metadata-aware commands resolve fixture directives from the copied directory.

## Data Contracts
Runner flags:
1. Workload: `--profile`.
2. Component selection: `--resource-server`, `--repo-type`, `--git-provider`, `--secret-provider`.
3. Resource-server security selection: `--resource-server-auth-type`, `--resource-server-mtls`.
4. Connection selection: `--resource-server-connection`, `--git-provider-connection`, `--secret-provider-connection`.
5. Runtime controls: `--list-components`, `--validate-components`, `--keep-runtime`, `--verbose`.
6. Cleanup controls: `--clean`, `--clean-all`.

`component.env` fields:
1. `COMPONENT_TYPE`, `COMPONENT_NAME`.
2. `SUPPORTED_CONNECTIONS`, `DEFAULT_CONNECTION`.
3. `REQUIRES_DOCKER`.
4. `COMPONENT_CONTRACT_VERSION` (current supported value `1`).
5. `COMPONENT_RUNTIME_KIND` (`native|compose`).
6. `COMPONENT_DEPENDS_ON` (space-separated dependency selectors using `<type>:<name>` or `<type>:*`).
7. `DESCRIPTION`.
8. `SUPPORTED_SECURITY_FEATURES` (`resource-server` only): whitespace-separated subset of `none basic-auth oauth2 custom-header mtls`, including at least one auth-type capability token.
9. `REQUIRED_SECURITY_FEATURES` (`resource-server` optional): whitespace-separated subset of `SUPPORTED_SECURITY_FEATURES`, with at most one auth-type capability token.

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
9. Selecting `--resource-server-auth-type` unsupported by the chosen resource-server component fails fast before startup with actionable validation output.
10. Local `simple-api-server` mTLS trust directory can transition from non-empty to empty and back during runtime; API access follows the current trust set for new connections.
11. `manual` with `repo-type=git` still initializes a local git repository so readiness checks do not fail with `git repository not initialized`.
12. Cleanup while a manual profile shell is still sourced MUST drop the `<run-dir>/bin` PATH insertion so the shell no longer resolves the deleted `declarest-e2e` alias or binary.

## Examples
1. `./run-e2e.sh --profile basic --repo-type filesystem --resource-server simple-api-server --secret-provider none` runs compatible main cases and reports deterministic summary.
2. `./run-e2e.sh --profile manual --repo-type git --git-provider github --git-provider-connection remote` fails initialization because manual mode is local-instantiable only.
3. `./run-e2e.sh --profile basic --repo-type git --git-provider gitlab --resource-server simple-api-server` runs dependency-aware parallel hooks while ensuring `repo-type:git` waits for `git-provider:*` initialization.
4. `./run-e2e.sh --resource-server keycloak --resource-server-auth-type none` fails selection because keycloak requires oauth2 auth-type support.
5. `./run-e2e.sh --profile full --resource-server simple-api-server --resource-server-mtls true` validates runtime mTLS trust reload by removing and re-adding trusted client certificates without restarting the server.
6. `./run-e2e.sh --profile basic --repo-type git --git-provider gitea --resource-server simple-api-server --secret-provider file` runs git main-case coverage against a local compose-backed Gitea provider.
7. `./run-e2e.sh --profile basic --repo-type git --git-provider gitea --git-provider-connection remote --resource-server simple-api-server --secret-provider none` fails `Preparing Components` when required `DECLAREST_E2E_GITEA_*` remote credentials are missing.
8. `./run-e2e.sh --profile manual --repo-type git --git-provider gitea --resource-server simple-api-server --secret-provider none` yields a handoff context where `declarest-e2e config check` and `declarest-e2e repo status` can run without `git repository not initialized`.
9. `./run-e2e.sh --validate-components` validates all discovered component manifests, hook scripts, dependency catalog, and resource-server fixture metadata, then exits without running test cases.
