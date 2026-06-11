# E2E Harness and Component Contracts

## Purpose
Define the Bash E2E harness contract: profiles, runner flags, component contracts, case filtering, runtime lifecycle/summary, and manual/operator handoff.

## Scope
E2E orchestration only. Underlying contracts are owned elsewhere and enforced here at the harness boundary: context YAML proxy/credentialsRef/`${ENV_VAR}` schema -> context-config.md; `metadata.bundle` ref forms and resolution -> context-config.md + metadata-bundle.md; OLM bundle/catalog/CSV shape -> k8s-operator.md; identity-template (`resource.id`/`resource.alias`) and required-attribute semantics -> metadata.md + domain.md.

## Normative Rules

### Entrypoints and profiles
1. The harness MUST expose repository entrypoint `run-e2e.sh` delegating to `test/e2e/run-e2e.sh`.
2. The harness MAY expose `test/e2e/run-e2e-parallel.sh`; when used it MUST accept one shell command per line from stdin or `--matrix-file`, run them concurrently, write one log file per child, and exit non-zero when any child fails.
3. Supported profiles MUST be `cli-basic`, `cli-full`, `cli-manual`, `operator-manual`, `operator-basic`, `operator-full`; default MUST be `cli-basic`.
4. `--platform <compose|kubernetes>` MUST default to `kubernetes`.
5. Default component selections MUST come from component-declared `DEFAULT_SELECTIONS` tokens; runner code MUST NOT infer defaults from component names.

### Profile behavior
6. `cli-basic` MUST run `smoke` cases only; `cli-full` MUST run `smoke`+`main`+`corner`; both run only requirement-compatible cases. Validation/auth corner cases that pass MUST NOT mark the run unstable.
7. `cli-manual` MUST: start selected local-instantiable components; seed the selected context repository directory with the selected managed-service `repo-template` tree; generate temporary context config plus setup/reset handoff scripts; skip automated cases; reject remote-only connection selections at init with actionable output.
8. `cli-manual` with `repo-type=git` MUST run `repository init` after context assembly so `context check` and `repository status` are immediately usable.
9. `operator-basic` MUST run `smoke`+`operator-main` automated cases after operator install; `operator-full` MUST run compatible `smoke`+compatible `main`+`operator-main`+`corner`.

### Runtime lifecycle and output
10. Lifecycle steps MUST run in this profile-specific order:
    - `cli-basic`/`cli-full` (7): `Initializing`, `Preparing Runtime`, `Preparing Components`, `Starting Components`, `Configuring Access`, `Running Test Cases`, `Finalizing`.
    - `cli-manual` (5): the first five above (through `Configuring Access`).
    - `operator-manual` (7): the first five, then `Installing Operator`, `Finalizing`.
    - `operator-basic`/`operator-full` (8): the first five, then `Installing Operator`, `Running Test Cases`, `Finalizing`.
11. Step statuses MUST be `RUNNING`, `OK`, `FAIL`, `SKIP`, rendered as bracketed labels (`[RUNNING]`, `[OK]`, `[FAILED]`, `[SKIP]`).
12. Non-TTY mode MUST emit deterministic plain logs; TTY mode MAY use live spinner/color output.
13. Step output MUST render as a divider-framed table with header `STEP | ACTION | SPAN | STATUS` printed once per run; header labels centered per column; spinner glyphs in `ACTION` while a step runs; `SPAN` showing live elapsed duration during a step and the final duration after; `STEP`, `SPAN`, `STATUS` values centered for every row.
14. The runner MUST maintain one live execution log file and print its path at startup.
15. The final summary MUST include step outcomes, resolved execution parameters (explicit selections and defaulted values, annotated e.g. `platform: kubernetes (default)`, `managed-service-auth-type: custom-header (component-default)`, `git-provider: gitea (profile-default)`), case counters, duration, context file path, and logs path.

