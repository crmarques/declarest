# E2E Component Standard

## Orchestrating from one place
`tests/run-tests.sh` is now the single orchestration surface for the E2E harness. It parses the CLI, sets `DECLAREST_MANAGED_SERVER`, `DECLAREST_REPO_PROVIDER`, `DECLAREST_SECRET_PROVIDER`, and the `DECLAREST_SKIP_TESTING_*` toggles, then loads the requested managed server’s `run-e2e.sh`. The script defines a fixed set of execution groups stored in `GROUP_DEFINITIONS` (preparing workspace/services, configuring services/context, each test category, variation, etc.) and the `run_group` helper watches the `should_run_*` flags to skip or run the appropriate functions.

When you add a new managed server, `run-tests.sh` simply sources its `run-e2e.sh`, calls an optional `managed_server_bootstrap` hook, and then runs the numbered groups in order. That makes the orchestration extensible: every managed server component just has to implement the standard hooks and any shared helpers (like `run_step`) that the orchestrator assumes exist.

## Managed server component contract
1. **Location:** place the component under `tests/managed-server/<name>`. The orchestrator always sources `run-e2e.sh` from that directory.
2. **Exports:** when sourced, the script must declare:
   - `managed_server_bootstrap()` (optional but expected). Use it to initialize secrets, call providers, or log container/runtime metadata; the orchestrator invokes it once before the grouped steps, and direct runs should call it as well.
   - One function per group in `GROUP_DEFINITIONS`:
     * `run_preparing_workspace`
     * `run_preparing_services`
     * `run_configuring_services`
     * `run_configuring_context`
     * `run_testing_context_operations`
     * `run_testing_metadata_operations`
     * `run_testing_openapi_operations`
     * `run_testing_declarest_main_flows`
     * `run_testing_secret_check_metadata`
   * `run_testing_variation_flows`
   * `run_finishing_execution`
  Each function should call `run_step` (or a equivalent helper) and respect the `should_run_*` helper variables to know when the orchestrator intends to skip work.

  The variation group should only run when `DECLAREST_E2E_PROFILE=complete`; it focuses on lightweight smoke checks for server auth, secret auth, repo auth, and TLS so that `--reduced` stays fast while `run_testing_declarest_main_flows` continues to cover the full workflow once.
3. **Flags:** compute and export the `should_run_*` variables (`should_run_context`, `should_run_metadata`, `should_run_openapi`, `should_run_declarest`, `should_run_variation`) so the orchestrator’s skip logic can make decisions. Components can also respect the `DECLAREST_SKIP_TESTING_*` environment variables if needed.
4. **Direct tooling:** guard the direct runway with `script_invoked_directly` so running `tests/managed-server/<name>/run-e2e.sh` still works. That script can reuse the same functions that the orchestrator calls.
5. **Logging and tokens:** load the shared libs under `scripts/lib/` (e.g., logging, CLI helpers, GitHub auth) and keep `log_line`, `run_step`, and other helpers around so the orchestrator’s status table and logs stay consistent.

## Repo provider contract
Repository providers live under `tests/repo-provider/<name>` and are consumed through the `REPO_SCRIPTS_DIR`/`PROVIDER_SCRIPTS_DIR` environment variables that managed servers can assemble from `DECLAREST_TESTS_ROOT`. To align with the existing harness:
- Provide `prepare.sh` to populate the workspace/repository.
- Provide `strip-secrets.sh`, `verify.sh`, `auth-smoke.sh`, and `print-log.sh` when applicable so managed servers can sanitize or validate credentials.
- Support auxiliary scripts such as `setup.sh` under provider-specific subfolders (e.g., `gitlab/setup.sh`) when extra service preparation is needed. Managed servers can source the provider’s `env` files (via `source $PROVIDER_ENV_FILE`) just like the Keycloak harness does.
- Prefer reusing `tests/repo-provider/common` helpers for shared logic (authentication, logging, CLI helpers) so new providers follow the same API.

## Secret provider contract
Secret providers live in `tests/secret-provider/<name>` and should expose at least `setup.sh` (and any supporting libs under `lib/`). The managed server harness is responsible for invoking the correct `setup.sh` based on `DECLAREST_SECRET_PROVIDER`. The script should:
- Exit cleanly when the wrong provider type is selected so the harness can run multiple providers without per-run changes.
- Populate credentials or environment files (e.g., writing to `$DECLAREST_WORK_DIR/vault.env`) so the managed server can source them afterwards.
- Declare any required environment variables (address, tokens, mount points) so helper scripts under `scripts/` have what they need.

## Adding a new component
1. **Managed server:** copy an existing harness under `tests/managed-server/<name>` and implement the required functions described above. Make sure the direct-invocation guard still works if you want standalone runs.
2. **Repo provider:** add `tests/repo-provider/<name>` with the standard scripts and libs. Update the managed server harness to point `REPO_SCRIPTS_DIR` to the new provider and call its helpers during workspace preparation, service configuration, and variation testing as needed.
3. **Secret provider:** add `tests/secret-provider/<name>/setup.sh` and any libs. Ensure the managed server harness runs the setup script during the appropriate group (e.g., `run_preparing_services` or `run_configuring_services`) and that the script exports the expected variables for downstream helpers.
4. **Documentation:** update this file and any component README with the new component’s purpose and any required environment variables. The orchestrator will automatically pick up the component as soon as a matching `run-e2e.sh` and/or provider folder exists.

With this standard in place, every new component simply exports the shared functions the orchestrator expects; no edits to `tests/run-tests.sh` are needed to support new managed servers, repo providers, or secret providers.
