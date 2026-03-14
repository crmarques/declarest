# E2E Harness and Component Contracts

## Purpose
Define the contract for the Bash E2E harness: profile behavior, component onboarding, case filtering, runtime status model, and profile handoff workflows.

## In Scope
1. Runner entrypoint and profile semantics.
2. Component directory/script contracts.
3. Case catalog and requirement filtering.
4. Runtime step lifecycle and summary output.
5. Manual/operator profile behavior and temporary context generation.

## Out of Scope
1. Non-E2E unit/integration provider assertions.
2. CI vendor job definitions.
3. Host/container runtime tuning.

## Normative Rules
1. The harness MUST expose repository entrypoint `run-e2e.sh` and delegate orchestration to `test/e2e/run-e2e.sh`.
2. Supported profiles MUST be `cli-basic`, `cli-full`, `cli-manual`, `operator-manual`, `operator-basic`, and `operator-full`; default is `cli-basic`.
3. Runner platform selection MUST support `--platform <compose|kubernetes>` with default `kubernetes`.
3. Default component selections MUST be `managed-server=simple-api-server`, `repo-type=filesystem`, and `secret-provider=file`.
4. `cli-basic` MUST run `smoke` cases only; `cli-full` MUST run `smoke` + `main` + `corner` cases; both run only requirement-compatible cases.
5. `cli-manual` MUST start selected local-instantiable components, generate temporary context config, generate shell setup/reset scripts for handoff, and skip automated cases.
6. `cli-manual` MUST seed the selected context repository directory with the selected managed-server `repo-template` tree.
7. `cli-manual` MUST reject remote-only connection selections during initialization with actionable validation output.
8. Runtime lifecycle MUST be profile-specific: `cli-basic`/`cli-full` use seven steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Running Test Cases`, `Finalizing`); `cli-manual` uses five steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`); `operator-manual` uses seven steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Installing Operator`, `Finalizing`); `operator-basic`/`operator-full` use eight steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Installing Operator`, `Running Test Cases`, `Finalizing`).
9. Step statuses MUST be `RUNNING`, `OK`, `FAIL`, `SKIP` and rendered as bracketed labels such as `[RUNNING]`, `[OK]`, `[FAILED]`, `[SKIP]`.
10. Non-TTY mode MUST emit deterministic plain logs; TTY mode MAY use live spinner/color output.
11. Step output MUST resemble a table framed by divider lines above and below the header row `STEP | ACTION | SPAN | STATUS`, print that header once per run, center each header label within its column, render spinner glyphs inside the `ACTION` column while a step is running, show the elapsed duration in `SPAN` for running steps and the final duration after completion, and center `STEP`, `SPAN`, and `STATUS` values for every row.
12. Final summary MUST include step outcomes, resolved execution parameters (including explicit selections and defaulted values), case counters, duration, context file path, and logs path.
13. Each component under `test/e2e/components/<type>/<name>/` MUST provide `component.env`, `scripts/init.sh`, `scripts/configure-auth.sh`, and `scripts/context.sh`.
14. Local containerized (`COMPONENT_RUNTIME_KIND=compose`) components MUST provide `compose/compose.yaml`, at least one `k8s/*.yaml` manifest, and `scripts/health.sh`.
15. `component.env` MUST declare `COMPONENT_CONTRACT_VERSION=1`, `COMPONENT_RUNTIME_KIND`, and `COMPONENT_DEPENDS_ON` explicitly.
16. The runner MUST expose `--validate-components` to validate all discovered component contracts and managed-server fixture metadata, then short-circuit workload execution with deterministic summary output.
17. The runner MUST reject `--managed-server none` with actionable validation output because managed-server selection is mandatory for E2E runs.
18. The runner MUST execute component hooks through one generic hook orchestration path (`init`, `start`, `health`, `configure-auth`, `context`, `stop`) rather than per-component ad hoc branching.
19. Hook orchestration MUST be dependency-aware using `COMPONENT_DEPENDS_ON` and MUST run ready batches in parallel when no dependency edge blocks them.
20. Missing dependency targets and dependency cycles MUST fail initialization or hook execution with actionable output.
21. Component scripts MUST be ShellCheck-friendly Bash and publish generated runtime values through the component state file.
22. Resource-server components MUST ship fixture trees under `repo-template/` with collection metadata at `<logical-collection>/_/metadata.yaml` or `<logical-collection>/_/metadata.json` and resource payloads at `<logical-resource>/resource.json`; YAML SHOULD be the default for new fixture metadata files.
23. Resource-server fixture metadata MUST model API-facing identifiers via `resource.id` and `resource.alias` identity templates (for example, keycloak realms use `{{/realm}}`).
24. The runner `--validate-components` mode MUST reject managed-server fixture metadata files that omit `resource.id` or `resource.alias`.
25. The loader MUST expand intermediary `/_/` metadata placeholders into concrete collection targets before invoking `resource metadata set`.
26. Cases MUST define `CASE_ID`, `CASE_SCOPE`, `CASE_REQUIRES`, and `case_run`; they MAY declare `CASE_PROFILES` as a whitespace-separated subset of `cli operator`.
27. `smoke` scope MUST represent the curated fast suite used by `cli-basic` and `operator-basic`.
28. Cases default to `CASE_PROFILES=cli` unless `CASE_SCOPE=operator-main`, which defaults to `CASE_PROFILES=operator`.
29. Cases discovered for a profile family MUST be filtered by `CASE_PROFILES` before workload execution.
30. Missing requirements default to `SKIP`; they become `FAIL` when tied to explicitly requested capabilities/selections.
31. Runtime artifacts MUST be written under `test/e2e/.runs/<run-id>/` (logs, state, context, per-case workdirs).
32. When managed-server metadata is sourced from a component `metadata/` directory, the runner MUST copy it into a run-scoped workspace under `test/e2e/.runs/<run-id>/` before generating contexts so metadata-mutating cases never write into checked-in component fixtures.
33. User-facing E2E env vars MUST use `DECLAREST_E2E_*`; container engine selection MUST support `podman` or `docker` via `DECLAREST_E2E_CONTAINER_ENGINE` (default `podman`).
34. The runner MUST maintain one live execution log file and print its path at startup.
35. Cleanup mode flags (`--clean`, `--clean-all`) MUST short-circuit workload execution, stop referenced runner processes, and remove execution artifacts plus run-recorded runtime resources associated with each run (`compose` projects or `kind` clusters), and they MUST also drop any run-specific `PATH` entries (for example `<run-dir>/bin`) that `cli-manual` or `operator-manual` handoff prepended so shells no longer reference cleaned runs.
32. Components MAY implement optional `scripts/manual-info.sh`; in `cli-manual` and `operator-manual` profiles, the runner MUST execute this hook for selected components after startup and print aggregated hook output in a `Manual Component Access` handoff section before `Repository provider access`.
33. When the selected managed-server component has no `scripts/manual-info.sh` hook or it emits no output, manual profiles MUST print state-derived managed-server connection details in the same `Manual Component Access` section when that state is available.
34. Runner security selection flags MUST include `--managed-server-auth-type <none|basic|oauth2|custom-header>` and `--managed-server-mtls`; `--managed-server-mtls` defaults to `false`, and auth type defaults MUST be elected by the selected managed-server component when the flag is omitted (preference order SHOULD be `oauth2`, then `custom-header`, then `basic`, then `none`).
35. Runner proxy selection flags MUST include `--managed-server-proxy [<true|false>]`; default MUST be `false`, and when enabled the generated context MUST include `managed-server.http.proxy` from `DECLAREST_E2E_MANAGED_SERVER_PROXY_*` values with at least one of `http-url` or `https-url`.
36. `managed-server` components MUST declare security capabilities in `component.env`, including at least one auth-type capability token (`none|basic-auth|oauth2|custom-header`) and optional `mtls`; runner selection MUST fail when requested auth-type or mTLS features are unsupported or required features are disabled.
37. When a managed-server component cannot support a selected auth-type or mTLS combination for the chosen connection mode, its hooks MUST fail with actionable output if selection validation did not already reject the combination.
38. `simple-api-server` mTLS trust MUST be reloaded from configured client-certificate sources for new connections without process restart.
39. `simple-api-server` mTLS mode MUST allow an empty trusted-certificate set and deny all client API access until trusted certificates are added.
40. `cli-manual` with `repo-type=git` MUST run `repository init` after context assembly so repository-dependent checks (`context check`, `repository status`) are immediately usable.
41. Runner metadata selection flags MUST include `--metadata-source <bundle|dir>`; default mode MUST be `bundle`, and the runner SHOULD continue accepting legacy `--metadata-type <bundle|base-dir>` as a compatibility alias.
42. In `bundle` mode, the runner MUST skip local `openapi.yaml` wiring so `managed-server.http.openapi` remains unset, and MUST use managed-server shorthand metadata bundle mappings when available (for example `keycloak-bundle:0.0.1` for `keycloak`).
43. In `dir` mode, managed-server components MAY ship a sibling `metadata/` directory; when present, the runner MUST set `E2E_METADATA_DIR` to that component-local directory and repository-type context fragments MUST emit `metadata.base-dir` using `E2E_METADATA_DIR` (fallbacking to the repo base dir when unset).
44. In `bundle` mode, when the selected managed-server has no shorthand mapping, the runner MUST fall back to the component-local `metadata/` directory as `metadata.base-dir` when present; otherwise it MUST continue without setting `metadata.bundle`.
45. Kubernetes runtime MUST use run-scoped `kind` clusters when platform is `kubernetes` and at least one local containerized component is selected; it MUST persist runtime state (`platform`, `container engine`, `cluster name`, `namespace`, `kubeconfig`) for cleanup/manual handoff.
46. Kubernetes component startup MUST apply rendered `k8s/*.yaml` manifests in the run namespace and manage service port-forwards from `declarest.e2e/port-forward` service annotations, persisting forward PIDs in component state for stop/cleanup.
47. For `DECLAREST_E2E_CONTAINER_ENGINE=podman`, kind operations MUST use provider mode `KIND_EXPERIMENTAL_PROVIDER=podman` and preflight MUST fail fast with actionable guidance when provider checks fail.
48. `operator-manual`, `operator-basic`, and `operator-full` MUST enforce kubernetes-only local-instantiable selections (`--platform kubernetes`, `--repo-type git`, `--git-provider <gitea|gitlab>`, `--secret-provider <file|vault>`, and local connections for selected components) and MUST fail initialization with actionable validation output when unsupported combinations are selected.
49. Operator profiles MUST seed selected managed-server fixture content into the repository, initialize git, commit/push seeded content to the selected git provider, install operator CRDs, build/load a run-scoped operator image, deploy `declarest-operator-manager` in the run namespace, and generate/apply `ResourceRepository`, `ManagedServer`, `SecretStore`, and `SyncPolicy` CRs.
50. `operator-manual` handoff output MUST include run-scoped setup/reset shell scripts, operator runtime details, and concrete repository-to-managed-server verification commands using managed-server-specific logical path/payload examples.
51. `operator-basic` MUST execute `smoke` plus `operator-main` scope automated cases after operator installation; `operator-full` MUST execute compatible `smoke`, compatible `main`, `operator-main`, and `corner` scope automated cases.
52. Operator readiness waits (`DECLAREST_E2E_OPERATOR_READY_TIMEOUT_SECONDS`) MUST default to `120`, reject non-positive values, and cap at `600`.
53. Operator profiles with git providers `gitea|gitlab` MUST precompute run-scoped repository webhook URL/secret values, configure provider webhooks during access setup, and emit `spec.git.webhook` configuration in generated `ResourceRepository` CRs.
54. Operator profile manager manifests MUST expose a dedicated in-cluster repository-webhook service endpoint and pass `--repository-webhook-bind-address` to the manager container.