### Component contracts
16. Each component under `test/e2e/components/<type>/<name>/` MUST provide `component.env`, `scripts/init.sh`, `scripts/configure-auth.sh`, `scripts/context.sh`.
17. `COMPONENT_RUNTIME_KIND=compose` components MUST additionally provide `compose/compose.yaml`, at least one `k8s/*.yaml` manifest, and `scripts/health.sh`.
18. `component.env` MUST declare `COMPONENT_CONTRACT_VERSION=1`, `COMPONENT_RUNTIME_KIND`, and `COMPONENT_DEPENDS_ON` explicitly; components participating in runner defaults MUST declare `DEFAULT_SELECTIONS`.
19. Component scripts MUST be ShellCheck-friendly Bash and publish generated runtime values through the component state file.
20. Successful `init` and `configure-auth` hooks MUST leave `${E2E_COMPONENT_STATE_FILE}` non-empty; successful `context` hooks for components owning persisted context sections (`managed-service`, `repo-type`, `secret-provider`) MUST leave `${E2E_COMPONENT_CONTEXT_FRAGMENT}` non-empty.
21. Resource-server components MUST ship fixture trees under `repo-template/` with collection metadata at `<logical-collection>/_/metadata.{yaml|json}` and resource payloads at `<logical-resource>/resource.json`; YAML SHOULD be the default for new fixture metadata. Fixture metadata MUST model API-facing identifiers via `resource.id` and `resource.alias` identity templates (e.g. keycloak realms use `{{/realm}}`).
22. `managed-service` components MUST declare `SUPPORTED_SECURITY_FEATURES` with at least one auth-type token (`none|basic-auth|oauth2|custom-header`) and optional `mtls` in `component.env`.

### Hook orchestration
23. The runner MUST drive component hooks through one generic path (`init`, `start`, `health`, `configure-auth`, `context`, `stop`), never per-component ad hoc branching.
24. Hook orchestration MUST be dependency-aware via `COMPONENT_DEPENDS_ON` and MUST run ready batches in parallel when no dependency edge blocks them. Missing dependency targets and dependency cycles MUST fail init or hook execution with actionable output.
25. The loader MUST expand intermediary `/_/` metadata placeholders into concrete collection targets before invoking `resource metadata set`.

### Cases
26. Cases MUST define `CASE_ID`, `CASE_SCOPE`, `CASE_REQUIRES`, `case_run`; they MAY declare `CASE_PROFILES` as a whitespace-separated subset of `cli operator`. `CASE_PROFILES` defaults to `cli`, except `CASE_SCOPE=operator-main` defaults to `operator`.
27. `smoke` scope MUST be the curated fast suite used by `cli-basic` and `operator-basic`.
28. Cases MUST be filtered by `CASE_PROFILES` against the active profile family before execution.
29. `CASE_REQUIRES` entries are `key=value` selectors or symbolic capabilities (e.g. `has-secret-provider`), AND-combined. Missing requirements default to `SKIP`; they become `FAIL` only when tied to explicitly requested capabilities/selections.
30. Case discovery MUST be deterministic and deduplicated, ordered global cases (`test/e2e/cases/<scope>/`) before selected-component cases (`test/e2e/components/<type>/<name>/cases/<scope>/`).

### Validation and selection
31. `--validate-components` MUST validate all discovered component contracts, hook scripts, dependency catalog, and managed-service fixture metadata, then short-circuit workload execution with deterministic summary output. It MUST reject fixture metadata files that omit `resource.id` or `resource.alias`. `--list-components` MUST likewise short-circuit runtime startup and still yield a deterministic summary.
32. The runner MUST reject `--managed-service none` with actionable output (managed-service selection is mandatory).
33. Runner security selection MUST fail when requested auth-type or mTLS features are unsupported or required features are disabled for the chosen connection mode; auth-type `prompt` MUST map to the same component capability requirement as `basic`. When selection validation does not already reject an unsupported auth-type/mTLS combination, the component hooks MUST fail with actionable output.

