# E2E Test Runner

This repository uses a componentized Bash e2e harness.

## Entrypoint

```bash
./run-e2e.sh --profile basic
```

## Profiles

- `basic` (default): runs all `main` cases that match the selected stack.
- `full`: runs all `main` and `corner` cases that match the selected stack.
- `manual`: starts local-instantiable components, writes a temporary context catalog, and exits after startup.
  - with no component flags, it uses the same component defaults as other profiles
  - remote component selections are rejected in Step 1
  - runtime resources are kept; clean them with `./run-e2e.sh --clean <run-id>` or `./run-e2e.sh --clean-all`

## Main Flags

- `--profile <basic|full|manual>`
- `--resource-server <keycloak|vault|rundeck|none>`
- `--resource-server-connection <local|remote>`
- `--repo-type <filesystem|git>`
- `--git-provider <git|gitlab|github>`
- `--git-provider-connection <local|remote>`
- `--secret-provider <file|vault|none>`
- `--secret-provider-connection <local|remote>`
- `--list-components`
- `--keep-runtime`
- `--verbose`
- `--clean <run-id>`
- `--clean-all`

Cleanup behavior:

- `--clean <run-id>` stops the referenced `e2e/run-e2e.sh` process (when running), tears down local compose projects for that run id, and removes `e2e/.runs/<run-id>`.
- `--clean-all` stops all running `e2e/run-e2e.sh` processes and applies the same cleanup to every run directory under `e2e/.runs/`.

## Runtime Environment Variables

- `DECLAREST_E2E_CONTAINER_ENGINE`: container CLI used for local compose startup (`podman` or `docker`, default: `podman`)
- `DECLAREST_E2E_EXECUTION_LOG`: optional path for the live execution log file (default: `e2e/.runs/<run-id>/execution.log`)

## Runtime Steps

The runner reports progress in grouped steps:

- `basic`/`full`: 7 steps

1. `Initializing`
2. `Preparing Runtime`
3. `Preparing Components`
4. `Starting Components`
5. `Configuring Access`
6. `Running Workload`
7. `Finalizing`

- `manual`: 5 steps

1. `Initializing`
2. `Preparing Runtime`
3. `Preparing Components`
4. `Starting Components`
5. `Configuring Access`

TTY mode renders dynamic spinner/status updates. Non-TTY mode emits structured plain step lines.
The runner prints a live log pointer at startup so progress can be followed with `tail -f`.

## Case Model

Cases live under:

- `e2e/cases/main/*.sh`
- `e2e/cases/corner/*.sh`
- `e2e/components/<type>/<name>/cases/main/*.sh` (component-scoped main cases)
- `e2e/components/<type>/<name>/cases/corner/*.sh` (component-scoped corner cases)

Case discovery order per scope:

1. global catalog (`e2e/cases/<scope>`)
2. selected component catalogs (`e2e/components/<type>/<name>/cases/<scope>`)

Each case file must define:

- `CASE_ID`
- `CASE_SCOPE`
- `CASE_REQUIRES` (space-separated requirements)
- `case_run` function

Requirement format:

- selector requirement: `repo-type=git`
- capability requirement: `has-secret-provider`

Missing requirement behavior:

- default: case is `SKIP` with the missing requirement list
- if a missing requirement was explicitly requested by flags, the case is marked `FAIL`

## Component Contract

Each component directory under `e2e/components/<type>/<name>/` must contain:

- `component.env`
- `scripts/init.sh`
- `scripts/configure-auth.sh`
- `scripts/context.sh`

Resource-server components must also provide a fixture tree used by sync-oriented cases:

- `repo-template/**`

For the `keycloak` resource-server, the runner connects directly to Keycloak Admin REST (`/admin/*`) and does not use an auxiliary adapter API.

Fixture tree rules:

- tree layout must match the repository format exactly.
- collection metadata must be stored at `<logical-collection>/_/metadata.json`.
- resource payloads must be stored at `<logical-resource>/resource.json`.
- metadata paths may be nested (for example `/admin/realms/_/organizations/_`) to avoid duplicated metadata files.
- when metadata paths include intermediary `/_/`, the e2e loader expands them to concrete collection metadata paths from template resources before calling `declarest metadata set`.
- metadata must represent API-specific identifiers using `idFromAttribute` and `aliasFromAttribute` (for example, keycloak realms use `realm`, not internal `id`).

Keycloak repo-template currently covers:

- `/admin/realms`
- `/admin/realms/_/clients`
- `/admin/realms/_/identity-provider/instances`
- `/admin/realms/_/organizations`

Container-backed local components should also provide:

- `compose.yaml`
- `scripts/health.sh`

## Remote Environment Variables

### Resource Server (`keycloak`, remote)

- `DECLAREST_E2E_RESOURCE_SERVER_BASE_URL`
- `DECLAREST_E2E_KEYCLOAK_TOKEN_URL`
- `DECLAREST_E2E_KEYCLOAK_CLIENT_ID`
- `DECLAREST_E2E_KEYCLOAK_CLIENT_SECRET`
- optional: `DECLAREST_E2E_KEYCLOAK_SCOPE`, `DECLAREST_E2E_KEYCLOAK_AUDIENCE`

### Resource Server (`vault`, remote)

- `DECLAREST_E2E_RESOURCE_SERVER_BASE_URL`
- `DECLAREST_E2E_RESOURCE_SERVER_TOKEN`
- optional: `DECLAREST_E2E_RESOURCE_SERVER_VAULT_MOUNT`, `DECLAREST_E2E_RESOURCE_SERVER_VAULT_PATH_PREFIX`, `DECLAREST_E2E_RESOURCE_SERVER_VAULT_KV_VERSION`

### Resource Server (`rundeck`, remote)

- `DECLAREST_E2E_RESOURCE_SERVER_BASE_URL`
- `DECLAREST_E2E_RESOURCE_SERVER_TOKEN`
- optional: `DECLAREST_E2E_RESOURCE_SERVER_RUNDECK_API_VERSION`, `DECLAREST_E2E_RESOURCE_SERVER_RUNDECK_AUTH_HEADER`

### Git Provider (`gitlab`, remote)

- `DECLAREST_E2E_GITLAB_REMOTE_URL`
- `DECLAREST_E2E_GITLAB_TOKEN`
- optional: `DECLAREST_E2E_GITLAB_REMOTE_BRANCH`

### Git Provider (`github`, remote)

- `DECLAREST_E2E_GITHUB_REMOTE_URL`
- `DECLAREST_E2E_GITHUB_TOKEN`
- optional: `DECLAREST_E2E_GITHUB_REMOTE_BRANCH`

### Secret Provider (`vault`, remote)

- `DECLAREST_E2E_VAULT_ADDRESS`
- optional: `DECLAREST_E2E_VAULT_MOUNT`, `DECLAREST_E2E_VAULT_PATH_PREFIX`, `DECLAREST_E2E_VAULT_KV_VERSION`
- auth one-of:
  - token: `DECLAREST_E2E_VAULT_TOKEN`
  - userpass: `DECLAREST_E2E_VAULT_USERNAME`, `DECLAREST_E2E_VAULT_PASSWORD`, optional `DECLAREST_E2E_VAULT_AUTH_MOUNT`
  - approle: `DECLAREST_E2E_VAULT_ROLE_ID`, `DECLAREST_E2E_VAULT_SECRET_ID`, optional `DECLAREST_E2E_VAULT_AUTH_MOUNT`

Legacy `E2E_*` remote env vars are still accepted temporarily with a warning.

## Artifacts

Each run writes artifacts under `e2e/.runs/<run-id>/`:

- `logs/`
- `state/`
- `contexts.yaml`
- `cases/`