## Data Contracts
Runner flags:
1. Workload: `--profile`.
2. Platform: `--platform`.
2. Component selection: `--managed-server`, `--metadata-source`, `--repo-type`, `--git-provider`, `--secret-provider`.
3. Resource-server security selection: `--managed-server-auth-type`, `--managed-server-mtls`, `--managed-server-proxy`.
4. Connection selection: `--managed-server-connection`, `--git-provider-connection`, `--secret-provider-connection`.
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
8. `SUPPORTED_SECURITY_FEATURES` (`managed-server` only): whitespace-separated subset of `none basic-auth oauth2 custom-header mtls`, including at least one auth-type capability token.
9. `REQUIRED_SECURITY_FEATURES` (`managed-server` optional): whitespace-separated subset of `SUPPORTED_SECURITY_FEATURES`, with at most one auth-type capability token.

Optional component hook:
1. `scripts/manual-info.sh` may emit plain-text access details for `cli-manual` and `operator-manual` profile output; when emitted, runner output groups details in a `Manual Component Access` handoff section before `Repository provider access`.
2. When the selected managed-server hook is absent or empty, the runner MAY derive fallback access details from the managed-server component state file and print them in the same section.
3. `scripts/start.sh` and `scripts/stop.sh` may override built-in compose runtime lifecycle adapters.
4. Built-in adapters are platform-aware: compose (`compose/compose.yaml`) or kubernetes (`k8s/*.yaml` + service annotation-driven port-forward).
5. Successful `init` and `configure-auth` hooks MUST leave `${E2E_COMPONENT_STATE_FILE}` non-empty; successful `context` hooks for components that own persisted context sections (`managed-server`, `repo-type`, `secret-provider`) MUST leave `${E2E_COMPONENT_CONTEXT_FRAGMENT}` non-empty.