### Runtime artifacts and env
34. Runtime artifacts MUST be written under `test/e2e/.runs/<run-id>/` (logs, state, context, per-case workdirs).
35. When managed-service metadata is sourced from a component `metadata/` directory, the runner MUST copy it into a run-scoped workspace under `test/e2e/.runs/<run-id>/` before generating contexts, so metadata-mutating cases never write into checked-in component fixtures.
36. `Preparing Runtime` MUST reuse cached E2E CLI and operator-manager binaries when Go source inputs are unchanged; operator profiles MUST build the runtime manager image from that cached prebuilt Linux binary, never recompiling inside the container build.
37. User-facing E2E env vars MUST use the `DECLAREST_E2E_*` prefix; container engine selection MUST support `podman` or `docker` via `DECLAREST_E2E_CONTAINER_ENGINE` (default `podman`).
38. Active local runs MUST reserve selected host ports for the run lifetime and release them at finalize/cleanup so concurrent runs do not reuse a pending port assignment before containers bind.

### Cleanup
39. `--clean`/`--clean-all` MUST short-circuit workload execution, stop referenced runner processes, remove execution artifacts plus run-recorded runtime resources (`compose` projects or `kind` clusters), and drop any run-specific `PATH` entry (e.g. `<run-dir>/bin`) that manual handoff prepended. Cleanup for an unknown run id MUST still return actionable output and attempt runner/runtime teardown from recorded runtime state, falling back to compose naming when state is missing.
40. Operator-profile OLM cleanup MUST tear down the run-scoped generated `Subscription`, `CSV`, `OperatorGroup`, and `CatalogSource` during `--clean`/`--clean-all` or normal finalization against reused kind clusters, while leaving shared OLM core installed.

### Security: managed-service auth and mTLS
41. `--managed-service-auth-type <none|basic|oauth2|custom-header|prompt>` and `--managed-service-mtls` (default `false`) MUST be supported. When auth-type is omitted, the selected managed-service component MUST elect its default (preference order SHOULD be `oauth2`, `custom-header`, `basic`, `none`). `prompt` MUST emit top-level prompt-backed credentials plus `managedService.http.auth.basic.credentialsRef` for components supporting `basic-auth`.
42. `simple-api-server` mTLS trust MUST reload from configured client-certificate sources for new connections without process restart, MUST allow an empty trusted-certificate set, and MUST deny all client API access until trusted certificates are added. When mTLS is enabled it MUST provision or consume mounted server/client certificate material from the configured host directory and fail fast when required TLS files are missing.

### Security: proxy (CLI profiles)
43. `--proxy-mode <none|local|external>` (default `none`) and `--proxy-auth-type <none|basic|prompt>` MUST be supported. Generated context proxy blocks use the proxy schema and `credentialsRef` rules owned by context-config.md.
44. `--proxy-mode external` MUST inject explicit proxy blocks into every eligible generated context section (`managedService.http`, `repository.git.remote` when the remote URL is `http|https`, `secretStore.vault`, and `metadata` when bundle-backed metadata is downloaded remotely) using `DECLAREST_E2E_PROXY_*` values, and MUST require at least one configured proxy URL.
45. `--proxy-mode local` MUST auto-select helper `proxy:forward-proxy`, provision one run-scoped proxy endpoint, and inject the same eligible blocks using canonical resolved values only; omitted auth type MUST default to `basic`, and the helper MUST generate run-scoped credentials when username/password are omitted. On `--platform compose`, local proxy wiring MUST rewrite eligible `localhost`/`127.0.0.1` managed-service, git-remote, and Vault URLs to one host-reachable non-loopback address when resolvable, so host-side CLI traffic traverses the proxy.
46. `--proxy-auth-type prompt` is CLI-only in v1: it MUST be rejected for non-`cli-manual` profiles before startup; MUST emit top-level prompt-backed credentials plus `*.proxy.auth.basic.credentialsRef` instead of inline proxy credentials; local prompt-backed proxy credentials MUST set `persistInSession: true` for username and password; manual handoff MUST print the generated proxy credentials; and the setup script MUST NOT export proxy-auth bootstrap variables for that local prompt path.

