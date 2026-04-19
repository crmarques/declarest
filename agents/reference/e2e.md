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
2. The harness MAY expose helper entrypoint `test/e2e/run-e2e-parallel.sh`; when used, it MUST accept one shell command per line from stdin or `--matrix-file`, launch those commands concurrently, write one log file per child command, and exit non-zero when any child command fails.
2. Supported profiles MUST be `cli-basic`, `cli-full`, `cli-manual`, `operator-manual`, `operator-basic`, and `operator-full`; default is `cli-basic`.
3. Runner platform selection MUST support `--platform <compose|kubernetes>` with default `kubernetes`.
3. Default component selections MUST come from component-declared `DEFAULT_SELECTIONS=base` entries rather than hard-coded component names.
4. `cli-basic` MUST run `smoke` cases only; `cli-full` MUST run `smoke` + `main` + `corner` cases; both run only requirement-compatible cases.
5. `cli-manual` MUST start selected local-instantiable components, generate temporary context config, generate shell setup/reset scripts for handoff, and skip automated cases.
6. `cli-manual` MUST seed the selected context repository directory with the selected managed-service `repo-template` tree.
7. `cli-manual` MUST reject remote-only connection selections during initialization with actionable validation output.
8. Runtime lifecycle MUST be profile-specific: `cli-basic`/`cli-full` use seven steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Running Test Cases`, `Finalizing`); `cli-manual` uses five steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`); `operator-manual` uses seven steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Installing Operator`, `Finalizing`); `operator-basic`/`operator-full` use eight steps in order (`Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Installing Operator`, `Running Test Cases`, `Finalizing`).
9. Step statuses MUST be `RUNNING`, `OK`, `FAIL`, `SKIP` and rendered as bracketed labels such as `[RUNNING]`, `[OK]`, `[FAILED]`, `[SKIP]`.
10. Non-TTY mode MUST emit deterministic plain logs; TTY mode MAY use live spinner/color output.
11. Step output MUST resemble a table framed by divider lines above and below the header row `STEP | ACTION | SPAN | STATUS`, print that header once per run, center each header label within its column, render spinner glyphs inside the `ACTION` column while a step is running, show the elapsed duration in `SPAN` for running steps and the final duration after completion, and center `STEP`, `SPAN`, and `STATUS` values for every row.
12. Final summary MUST include step outcomes, resolved execution parameters (including explicit selections and defaulted values), case counters, duration, context file path, and logs path.
13. Each component under `test/e2e/components/<type>/<name>/` MUST provide `component.env`, `scripts/init.sh`, `scripts/configure-auth.sh`, and `scripts/context.sh`.
14. Local containerized (`COMPONENT_RUNTIME_KIND=compose`) components MUST provide `compose/compose.yaml`, at least one `k8s/*.yaml` manifest, and `scripts/health.sh`.
15. `component.env` MUST declare `COMPONENT_CONTRACT_VERSION=1`, `COMPONENT_RUNTIME_KIND`, and `COMPONENT_DEPENDS_ON` explicitly.
16. Components that want to participate in runner defaults MUST declare `DEFAULT_SELECTIONS` tokens in `component.env`; runner code MUST NOT infer defaults from component names.
17. The runner MUST expose `--validate-components` to validate all discovered component contracts and managed-service fixture metadata, then short-circuit workload execution with deterministic summary output.
17. The runner MUST reject `--managed-service none` with actionable validation output because managed-service selection is mandatory for E2E runs.
18. The runner MUST execute component hooks through one generic hook orchestration path (`init`, `start`, `health`, `configure-auth`, `context`, `stop`) rather than per-component ad hoc branching.
19. Hook orchestration MUST be dependency-aware using `COMPONENT_DEPENDS_ON` and MUST run ready batches in parallel when no dependency edge blocks them.
20. Missing dependency targets and dependency cycles MUST fail initialization or hook execution with actionable output.
21. Component scripts MUST be ShellCheck-friendly Bash and publish generated runtime values through the component state file.
22. Resource-server components MUST ship fixture trees under `repo-template/` with collection metadata at `<logical-collection>/_/metadata.yaml` or `<logical-collection>/_/metadata.json` and resource payloads at `<logical-resource>/resource.json`; YAML SHOULD be the default for new fixture metadata files.
23. Resource-server fixture metadata MUST model API-facing identifiers via `resource.id` and `resource.alias` identity templates (for example, keycloak realms use `{{/realm}}`).
24. The runner `--validate-components` mode MUST reject managed-service fixture metadata files that omit `resource.id` or `resource.alias`.
25. The loader MUST expand intermediary `/_/` metadata placeholders into concrete collection targets before invoking `resource metadata set`.
26. Cases MUST define `CASE_ID`, `CASE_SCOPE`, `CASE_REQUIRES`, and `case_run`; they MAY declare `CASE_PROFILES` as a whitespace-separated subset of `cli operator`.
27. `smoke` scope MUST represent the curated fast suite used by `cli-basic` and `operator-basic`.
28. Cases default to `CASE_PROFILES=cli` unless `CASE_SCOPE=operator-main`, which defaults to `CASE_PROFILES=operator`.
29. Cases discovered for a profile family MUST be filtered by `CASE_PROFILES` before workload execution.
30. Missing requirements default to `SKIP`; they become `FAIL` when tied to explicitly requested capabilities/selections.
31. Runtime artifacts MUST be written under `test/e2e/.runs/<run-id>/` (logs, state, context, per-case workdirs).
32. When managed-service metadata is sourced from a component `metadata/` directory, the runner MUST copy it into a run-scoped workspace under `test/e2e/.runs/<run-id>/` before generating contexts so metadata-mutating cases never write into checked-in component fixtures.
33. `Preparing Runtime` MUST reuse cached E2E CLI and operator-manager binaries when their Go source inputs are unchanged; operator profiles MUST build the runtime manager image from that cached prebuilt Linux binary instead of recompiling inside the container build.
33. User-facing E2E env vars MUST use `DECLAREST_E2E_*`; container engine selection MUST support `podman` or `docker` via `DECLAREST_E2E_CONTAINER_ENGINE` (default `podman`).
34. The runner MUST maintain one live execution log file and print its path at startup.
35. Cleanup mode flags (`--clean`, `--clean-all`) MUST short-circuit workload execution, stop referenced runner processes, and remove execution artifacts plus run-recorded runtime resources associated with each run (`compose` projects or `kind` clusters), and they MUST also drop any run-specific `PATH` entries (for example `<run-dir>/bin`) that `cli-manual` or `operator-manual` handoff prepended so shells no longer reference cleaned runs.
36. Active local runs MUST reserve selected host ports for the lifetime of the run and release those reservations during finalize or cleanup so concurrent runs do not reuse the same pending port assignment before containers bind.
32. Components MAY implement optional `scripts/manual-info.sh`; in `cli-manual` and `operator-manual` profiles, the runner MUST execute this hook for selected components after startup and print aggregated hook output in a `Manual Component Access` handoff section before `Repository provider access`.
33. When the selected managed-service component has no `scripts/manual-info.sh` hook or it emits no output, manual profiles MUST print state-derived managed-service connection details in the same `Manual Component Access` section when that state is available.
34. Runner security selection flags MUST include `--managed-service-auth-type <none|basic|oauth2|custom-header|prompt>` and `--managed-service-mtls`; `--managed-service-mtls` defaults to `false`, auth type defaults MUST be elected by the selected managed-service component when the flag is omitted (preference order SHOULD be `oauth2`, then `custom-header`, then `basic`, then `none`), and `prompt` MUST emit top-level prompt-backed credentials plus `managedService.http.auth.basic.credentialsRef` for managed-service components that support `basic-auth`.
35. Runner proxy selection flags MUST include canonical shared proxy inputs `--proxy-mode <none|local|external>` and `--proxy-auth-type <none|basic|prompt>`; proxy default MUST be `none`.
36. For CLI profiles, `--proxy-mode external` MUST inject explicit proxy blocks into every eligible generated context section (`managedService.http`, `repository.git.remote` when the remote URL uses `http|https`, `secretStore.vault`, and `metadata` when bundle-backed metadata is downloaded remotely) using `DECLAREST_E2E_PROXY_*` values and requiring at least one configured proxy URL.
37. For CLI profiles, `--proxy-mode local` MUST auto-select helper component `proxy:forward-proxy`, provision one run-scoped proxy endpoint, and inject the same eligible context proxy blocks using canonical resolved proxy values only; omitted proxy auth type in local mode MUST default to `basic`, and when username/password are omitted the helper component MUST generate run-scoped credentials. When platform is `compose`, local proxy wiring MUST rewrite eligible localhost or `127.0.0.1` managed-service, git-remote, and Vault URLs in generated contexts to one host-reachable non-loopback address when that address can be resolved so host-side CLI traffic traverses the local proxy.
38. Proxy prompt auth is CLI-only in v1: `--proxy-auth-type prompt` MUST be rejected for non-`cli-manual` profiles before startup, MUST emit top-level prompt-backed credentials plus `*.proxy.auth.basic.credentialsRef` instead of inline proxy credentials in generated contexts, local prompt-backed proxy credentials MUST set `persistInSession: true` for both username and password, manual handoff output MUST print the generated proxy credentials for testing, and the generated setup script MUST NOT export proxy-auth bootstrap variables for that local prompt path.
39. `managed-service` components MUST declare security capabilities in `component.env`, including at least one auth-type capability token (`none|basic-auth|oauth2|custom-header`) and optional `mtls`; runner selection MUST fail when requested auth-type or mTLS features are unsupported or required features are disabled, and runner auth-type `prompt` MUST map to the same component capability requirement as `basic`.
40. When a managed-service component cannot support a selected auth-type or mTLS combination for the chosen connection mode, its hooks MUST fail with actionable output if selection validation did not already reject the combination.
41. `simple-api-server` mTLS trust MUST be reloaded from configured client-certificate sources for new connections without process restart.
42. `simple-api-server` mTLS mode MUST allow an empty trusted-certificate set and deny all client API access until trusted certificates are added.
43. `cli-manual` with `repo-type=git` MUST run `repository init` after context assembly so repository-dependent checks (`context check`, `repository status`) are immediately usable.
41. Runner metadata selection flags MUST include `--metadata-source <bundle|dir>`; default mode MUST be `bundle`.
42. In `bundle` mode, the runner MUST skip local `openapi.yaml` wiring so `managed-service.http.openapi` remains unset, and MUST use the selected managed-service component `METADATA_BUNDLE_REF` when declared.
43. In `dir` mode, managed-service components MAY ship a sibling `metadata/` directory; when present, the runner MUST set `E2E_METADATA_DIR` to that component-local directory and repository-type context fragments MUST emit `metadata.baseDir` using `E2E_METADATA_DIR` (fallbacking to the repo base dir when unset).
44. In `bundle` mode, when the selected managed-service omits `METADATA_BUNDLE_REF`, the runner MUST fall back to the component-local `metadata/` directory as `metadata.baseDir` when present; otherwise it MUST continue without setting `metadata.bundle`.
45. Kubernetes runtime MUST use run-scoped `kind` clusters when platform is `kubernetes` and at least one local containerized component is selected; it MUST persist runtime state (`platform`, `container engine`, `cluster name`, `namespace`, `kubeconfig`) for cleanup/manual handoff.
46. Kubernetes image preload MUST deduplicate identical image references within one run and SHOULD reuse shared exported archives under `.e2e-build/k8s-image-cache/` when the local image ID is unchanged; each run MUST still load its required archives into the selected kind cluster.
46. Kubernetes component startup MUST apply rendered `k8s/*.yaml` manifests in the run namespace and manage service port-forwards from `declarest.e2e/port-forward` service annotations, persisting forward PIDs in component state for stop/cleanup.
47. For `DECLAREST_E2E_CONTAINER_ENGINE=podman`, kind operations MUST use provider mode `KIND_EXPERIMENTAL_PROVIDER=podman` and preflight MUST fail fast with actionable guidance when provider checks fail.
48. `operator-manual`, `operator-basic`, and `operator-full` MUST enforce kubernetes-only local-instantiable selections (`--platform kubernetes`, operator-default `repo-type`, operator-default webhook-capable `git-provider`, non-`none` secret provider, and local connections for selected components) and MUST fail initialization with actionable validation output when unsupported combinations are selected.
49. Operator profiles MUST install the operator via full OLM rather than direct CRD/Deployment apply: the runner MUST build a run-scoped operator image, generate a run-scoped bundle workspace from `bundle/` with the CSV patched only for e2e runtime needs (manager image, `--watch-namespace=<run namespace>`, webhook definitions targeting the OLM-managed Deployment, PVC-backed state volumes replaced by `emptyDir`, and managed-service metadata bundle mounts when present), MUST preserve `webhookdefinitions`, `--enable-admission-webhooks=true`, `--leader-elect`, and the repository-webhook bind address, build run-scoped bundle and file-based catalog images from that workspace, load all three images into the run-scoped kind cluster, install OLM core by applying vendored `test/e2e/olm/v0.42.0/crds.yaml` and `test/e2e/olm/v0.42.0/olm.yaml` when OLM is absent, remove the upstream default `operatorhubio-catalog` from local e2e clusters so resolution uses only the generated test catalog, apply the run-scoped `CatalogSource` first and wait for it to report `READY`, then apply the run-scoped `OperatorGroup` (SingleNamespace mode targeting the run namespace) and `Subscription` that together drive `CSV` install to `Succeeded` before CRs are applied.
50. Operator profiles MUST seed selected managed-service fixture content into the repository, initialize git, commit/push seeded content to the selected git provider, and generate/apply `ResourceRepository`, `ManagedService`, `SecretStore`, and `SyncPolicy` CRs in the run namespace after the OLM-managed operator `Deployment` is ready.
51. `operator-manual` handoff output MUST include run-scoped setup/reset shell scripts, operator runtime details (including OLM subscription and CSV names), and concrete repository-to-managed-service verification commands using component-declared `OPERATOR_EXAMPLE_RESOURCE_PATH` and `OPERATOR_EXAMPLE_RESOURCE_PAYLOAD` values when available.
52. `operator-basic` MUST execute `smoke` plus `operator-main` scope automated cases after operator installation; `operator-full` MUST execute compatible `smoke`, compatible `main`, `operator-main`, and `corner` scope automated cases.
53. Operator readiness waits (`DECLAREST_E2E_OPERATOR_READY_TIMEOUT_SECONDS`) MUST default to `120`, reject non-positive values, and cap at `600`; the same timeout MUST gate both `Subscription`/`CSV` install progression and operator `Deployment` readiness.
54. Operator profiles with git providers that declare `REPOSITORY_WEBHOOK_PROVIDER` MUST precompute run-scoped repository webhook URL/secret values, configure provider webhooks during access setup, and emit `spec.git.webhook` configuration in generated `ResourceRepository` CRs.
55. Operator profile CSV patches MUST preserve a repository-webhook service endpoint (port `8082`, selector matching the OLM-installed operator pods) and the manager container MUST receive `--repository-webhook-bind-address` so repository webhook receipts reach the reconciler; OLM cleanup MUST tear down the run-scoped generated `Subscription`, `CSV`, `OperatorGroup`, and `CatalogSource` during `--clean`/`--clean-all` or normal finalization against reused kind clusters, while leaving shared OLM core installed.

## Data Contracts
Runner flags:
1. Workload: `--profile`.
2. Platform: `--platform`.
2. Component selection: `--managed-service`, `--metadata-source`, `--repo-type`, `--git-provider`, `--secret-provider`.
3. Resource-server and proxy security selection: `--managed-service-auth-type`, `--managed-service-mtls`, `--proxy-mode`, `--proxy-auth-type`.
4. Connection selection: `--managed-service-connection`, `--git-provider-connection`, `--secret-provider-connection`.
5. Runtime controls: `--list-components`, `--validate-components`, `--keep-runtime`, `--verbose`.
6. Cleanup controls: `--clean`, `--clean-all`.

Parallel helper flags:
1. Input source: `--matrix-file`.
2. Output control: `--log-dir`.

`component.env` fields:
1. `COMPONENT_TYPE`, `COMPONENT_NAME`.
2. `SUPPORTED_CONNECTIONS`, `DEFAULT_CONNECTION`.
3. `DEFAULT_SELECTIONS` (optional): whitespace-separated subset of `base operator`.
4. `REQUIRES_DOCKER`.
5. `COMPONENT_CONTRACT_VERSION` (current supported value `1`).
6. `COMPONENT_RUNTIME_KIND` (`native|compose`).
7. `COMPONENT_DEPENDS_ON` (space-separated dependency selectors using `<type>:<name>` or `<type>:*`).
8. `DESCRIPTION`.
9. `SUPPORTED_SECURITY_FEATURES` (`managed-service` only): whitespace-separated subset of `none basic-auth oauth2 custom-header mtls`, including at least one auth-type capability token.
10. `REQUIRED_SECURITY_FEATURES` (`managed-service` optional): whitespace-separated subset of `SUPPORTED_SECURITY_FEATURES`, with at most one auth-type capability token.
11. `COMPONENT_SERVICE_PORT` (optional): service port used for generic in-cluster URL rewriting.
12. `METADATA_BUNDLE_REF` (`managed-service` only, optional): bundle ref used by `--metadata-source bundle`.
13. `OPERATOR_EXAMPLE_RESOURCE_PATH` and `OPERATOR_EXAMPLE_RESOURCE_PAYLOAD` (`managed-service` only, optional): paired operator handoff example resource.
14. `REPOSITORY_WEBHOOK_PROVIDER` (`git-provider` only, optional): webhook provider token used by operator webhook configuration.
15. `REPO_PROVIDER_LOGIN_PATH` (`git-provider` only, optional): login path appended to `REPO_PROVIDER_BASE_URL` for manual handoff output.

Optional component hook:
1. `scripts/manual-info.sh` may emit plain-text access details for `cli-manual` and `operator-manual` profile output; when emitted, runner output groups details in a `Manual Component Access` handoff section before `Repository provider access`.
2. When the selected managed-service hook is absent or empty, the runner MAY derive fallback access details from the managed-service component state file and print them in the same section.
3. `scripts/start.sh` and `scripts/stop.sh` may override built-in compose runtime lifecycle adapters.
4. Built-in adapters are platform-aware: compose (`compose/compose.yaml`) or kubernetes (`k8s/*.yaml` + service annotation-driven port-forward).
5. Successful `init` and `configure-auth` hooks MUST leave `${E2E_COMPONENT_STATE_FILE}` non-empty; successful `context` hooks for components that own persisted context sections (`managed-service`, `repo-type`, `secret-provider`) MUST leave `${E2E_COMPONENT_CONTEXT_FRAGMENT}` non-empty.
6. `scripts/prepare-repo-template.sh` MAY adjust the copied managed-service `repo-template/` tree after the generic copy step and before git initialization.

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
2. Emit setup/reset shell script paths; setup script MUST export runtime vars, define alias `declarest-e2e` to the run-local binary, and initialize prompt-auth shell-session reuse for prompt-backed credentials by evaluating the bash session hook against that binary, while reset script MUST unset those vars, remove the alias, and restore the prior prompt-auth shell state.
3. Print concrete follow-up `declarest-e2e` commands.
4. Exit after startup and keep runtime resources available until explicit `--clean`/`--clean-all`.
5. When platform is `kubernetes`, print cluster access details (`cluster`, `namespace`, `kubeconfig`) and example `kubectl` commands.
6. When selected components emit manual-info details, print those details in `Manual Component Access` before `Repository provider access`.

Operator handoff:
1. Emit temporary context catalog path and run-scoped setup/reset shell scripts.
2. Print operator runtime details (`manager-deployment`, `manager-pod`, `manager-logs`, namespace, sync-policy name).
3. Print concrete `declarest-e2e` commands to save a repository resource, commit/push it, and read the same logical path from the managed service.
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
9. Operator profile applies the run-scoped `Subscription` before the `CatalogSource` `grpc` pod reports ready, so `Subscription` resolution stalls with no actionable log pointer.
10. Operator profile leaves the OLM-generated `CSV`, `Subscription`, `OperatorGroup`, or `CatalogSource` behind after `--clean`, leaking cluster state into subsequent runs.

## Edge Cases
1. `--list-components` short-circuits runtime startup but still yields deterministic summary.
2. `cli-manual` with explicit subset flags starts only selected local-instantiable components.
3. `cli-full` runs validation/auth corner cases without marking run unstable when assertions pass.
4. Remote-capable selections with missing env credentials fail fast with guidance.
5. Nested fixture metadata patterns (for example `/x/_/y/_/_`) expand deterministically into concrete collection targets.
6. Cleanup for unknown run ids returns actionable output and still attempts runner/runtime teardown using recorded runtime state (fallback compose naming when state is missing).
7. Dependency selector wildcard (for example `git-provider:*`) resolves exactly one selected provider in some runs and multiple in others without changing correctness.
8. Local `simple-api-server` with `ENABLE_MTLS=true` generates or consumes mounted cert material from a host directory and fails fast when required TLS files are missing.
9. Selecting `--managed-service-auth-type` unsupported by the chosen managed-service component fails fast before startup with actionable validation output.
10. Local `simple-api-server` mTLS trust directory can transition from non-empty to empty and back during runtime; API access follows the current trust set for new connections.
11. `cli-manual` with `repo-type=git` still initializes a local git repository so readiness checks do not fail with `git repository not initialized`.
12. Cleanup while a manual profile shell is still sourced MUST drop the `<run-dir>/bin` PATH insertion so the shell no longer resolves the deleted `declarest-e2e` alias or binary.
13. `managed-service=keycloak` runs in `bundle` mode fail during context validation when shorthand bundle `keycloak-bundle:0.0.1` cannot be resolved from the default remote path.
14. `bundle` mode with a managed-service that has no shorthand mapping falls back to component-local `metadata/` when present, otherwise continues without `metadata.bundle`; in both cases local `openapi.yaml` remains unset.
15. Metadata-mutating E2E cases (for example `resource metadata set` or `secret detect --fix`) write only into the run-scoped metadata workspace copy and MUST leave checked-in component metadata directories unchanged.
16. `--metadata-source` MUST reject any value outside `bundle|dir` before runtime startup.
17. `--platform kubernetes` with only remote/native selections MUST not create a kind cluster.
18. `--proxy-mode external` without `DECLAREST_E2E_PROXY_HTTP_URL` or `DECLAREST_E2E_PROXY_HTTPS_URL` fails argument validation before runtime startup.
19. `--managed-service-auth-type prompt` with a managed-service component that lacks `basic-auth` capability fails selection validation before startup, while a basic-auth-capable component emits a prompt-auth context block without inline managed-service credentials.
20. `--proxy-mode local --proxy-auth-type prompt` in `cli-manual` emits top-level prompt-backed proxy credentials with `persistInSession: true` plus proxy `basic.credentialsRef` placeholders, prints the generated proxy credentials in manual handoff output, keeps those proxy auth values out of the generated setup-script environment, and rejects simultaneous inline `DECLAREST_E2E_PROXY_AUTH_USERNAME` or `DECLAREST_E2E_PROXY_AUTH_PASSWORD` inputs before runtime startup.
21. `--platform compose --proxy-mode local` with local managed-service, git-provider, or Vault URLs on `localhost|127.0.0.1` rewrites those generated context URLs to one host-reachable non-loopback address so explicit proxy configuration is exercised; when no such host address can be resolved, the runner leaves the original loopback URLs unchanged.
22. `--profile operator-manual --git-provider git` fails initialization because operator profiles support only `gitea` and `gitlab`.
23. `--profile operator-manual --secret-provider none` fails initialization because operator profiles require an instantiated secret provider.
24. Operator profiles in kubernetes mode rewrite localhost component URLs to in-cluster endpoints (preferring component pod IP, then service ClusterIP/DNS) so the in-cluster manager can reach local providers.
25. Manual profiles with no component manual-info output omit the `Manual Component Access` section and still render handoff access sections deterministically.
26. Operator profile with `git-provider=git` does not configure provider webhooks and still fails fast from operator-profile provider validation.
27. `run-e2e-parallel.sh` returns non-zero when any one child run fails, even if the other child runs succeed.
28. Repeated operator runs against an unchanged tree reuse the cached Linux manager binary and MUST NOT re-enter in-container module download or source rebuild paths during `Preparing Runtime`.
29. Repeated kubernetes runs that reference the same unchanged local image reuse the shared exported archive while still loading that archive into each new run-scoped kind cluster.

## Examples
1. `./run-e2e.sh --profile cli-basic --repo-type filesystem --managed-service simple-api-server --secret-provider none` runs compatible smoke cases and reports deterministic summary.
2. `./run-e2e.sh --profile cli-manual --repo-type git --git-provider github --git-provider-connection remote` fails initialization because manual mode is local-instantiable only.
3. `./run-e2e.sh --profile cli-basic --repo-type git --git-provider gitlab --managed-service simple-api-server` runs dependency-aware parallel hooks while ensuring `repo-type:git` waits for `git-provider:*` initialization.
4. `./run-e2e.sh --managed-service keycloak --managed-service-auth-type none` fails selection because keycloak requires oauth2 auth-type support.
5. `./run-e2e.sh --profile cli-full --managed-service simple-api-server --managed-service-mtls true` validates runtime mTLS trust reload by removing and re-adding trusted client certificates without restarting the server.
6. `./run-e2e.sh --profile cli-basic --repo-type git --git-provider gitea --managed-service simple-api-server --secret-provider file` runs git main-case coverage against a local compose-backed Gitea provider.
7. `./run-e2e.sh --profile cli-basic --repo-type git --git-provider gitea --git-provider-connection remote --managed-service simple-api-server --secret-provider none` fails `Preparing Components` when required `DECLAREST_E2E_GITEA_*` remote credentials are missing.
8. `./run-e2e.sh --profile cli-manual --repo-type git --git-provider gitea --managed-service simple-api-server --secret-provider none` yields a handoff context where `declarest-e2e context check` and `declarest-e2e repository status` can run without `git repository not initialized`.
9. `./run-e2e.sh --validate-components` validates all discovered component manifests, hook scripts, dependency catalog, and managed-service fixture metadata, then exits without running test cases.
10. `./run-e2e.sh --profile cli-basic --managed-service keycloak` emits a context with `metadata.bundle: keycloak-bundle:0.0.1` (and no `metadata.baseDir`) so keycloak runs consume metadata from the default remote bundle source.
11. `./run-e2e.sh --profile cli-basic --managed-service simple-api-server --metadata-source dir` emits `metadata.baseDir` from `test/e2e/components/managed-service/simple-api-server/metadata` and keeps local `managed-service.http.openapi`.
12. `./run-e2e.sh --profile cli-basic --platform compose --repo-type git --git-provider gitea --managed-service simple-api-server --secret-provider file` runs local containerized components via compose artifacts under each selected component `compose/compose.yaml`.
13. `./run-e2e.sh --profile cli-manual --platform kubernetes --repo-type filesystem --managed-service keycloak --secret-provider file` starts a run-scoped kind cluster, prints kubeconfig/namespace details for manual interaction, and `./run-e2e.sh --clean <run-id>` deletes the run cluster.
14. `DECLAREST_E2E_PROXY_HTTP_URL=http://proxy.example:3128 ./run-e2e.sh --profile cli-basic --proxy-mode external` injects shared proxy blocks into eligible CLI context sections without changing operator-profile wiring.
15. `./run-e2e.sh --profile cli-manual --managed-service simple-api-server --managed-service-auth-type prompt` emits top-level prompt-backed credentials plus `managedService.http.auth.basic.credentialsRef` while the local component state still keeps basic-auth credentials available for manual handoff details.
16. `./run-e2e.sh --profile cli-manual --platform compose --proxy-mode local --proxy-auth-type prompt` injects top-level prompt-backed proxy credentials plus proxy `basic.credentialsRef` placeholders, rewrites local managed-service and other eligible loopback URLs to one host-reachable non-loopback address when available so the proxy is exercised, keeps inline proxy credentials out of `contexts.yaml`, prints the generated proxy credentials in manual handoff output, and leaves proxy auth bootstrap env vars unset in the generated setup script.
17. `./run-e2e.sh --profile operator-manual --managed-service simple-api-server --git-provider gitea --secret-provider file` starts a run-scoped kind cluster, installs OLM core from the vendored v0.42.0 YAML manifests, builds run-scoped operator/bundle/catalog images, applies and waits on a run-scoped `CatalogSource`, applies a run-scoped `OperatorGroup`/`Subscription`, waits for the `CSV` to report `Succeeded` and the Deployment to become available, applies generated CRs, and prints manual repository-to-managed-service verification commands.
18. `./run-e2e.sh --profile operator-basic --managed-service simple-api-server --git-provider gitea --secret-provider file` starts the operator stack and runs compatible shared smoke coverage plus automated operator reconcile coverage.
19. `./run-e2e.sh --profile operator-full --managed-service simple-api-server --git-provider gitea --secret-provider file` extends `operator-basic` with compatible shared main coverage plus corner-case validations.
20. `./run-e2e.sh --profile operator-manual --managed-service keycloak --repo-type git --git-provider gitea --secret-provider vault` prints `Manual Component Access` from component `manual-info` hooks in handoff output before `Repository provider access`.
21. `./run-e2e.sh --profile cli-basic --managed-service rundeck` emits `metadata.baseDir` from `test/e2e/components/managed-service/rundeck/metadata` because `rundeck` has no shorthand bundle mapping, while still leaving local `managed-service.http.openapi` unset.
22. `./run-e2e.sh --profile operator-manual --managed-service rundeck --repo-type git --git-provider gitea --secret-provider vault` prints `Manual Component Access` with rundeck URL and credentials via managed-service-state fallback even when no selected-component `manual-info` output is present.
23. `./run-e2e.sh --profile operator-manual --managed-service simple-api-server --git-provider gitea --secret-provider file` registers a gitea push webhook to the run-scoped operator service URL and handoff output prints that URL plus a `kubectl ... jsonpath` command for `declarest.io/webhook-last-received-at`.
24. `./run-e2e.sh --profile cli-basic --managed-service rundeck` prints summary parameter lines for omitted defaults such as `platform: kubernetes (default)` and `managed-service-auth-type: custom-header (component-default)`.
25. `./run-e2e.sh --profile operator-manual --managed-service simple-api-server --secret-provider file` prints summary parameter lines for operator profile defaults such as `repository-type: git (profile-default)` and `git-provider: gitea (profile-default)` when those flags are omitted.
26. A long-running step such as `Starting Components` in a TTY session updates the `SPAN` column live from values such as `0s` to `1s` to `2s` while the spinner remains active, then preserves the final duration once the step completes.
27. `./run-e2e.sh --profile cli-basic --managed-service simple-api-server --metadata-source nope` fails argument validation before runtime startup with the allowed metadata-source values.
28. `./test/e2e/run-e2e-parallel.sh <<'EOF' ... EOF` runs a pasted command matrix concurrently, writes one job log per line under `test/e2e/.runs/parallel-<id>/`, and exits `1` when any listed `run-e2e.sh` command fails.
29. Running `./run-e2e.sh --profile operator-basic --managed-service keycloak` twice on an unchanged tree reuses the cached Linux operator-manager binary for `Preparing Runtime` and only rebuilds the runtime image wrapper layer.
30. Two kubernetes runs that both reference `docker.io/rundeck/rundeck:5.20.0` reuse the shared `.e2e-build/k8s-image-cache/docker.io_rundeck_rundeck_5.20.0.tar` export while still issuing one `kind load image-archive ...` per run-scoped cluster.
31. `./run-e2e.sh --profile cli-basic --managed-service haproxy` emits `metadata.baseDir` from `test/e2e/components/managed-service/haproxy/metadata` (mirroring the checked-in `declarest-bundle-haproxy/metadata/` tree) and leaves local `managed-service.http.openapi` unset because `haproxy` ships no resolvable bundle shorthand.
32. `./run-e2e.sh --profile operator-basic --managed-service haproxy --git-provider gitea --secret-provider file` drives the same OLM install path as the other managed-services and reconciles haproxy `sites` resources through the OLM-managed operator.
