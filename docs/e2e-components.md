# E2E Component Standard

## Orchestrator
`tests/run-tests.sh` is the orchestration surface for the E2E harness. It parses the CLI, sets `DECLAREST_MANAGED_SERVER`, `DECLAREST_REPO_PROVIDER`, `DECLAREST_SECRET_PROVIDER`, and the `DECLAREST_SKIP_TESTING_*` toggles, then loads the requested managed server’s `run-e2e.sh`. The script defines fixed execution groups in `GROUP_DEFINITIONS` (prepare workspace/services, configure services/context, each test category, variations, finish) and `run_group` coordinates those functions. When `DECLAREST_E2E_PROFILE` is `reduced`, the orchestrator disables the metadata, OpenAPI, and variation groups.

Component selection is resolved by `tests/scripts/components.sh`, which expects a `component.sh` in each component directory and exposes `load_*_component` helpers.

## Component layout
- Managed server: `tests/managed-server/<name>/component.sh`
- Repo provider: `tests/repo-provider/<name>/component.sh`
- Secret provider: `tests/secret-provider/<name>/component.sh`

## Managed server contract
Each managed server component must define:
- `MANAGED_SERVER_NAME`
- `managed_server_default_repo_provider()`
- `managed_server_default_secret_provider()`
- `managed_server_validate <repo_provider> <secret_provider>`
- `managed_server_runner <mode>` returning the runner path (`run-e2e.sh` or `run-interactive.sh`)

`run-e2e.sh` must implement the group functions listed in `tests/run-tests.sh` (`run_preparing_workspace`, `run_preparing_services`, `run_configuring_services`, `run_configuring_context`, `run_testing_context_operations`, `run_testing_metadata_operations`, `run_testing_openapi_operations`, `run_testing_declarest_main_flows`, `run_testing_secret_check_metadata`, `run_testing_variation_flows`, `run_finishing_execution`). Set `should_run_*` flags so the orchestrator can skip groups. Direct invocation should still work via `script_invoked_directly`.

Variation flows should remain lightweight and should not run when `DECLAREST_E2E_PROFILE` is `reduced`.

## Repo provider contract
Each repo provider component must define:
- `REPO_PROVIDER_NAME`
- `REPO_PROVIDER_TYPE` (one of `fs`, `git-local`, `git-remote`)
- `REPO_PROVIDER_PRIMARY_AUTH` (empty if none)
- `REPO_PROVIDER_SECONDARY_AUTH` (bash array, empty if none)
- `REPO_PROVIDER_REMOTE_PROVIDER` (empty for non-remote)
- `REPO_PROVIDER_INTERACTIVE_AUTH` (`1` when interactive runs must configure auth)
- `repo_provider_apply_env`
- `repo_provider_prepare_services`
- `repo_provider_prepare_interactive`
- `repo_provider_configure_auth <auth_type>`

Components can reuse `tests/repo-provider/lib/component.sh` to configure auth from the generic `DECLAREST_REPO_*` environment variables.

## Secret provider contract
Each secret provider component must define:
- `SECRET_PROVIDER_NAME`
- `SECRET_PROVIDER_PRIMARY_AUTH`
- `SECRET_PROVIDER_SECONDARY_AUTH` (bash array, empty if none)
- `secret_provider_apply_env`
- `secret_provider_prepare_services`
- `secret_provider_configure_auth <auth_type>`

## Adding a new component
1. Add `component.sh` under the group directory and implement the required variables/functions.
2. For managed servers, implement the group hooks in `run-e2e.sh` and expose a runner for `run-tests.sh`.
3. For repo/secret providers, make sure `repo_provider_apply_env`/`secret_provider_apply_env` set the environment expected by the managed server scripts and any setup scripts.
4. Update this file with any extra environment variables or setup steps required by the new component.