### Metadata source
47. `--metadata-source <bundle|dir>` MUST default to `bundle` and MUST reject any other value before startup.
48. In `bundle` mode the runner MUST skip local `openapi.yaml` wiring (`managedService.http.openapi` stays unset) and MUST use the selected component `METADATA_BUNDLE_REF` when declared. For an `oci://` ref, the runner MUST skip local cache seeding and let the declarest resolver pull the OCI artifact at first use; for any other form (shorthand, URL, local path), the runner MAY pre-seed the local cache from the peer bundles repo at `E2E_METADATA_BUNDLES_ROOT` for offline/deterministic runs. When `METADATA_BUNDLE_REF` is absent, the runner MUST fall back to the component-local `metadata/` directory as `metadata.baseDir` when present, otherwise continue without setting `metadata.bundle`.
49. In `dir` mode, when a managed-service component ships a sibling `metadata/` directory the runner MUST set `E2E_METADATA_DIR` to it and repository-type context fragments MUST emit `metadata.baseDir` from `E2E_METADATA_DIR` (falling back to the repo base dir when unset).

### Kubernetes runtime
50. On `--platform kubernetes` with at least one local containerized component, the runner MUST use a run-scoped `kind` cluster and persist runtime state (`platform`, container engine, `cluster name`, `namespace`, `kubeconfig`) for cleanup/handoff; selections with only remote/native components MUST NOT create a kind cluster.
51. Kubernetes image preload MUST deduplicate identical image references within a run and SHOULD reuse shared exported archives under `.e2e-build/k8s-image-cache/` when the local image ID is unchanged; each run MUST still load its required archives into its kind cluster.
52. Kubernetes component startup MUST apply rendered `k8s/*.yaml` manifests in the run namespace and manage port-forwards from `declarest.e2e/port-forward` service annotations, persisting forward PIDs in component state.
53. For `DECLAREST_E2E_CONTAINER_ENGINE=podman`, kind operations MUST set `KIND_EXPERIMENTAL_PROVIDER=podman` and preflight MUST fail fast with actionable guidance when provider checks fail.

### Operator profiles
54. `operator-manual`, `operator-basic`, `operator-full` MUST enforce kubernetes-only local-instantiable selections (`--platform kubernetes`, operator-default `repo-type`, operator-default webhook-capable `git-provider`, non-`none` secret provider, local connections for selected components) and MUST fail init with actionable output on unsupported combinations.
55. Operator profiles MUST install via full OLM (not direct CRD/Deployment apply): build a run-scoped operator image; generate a run-scoped bundle workspace from `bundle/` with the CSV patched only for e2e runtime needs (manager image, `--watch-namespace=<run namespace>`, webhook definitions targeting the OLM-managed Deployment, PVC-backed state volumes replaced by `emptyDir`, managed-service metadata bundle mounts when present) while preserving `webhookdefinitions`, `--enable-admission-webhooks=true`, `--leader-elect`, and the repository-webhook bind address; build run-scoped bundle and file-based catalog images; load all three images into the run-scoped kind cluster; install OLM core from vendored `test/e2e/olm/v0.42.0/crds.yaml` and `olm.yaml` when OLM is absent; remove the upstream default `operatorhubio-catalog` so resolution uses only the generated catalog; apply the run-scoped `CatalogSource` first and wait for it to report `READY`; then apply the run-scoped `OperatorGroup` (SingleNamespace targeting the run namespace) and `Subscription`, driving `CSV` install to `Succeeded` before any CRs are applied.
56. After the OLM-managed Deployment is ready, operator profiles MUST seed selected managed-service fixture content into the repository, init git, commit/push it to the selected git provider, and generate/apply `ResourceRepository`, `ManagedService`, `SecretStore`, `SyncPolicy` CRs in the run namespace.
57. Operator profiles with git providers declaring `REPOSITORY_WEBHOOK_PROVIDER` MUST precompute run-scoped repository webhook URL/secret values, configure provider webhooks during access setup, and emit `spec.git.webhook` in generated `ResourceRepository` CRs.
58. Operator CSV patches MUST preserve a repository-webhook service endpoint (port `8082`, selector matching the OLM-installed operator pods) and the manager container MUST receive `--repository-webhook-bind-address` so webhook receipts reach the reconciler.
59. Operator readiness timeout `DECLAREST_E2E_OPERATOR_READY_TIMEOUT_SECONDS` MUST default to `120`, reject non-positive values, cap at `600`, and gate both `Subscription`/`CSV` install progression and operator `Deployment` readiness.
60. Operator profiles in kubernetes mode MUST rewrite localhost component URLs to in-cluster endpoints (preferring component pod IP, then service ClusterIP/DNS) so the in-cluster manager reaches local providers.