Case requirements (`CASE_REQUIRES`):
1. Selector format: `key=value`.
2. Capability format: symbolic value such as `has-secret-provider`.
3. All requirements are AND-combined.

`CASE_PROFILES`:
1. Allowed values: `cli`, `operator`.
2. Omitted value defaults to `cli`, except `operator-main` defaults to `operator`.
3. Cases whose `CASE_PROFILES` omit the active profile family MUST be excluded before execution.

Case discovery order:
1. Global cases first: `test/e2e/cases/<scope>/`.
2. Selected component cases next: `test/e2e/components/<type>/<name>/cases/<scope>/`.
3. Discovery MUST be deterministic and deduplicated.

Manual handoff:
1. Emit temporary context catalog path.
2. Emit setup/reset shell script paths; setup script MUST export runtime vars and define alias `declarest-e2e` to the run-local binary, and reset script MUST unset those vars and remove the alias.
3. Print concrete follow-up `declarest-e2e` commands.
4. Exit after startup and keep runtime resources available until explicit `--clean`/`--clean-all`.
5. When platform is `kubernetes`, print cluster access details (`cluster`, `namespace`, `kubeconfig`) and example `kubectl` commands.
6. When selected components emit manual-info details, print those details in `Manual Component Access` before `Repository provider access`.