### Handoff (manual and operator)
61. Manual/operator handoff MUST emit the temporary context catalog path and run-scoped setup/reset shell scripts, then exit after startup keeping runtime resources available until explicit `--clean`/`--clean-all`. The setup script MUST export runtime vars, define alias `declarest-e2e` to the run-local binary, and initialize prompt-auth shell-session reuse by evaluating the bash session hook against that binary; the reset script MUST unset those vars, remove the alias, and restore prior prompt-auth state.
62. Handoff MUST print concrete follow-up `declarest-e2e` commands. When platform is `kubernetes`, it MUST print cluster access details (`cluster`, `namespace`, `kubeconfig`) and example `kubectl` commands.
63. Components MAY implement optional `scripts/manual-info.sh`; in manual profiles the runner MUST execute it for selected components after startup and print aggregated output in a `Manual Component Access` section before `Repository provider access`. When the selected managed-service component has no such hook or it emits nothing, manual profiles MUST print state-derived managed-service connection details in that same section when state is available; with no manual-info at all, the section is omitted and other handoff sections still render deterministically.
64. `operator-manual` handoff MUST additionally print operator runtime details (`manager-deployment`, `manager-pod`, `manager-logs`, namespace, sync-policy name, OLM subscription and CSV names), repository-webhook runtime details (`repository-webhook-url`), one `kubectl` command to inspect webhook receipt annotations on the generated `ResourceRepository`, and concrete `declarest-e2e` commands to save a repository resource, commit/push it, and read the same logical path from the managed service (using component-declared `OPERATOR_EXAMPLE_RESOURCE_PATH`/`OPERATOR_EXAMPLE_RESOURCE_PAYLOAD` when available).

## Data Contracts

Runner flags (`run-e2e.sh`):
1. Workload: `--profile`. Platform: `--platform`.
2. Component selection: `--managed-service`, `--metadata-source`, `--repo-type`, `--git-provider`, `--secret-provider`.
3. Security selection: `--managed-service-auth-type`, `--managed-service-mtls`, `--proxy-mode`, `--proxy-auth-type`.
4. Connection selection: `--managed-service-connection`, `--git-provider-connection`, `--secret-provider-connection`.
5. Runtime controls: `--list-components`, `--validate-components`, `--keep-runtime`, `--verbose`.
6. Cleanup controls: `--clean`, `--clean-all`.

Parallel helper flags (`run-e2e-parallel.sh`): `--matrix-file` (input source), `--log-dir` (output control).

`component.env` fields:
1. `COMPONENT_TYPE`, `COMPONENT_NAME`, `DESCRIPTION`.
2. `SUPPORTED_CONNECTIONS`, `DEFAULT_CONNECTION`.
3. `DEFAULT_SELECTIONS` (optional): whitespace-separated subset of `base operator`.
4. `REQUIRES_DOCKER`, `COMPONENT_CONTRACT_VERSION` (current `1`), `COMPONENT_RUNTIME_KIND` (`native|compose`).
5. `COMPONENT_DEPENDS_ON` (space-separated `<type>:<name>` or `<type>:*` selectors).
6. `SUPPORTED_SECURITY_FEATURES` (`managed-service` only): subset of `none basic-auth oauth2 custom-header mtls`, with at least one auth-type token.
7. `REQUIRED_SECURITY_FEATURES` (`managed-service`, optional): subset of `SUPPORTED_SECURITY_FEATURES`, at most one auth-type token.
8. `COMPONENT_SERVICE_PORT` (optional): service port for generic in-cluster URL rewriting.
9. `METADATA_BUNDLE_REF` (`managed-service`, optional): bundle ref for `--metadata-source bundle`.
10. `OPERATOR_EXAMPLE_RESOURCE_PATH` + `OPERATOR_EXAMPLE_RESOURCE_PAYLOAD` (`managed-service`, optional): paired operator handoff example.
11. `REPOSITORY_WEBHOOK_PROVIDER` (`git-provider`, optional): webhook provider token for operator webhook config.
12. `REPO_PROVIDER_LOGIN_PATH` (`git-provider`, optional): login path appended to `REPO_PROVIDER_BASE_URL` for handoff output.

Optional component hooks:
1. `scripts/manual-info.sh`: plain-text access details for manual profiles (see Rule 63).
2. `scripts/start.sh` / `scripts/stop.sh`: override built-in compose runtime lifecycle adapters. Built-in adapters are platform-aware: compose (`compose/compose.yaml`) or kubernetes (`k8s/*.yaml` + annotation-driven port-forward).
3. `scripts/prepare-repo-template.sh`: MAY adjust the copied managed-service `repo-template/` tree after the generic copy and before git init.

## Failure Modes
1. `cli-manual` accepts unsupported remote selections.
2. Component hook failures are swallowed; execution continues with invalid state.
3. Partial startup passes without health checks.
4. Requirement filtering hides explicitly requested mandatory coverage.
5. Summary omits actionable failing-step log pointers.
6. Dependency selectors reference non-discovered components (fail late) or form cycles (deadlock startup).
7. Remote-capable connection selections (e.g. `--git-provider-connection remote`) with missing `DECLAREST_E2E_*` credentials MUST fail fast at `Preparing Components` with actionable guidance rather than proceeding with invalid state.
8. Operator profile leaves repository webhook URL/secret unset, so git provider hooks are not registered and reconcile falls back to poll interval.
9. Operator profile applies the run-scoped `Subscription` before the `CatalogSource` grpc pod is ready, so resolution stalls without an actionable log pointer.
10. Operator profile leaks the OLM-generated `CSV`/`Subscription`/`OperatorGroup`/`CatalogSource` after `--clean` into subsequent runs.