Operator handoff:
1. Emit temporary context catalog path and run-scoped setup/reset shell scripts.
2. Print operator runtime details (`manager-deployment`, `manager-pod`, `manager-logs`, namespace, sync-policy name).
3. Print concrete `declarest-e2e` commands to save a repository resource, commit/push it, and read the same logical path from the managed server.
4. Exit after startup and keep runtime resources available until explicit `--clean`/`--clean-all`.
5. When selected components emit manual-info details, print those details in `Manual Component Access` before `Repository provider access`.
6. Include repository-webhook runtime details (`repository-webhook-url`) and one concrete `kubectl` command to inspect webhook receipt annotations on the generated `ResourceRepository`.

## Failure Modes
1. `cli-manual` profile accepts unsupported remote selections.
2. Component hook failures are swallowed and execution continues with invalid state.
3. Partial startup passes without health checks.
4. Requirement filtering hides explicitly requested mandatory coverage.
5. Summary output omits actionable failing-step log pointers.
6. Component dependency selectors reference non-discovered components and fail late.
7. Hook dependency cycles deadlock startup sequencing.
8. Operator profile leaves repository webhook URL/secret unset, so git provider hooks are not registered and reconcile falls back to poll interval.

## Edge Cases
1. `--list-components` short-circuits runtime startup but still yields deterministic summary.
2. `cli-manual` with explicit subset flags starts only selected local-instantiable components.
3. `cli-full` runs validation/auth corner cases without marking run unstable when assertions pass.
4. Remote-capable selections with missing env credentials fail fast with guidance.
5. Nested fixture metadata patterns (for example `/x/_/y/_/_`) expand deterministically into concrete collection targets.
6. Cleanup for unknown run ids returns actionable output and still attempts runner/runtime teardown using recorded runtime state (fallback compose naming when state is missing).
7. Dependency selector wildcard (for example `git-provider:*`) resolves exactly one selected provider in some runs and multiple in others without changing correctness.
8. Local `simple-api-server` with `ENABLE_MTLS=true` generates or consumes mounted cert material from a host directory and fails fast when required TLS files are missing.
9. Selecting `--managed-server-auth-type` unsupported by the chosen managed-server component fails fast before startup with actionable validation output.
10. Local `simple-api-server` mTLS trust directory can transition from non-empty to empty and back during runtime; API access follows the current trust set for new connections.
11. `cli-manual` with `repo-type=git` still initializes a local git repository so readiness checks do not fail with `git repository not initialized`.
12. Cleanup while a manual profile shell is still sourced MUST drop the `<run-dir>/bin` PATH insertion so the shell no longer resolves the deleted `declarest-e2e` alias or binary.
13. `managed-server=keycloak` runs in `bundle` mode fail during context validation when shorthand bundle `keycloak-bundle:0.0.1` cannot be resolved from the default remote path.
14. `bundle` mode with a managed-server that has no shorthand mapping falls back to component-local `metadata/` when present, otherwise continues without `metadata.bundle`; in both cases local `openapi.yaml` remains unset.
15. Metadata-mutating E2E cases (for example `resource metadata set` or `secret detect --fix`) write only into the run-scoped metadata workspace copy and MUST leave checked-in component metadata directories unchanged.
16. Legacy `--metadata-type base-dir` selections normalize to `metadata-source=dir`, while `--metadata-source base-dir` MUST fail argument validation before runtime startup.
17. `--platform kubernetes` with only remote/native selections MUST not create a kind cluster.
18. `--managed-server-proxy true` without `DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL` or `DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL` fails argument validation before runtime startup.
19. `--profile operator-manual --git-provider git` fails initialization because operator profiles support only `gitea` and `gitlab`.
20. `--profile operator-manual --secret-provider none` fails initialization because operator profiles require an instantiated secret provider.
21. Operator profiles in kubernetes mode rewrite localhost component URLs to in-cluster endpoints (preferring component pod IP, then service ClusterIP/DNS) so the in-cluster manager can reach local providers.
22. Manual profiles with no component manual-info output omit the `Manual Component Access` section and still render handoff access sections deterministically.
23. Operator profile with `git-provider=git` does not configure provider webhooks and still fails fast from operator-profile provider validation.