## Examples
1. `./run-e2e.sh --profile cli-basic --repo-type filesystem --managed-service simple-api-server --secret-provider none` runs compatible smoke cases and a deterministic summary.
2. `./run-e2e.sh --profile cli-manual --repo-type git --git-provider github --git-provider-connection remote` fails init: manual mode is local-instantiable only.
3. `./run-e2e.sh --profile cli-basic --repo-type git --git-provider gitlab --managed-service simple-api-server` runs dependency-aware parallel hooks while `repo-type:git` waits for `git-provider:*` init.
4. `./run-e2e.sh --managed-service keycloak --managed-service-auth-type none` fails selection: keycloak requires oauth2.
5. `./run-e2e.sh --profile cli-full --managed-service simple-api-server --managed-service-mtls true` validates runtime mTLS trust reload by removing/re-adding trusted client certs without restart.
6. `./run-e2e.sh --profile cli-manual --repo-type git --git-provider gitea --managed-service simple-api-server --secret-provider none` yields a handoff context where `context check` and `repository status` run without `git repository not initialized`.
7. `./run-e2e.sh --validate-components` validates all manifests, hooks, dependency catalog, and fixture metadata, then exits without running cases; metadata missing `resource.id`/`resource.alias` is rejected.
8. `./run-e2e.sh --profile cli-basic --managed-service keycloak` emits `metadata.bundle: oci://ghcr.io/crmarques/declarest-metadata-bundles/keycloak:0.0.1` with no `metadata.baseDir`, pulling metadata from the OCI artifact at first use; context validation fails if that ref cannot be pulled. `--metadata-source dir` falls back to the peer `declarest-metadata-bundles/bundles/<component>/` tree (overridable via `E2E_METADATA_BUNDLES_ROOT`); local `managedService.http.openapi` stays unset in both modes.
9. `./run-e2e.sh --profile cli-basic --managed-service simple-api-server --metadata-source dir` emits `metadata.baseDir` from the component `metadata/` directory and keeps local `managedService.http.openapi`. `--metadata-source nope` fails argument validation before startup, listing allowed values.
10. `DECLAREST_E2E_PROXY_HTTP_URL=http://proxy.example:3128 ./run-e2e.sh --profile cli-basic --proxy-mode external` injects shared proxy blocks into eligible CLI context sections; omitting both `*_PROXY_HTTP_URL`/`*_PROXY_HTTPS_URL` fails argument validation before startup.
11. `./run-e2e.sh --profile cli-manual --platform compose --proxy-mode local --proxy-auth-type prompt` injects top-level prompt-backed proxy credentials plus proxy `basic.credentialsRef` placeholders, rewrites eligible loopback URLs to one host-reachable non-loopback address when available, keeps inline proxy credentials out of `contexts.yaml`, prints generated proxy credentials in handoff, and leaves proxy auth bootstrap env vars unset in the setup script; simultaneous inline `DECLAREST_E2E_PROXY_AUTH_USERNAME`/`_PASSWORD` is rejected before startup.
12. `./run-e2e.sh --profile cli-manual --managed-service simple-api-server --managed-service-auth-type prompt` emits top-level prompt-backed credentials plus `managedService.http.auth.basic.credentialsRef` while component state retains basic-auth credentials for handoff details.
13. `./run-e2e.sh --profile cli-manual --platform kubernetes --repo-type filesystem --managed-service keycloak --secret-provider file` starts a run-scoped kind cluster and prints kubeconfig/namespace details; `./run-e2e.sh --clean <run-id>` deletes the run cluster and drops the `<run-dir>/bin` PATH insertion so a still-sourced shell no longer resolves the deleted alias/binary.
14. `./run-e2e.sh --profile operator-manual --managed-service simple-api-server --git-provider gitea --secret-provider file` starts a kind cluster, installs OLM core from vendored v0.42.0 YAML, builds run-scoped operator/bundle/catalog images, applies and waits on the `CatalogSource`, applies `OperatorGroup`/`Subscription`, waits for `CSV` `Succeeded` and Deployment availability, applies generated CRs, registers a gitea push webhook to the run-scoped operator service URL, and prints that URL plus a `kubectl ... jsonpath` command for `declarest.io/webhook-last-received-at`.
15. `./run-e2e.sh --profile operator-basic --managed-service haproxy --git-provider gitea --secret-provider file` drives the same OLM install path and reconciles haproxy `sites` resources through the OLM-managed operator (`operator-full` extends it with compatible main coverage plus corner-case validations).
16. `./run-e2e.sh --profile operator-manual --managed-service rundeck --repo-type git --git-provider gitea --secret-provider vault` prints `Manual Component Access` (rundeck URL and credentials via managed-service-state fallback when no `manual-info` output is present) before `Repository provider access`; `--git-provider git` would instead fail operator-profile provider validation, and `--secret-provider none` would fail init.
17. `./test/e2e/run-e2e-parallel.sh <<'EOF' ... EOF` runs a pasted command matrix concurrently, writes one job log per line under `test/e2e/.runs/parallel-<id>/`, and exits `1` when any listed run fails.
18. Repeated `operator-basic` runs on an unchanged tree reuse the cached Linux operator-manager binary in `Preparing Runtime` (only the runtime image wrapper layer rebuilds, no in-container module download/source rebuild); two kubernetes runs referencing the same image reuse the shared `.e2e-build/k8s-image-cache/*.tar` export while still issuing one `kind load image-archive` per run-scoped cluster.