## Examples
1. `./run-e2e.sh --profile cli-basic --repo-type filesystem --managed-server simple-api-server --secret-provider none` runs compatible smoke cases and reports deterministic summary.
2. `./run-e2e.sh --profile cli-manual --repo-type git --git-provider github --git-provider-connection remote` fails initialization because manual mode is local-instantiable only.
3. `./run-e2e.sh --profile cli-basic --repo-type git --git-provider gitlab --managed-server simple-api-server` runs dependency-aware parallel hooks while ensuring `repo-type:git` waits for `git-provider:*` initialization.
4. `./run-e2e.sh --managed-server keycloak --managed-server-auth-type none` fails selection because keycloak requires oauth2 auth-type support.
5. `./run-e2e.sh --profile cli-full --managed-server simple-api-server --managed-server-mtls true` validates runtime mTLS trust reload by removing and re-adding trusted client certificates without restarting the server.
6. `./run-e2e.sh --profile cli-basic --repo-type git --git-provider gitea --managed-server simple-api-server --secret-provider file` runs git main-case coverage against a local compose-backed Gitea provider.
7. `./run-e2e.sh --profile cli-basic --repo-type git --git-provider gitea --git-provider-connection remote --managed-server simple-api-server --secret-provider none` fails `Preparing Components` when required `DECLAREST_E2E_GITEA_*` remote credentials are missing.
8. `./run-e2e.sh --profile cli-manual --repo-type git --git-provider gitea --managed-server simple-api-server --secret-provider none` yields a handoff context where `declarest-e2e context check` and `declarest-e2e repository status` can run without `git repository not initialized`.
9. `./run-e2e.sh --validate-components` validates all discovered component manifests, hook scripts, dependency catalog, and managed-server fixture metadata, then exits without running test cases.
10. `./run-e2e.sh --profile cli-basic --managed-server keycloak` emits a context with `metadata.bundle: keycloak-bundle:0.0.1` (and no `metadata.base-dir`) so keycloak runs consume metadata from the default remote bundle source.
11. `./run-e2e.sh --profile cli-basic --managed-server simple-api-server --metadata-source dir` emits `metadata.base-dir` from `test/e2e/components/managed-server/simple-api-server/metadata` and keeps local `managed-server.http.openapi`.
12. `./run-e2e.sh --profile cli-basic --platform compose --repo-type git --git-provider gitea --managed-server simple-api-server --secret-provider file` runs local containerized components via compose artifacts under each selected component `compose/compose.yaml`.
13. `./run-e2e.sh --profile cli-manual --platform kubernetes --repo-type filesystem --managed-server keycloak --secret-provider file` starts a run-scoped kind cluster, prints kubeconfig/namespace details for manual interaction, and `./run-e2e.sh --clean <run-id>` deletes the run cluster.
14. `DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL=http://proxy.example:3128 ./run-e2e.sh --profile cli-basic --managed-server-proxy true` injects `managed-server.http.proxy.http-url` into the generated context.
15. `./run-e2e.sh --profile operator-manual --managed-server simple-api-server --git-provider gitea --secret-provider file` starts a run-scoped kind cluster, installs operator CRDs, deploys the operator manager in-cluster, applies generated CRs, and prints manual repository-to-managed-server verification commands.
16. `./run-e2e.sh --profile operator-basic --managed-server simple-api-server --git-provider gitea --secret-provider file` starts the operator stack and runs compatible shared smoke coverage plus automated operator reconcile coverage.
17. `./run-e2e.sh --profile operator-full --managed-server simple-api-server --git-provider gitea --secret-provider file` extends `operator-basic` with compatible shared main coverage plus corner-case validations.
18. `./run-e2e.sh --profile operator-manual --managed-server keycloak --repo-type git --git-provider gitea --secret-provider vault` prints `Manual Component Access` from component `manual-info` hooks in handoff output before `Repository provider access`.
19. `./run-e2e.sh --profile cli-basic --managed-server rundeck` emits `metadata.base-dir` from `test/e2e/components/managed-server/rundeck/metadata` because `rundeck` has no shorthand bundle mapping, while still leaving local `managed-server.http.openapi` unset.
20. `./run-e2e.sh --profile operator-manual --managed-server rundeck --repo-type git --git-provider gitea --secret-provider vault` prints `Manual Component Access` with rundeck URL and credentials via managed-server-state fallback even when no selected-component `manual-info` output is present.
21. `./run-e2e.sh --profile operator-manual --managed-server simple-api-server --git-provider gitea --secret-provider file` registers a gitea push webhook to the run-scoped operator service URL and handoff output prints that URL plus a `kubectl ... jsonpath` command for `declarest.io/webhook-last-received-at`.
22. `./run-e2e.sh --profile cli-basic --managed-server rundeck` prints summary parameter lines for omitted defaults such as `platform: kubernetes (default)` and `managed-server-auth-type: custom-header (component-default)`.
23. `./run-e2e.sh --profile operator-manual --managed-server simple-api-server --secret-provider file` prints summary parameter lines for operator profile defaults such as `repository-type: git (profile-default)` and `git-provider: gitea (profile-default)` when those flags are omitted.
24. A long-running step such as `Starting Components` in a TTY session updates the `SPAN` column live from values such as `0s` to `1s` to `2s` while the spinner remains active, then preserves the final duration once the step completes.
25. `./run-e2e.sh --profile cli-basic --managed-server simple-api-server --metadata-type base-dir` behaves the same as `--metadata-source dir`, and the final summary reports the canonical execution parameter label `metadata-source: dir`.
