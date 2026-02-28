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
  - default stack includes `resource-server=simple-api-server`
  - remote component selections are rejected in Step 1
  - when a resource-server is selected, its `repo-template` tree is copied into the context repository directory
  - when `repo-type=git`, the runner initializes the local git repository before handoff so `config check`/`repo status` are immediately usable
  - component manual access details are printed after Step 5 (Configuring Access) when available
  - creates `declarest-e2e-env.sh` and `declarest-e2e-env-reset.sh` under `test/e2e/.runs/<run-id>/`; source setup script to export runtime vars and define alias `declarest-e2e`
  - simple-api-server local oauth2 defaults: client-id `declarest-e2e-client`; client secret is generated per run unless overridden with `DECLAREST_E2E_SIMPLE_API_CLIENT_SECRET`
  - simple-api-server local mTLS defaults: disabled; when enabled, cert material is generated under `test/e2e/.runs/<run-id>/certs/resource-server-simple-api-server` and mounted to `/etc/simple-api-server/certs`
  - simple-api-server mTLS trusted client certs are loaded from the mounted cert directory for new connections without restart; an empty trusted-cert directory denies all client API access
  - runtime resources are kept; clean them with `./run-e2e.sh --clean <run-id>` or `./run-e2e.sh --clean-all`

## Main Flags

- `--profile <basic|full|manual>`
- `--resource-server <name>` (mandatory; `none` is not supported)
- `--resource-server-connection <local|remote>`
- `--resource-server-auth-type <none|basic|oauth2|custom-header>` (default: component-elected)
- `--resource-server-mtls [<true|false>]` (default: `false`)
- `--metadata <bundle|local-dir>` (default: `bundle`)
- `--repo-type <name>`
- `--git-provider <name>`
- `--git-provider-connection <local|remote>`
- `--secret-provider <name|none>`
- `--secret-provider-connection <local|remote>`
- `--list-components`
- `--validate-components`
- `--keep-runtime`
- `--verbose`
- `--clean <run-id>`
- `--clean-all`

Use `--list-components` to see currently available component names and metadata.
Use `--validate-components` to run plugin/component contract validation (manifest fields, hook script syntax, dependency catalog, and resource-server fixture metadata rules) and exit without running test cases.
When `--resource-server-auth-type` is omitted, the selected resource-server component elects a default auth type (preferring `oauth2`, then `custom-header`, then `basic`, then `none`) that matches its capability contract.
Selections are validated against each resource-server capability contract; unsupported auth-type or mTLS combinations fail before startup.
`--metadata bundle` uses shorthand `metadata.bundle` mappings for supported resource-server components (currently `keycloak-bundle:0.0.1` for `keycloak`) and skips local `openapi.yaml` wiring so `resource-server.http.openapi` stays unset.
`--metadata local-dir` uses the selected resource-server component `metadata/` directory (when present) as `metadata.base-dir` and keeps normal local `openapi.yaml` wiring.

Cleanup behavior:

-- `--clean <run-id>` stops the referenced `test/e2e/run-e2e.sh` process (when running), tears down local compose projects recorded for that run id, and removes `test/e2e/.runs/<run-id>`.
-- `--clean-all` stops all running `test/e2e/run-e2e.sh` processes and applies the same cleanup to every run directory under `test/e2e/.runs/`.

Both cleanup modes also drop any `<run-id>/bin` entries that were prepended to `PATH` when a manual profile exported runtime variables, so the shell no longer retains stale run-specific directories.

## Runtime Environment Variables

- `DECLAREST_E2E_CONTAINER_ENGINE`: container CLI used for local compose startup (`podman` or `docker`, default: `podman`)
- `DECLAREST_E2E_EXECUTION_LOG`: optional path for the live execution log file (default: `test/e2e/.runs/<run-id>/execution.log`)

## Runtime Steps

The runner reports progress in grouped steps:

- `basic`/`full`: 7 steps

1. `Initializing`
2. `Preparing Runtime`
3. `Preparing Components`
4. `Starting Components`
5. `Configuring Access`
6. `Running Test Cases`
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

-- `test/e2e/cases/main/*.sh`
-- `test/e2e/cases/corner/*.sh`
-- `test/e2e/components/<type>/<name>/cases/main/*.sh` (component-scoped main cases)
-- `test/e2e/components/<type>/<name>/cases/corner/*.sh` (component-scoped corner cases)

Case discovery order per scope:

1. global catalog (`test/e2e/cases/<scope>`)
2. selected component catalogs (`test/e2e/components/<type>/<name>/cases/<scope>`)

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

Component authoring is contract-driven. Use `test/e2e/components/STANDARD.md` as the canonical onboarding guide.

Each component directory under `test/e2e/components/<type>/<name>/` must include:

- `component.env` with `COMPONENT_CONTRACT_VERSION=1`, explicit `COMPONENT_RUNTIME_KIND`, and explicit `COMPONENT_DEPENDS_ON`
- `resource-server` components must also declare `SUPPORTED_SECURITY_FEATURES` and may declare `REQUIRED_SECURITY_FEATURES`
- `scripts/init.sh`
- `scripts/configure-auth.sh`
- `scripts/context.sh`

Compose-runtime components must also include:

- `compose.yaml`
- `scripts/health.sh`

Optional hooks:

- `scripts/manual-info.sh`: printed after `Configuring Access` in `manual` profile
- `scripts/start.sh` and `scripts/stop.sh`: override built-in compose start/stop behavior when needed

Hook orchestration:

- `run-e2e.sh` is the single orchestrator entrypoint.
- Hook execution is dependency-aware by `COMPONENT_DEPENDS_ON`.
- Components in the same ready batch run in parallel for `init`, `start`, `health`, `configure-auth`, and `context`.

Fast validation and harness tests:

- `./test/e2e/run-e2e.sh --validate-components` validates all discovered component contracts and fixture metadata.
- `./test/e2e/tests/run.sh` runs fast Bash contract tests for the E2E harness libraries (args/cases/hooks/ui/cleanup/validation).

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

## Remote Environment Variables

### Resource Server (`simple-api-server`, remote)

- `DECLAREST_E2E_RESOURCE_SERVER_BASE_URL`
- optional toggles: `DECLAREST_E2E_SIMPLE_API_ENABLE_BASIC_AUTH`, `DECLAREST_E2E_SIMPLE_API_ENABLE_OAUTH2`, `DECLAREST_E2E_SIMPLE_API_ENABLE_MTLS`
  - defaults come from runner selection flags: `--resource-server-auth-type`, `--resource-server-mtls`
- when basic-auth is enabled: `DECLAREST_E2E_SIMPLE_API_BASIC_AUTH_USERNAME`, `DECLAREST_E2E_SIMPLE_API_BASIC_AUTH_PASSWORD`
- when oauth2 is enabled: `DECLAREST_E2E_SIMPLE_API_CLIENT_ID`, `DECLAREST_E2E_SIMPLE_API_CLIENT_SECRET`
- optional oauth2: `DECLAREST_E2E_SIMPLE_API_TOKEN_URL`, `DECLAREST_E2E_SIMPLE_API_SCOPE`, `DECLAREST_E2E_SIMPLE_API_AUDIENCE`
- when mTLS is enabled: `DECLAREST_E2E_SIMPLE_API_TLS_CA_CERT_FILE`, `DECLAREST_E2E_SIMPLE_API_TLS_CLIENT_CERT_FILE`, `DECLAREST_E2E_SIMPLE_API_TLS_CLIENT_KEY_FILE`
  - local-only cert volume overrides: `DECLAREST_E2E_SIMPLE_API_CERTS_HOST_DIR` (default `test/e2e/.runs/<run-id>/certs/resource-server-simple-api-server`), `DECLAREST_E2E_SIMPLE_API_CERTS_DIR` (default `/etc/simple-api-server/certs`), `DECLAREST_E2E_SIMPLE_API_MTLS_CLIENT_CERT_DIR` (default `/etc/simple-api-server/certs/clients/allowed`), `DECLAREST_E2E_SIMPLE_API_MTLS_CLIENT_CERT_FILES` (comma-separated container paths)

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
- remote vault currently supports `--resource-server-auth-type custom-header` only (`X-Vault-Token`)

### Resource Server (`rundeck`, remote)

- `DECLAREST_E2E_RESOURCE_SERVER_BASE_URL`
- `DECLAREST_E2E_RESOURCE_SERVER_TOKEN`
- optional: `DECLAREST_E2E_RESOURCE_SERVER_RUNDECK_API_VERSION`, `DECLAREST_E2E_RESOURCE_SERVER_RUNDECK_AUTH_HEADER`
- local `rundeck` with `--resource-server-auth-type custom-header` bootstraps an admin API token after startup and writes it into the generated context as `custom-header` auth (`X-Rundeck-Auth-Token`)
- remote rundeck currently supports `--resource-server-auth-type custom-header` only

### Git Provider (`gitlab`, remote)

- `DECLAREST_E2E_GITLAB_REMOTE_URL`
- `DECLAREST_E2E_GITLAB_TOKEN`
- optional: `DECLAREST_E2E_GITLAB_REMOTE_BRANCH`

### Git Provider (`github`, remote)

- `DECLAREST_E2E_GITHUB_REMOTE_URL`
- `DECLAREST_E2E_GITHUB_TOKEN`
- optional: `DECLAREST_E2E_GITHUB_REMOTE_BRANCH`

### Git Provider (`gitea`, remote)

- `DECLAREST_E2E_GITEA_REMOTE_URL`
- `DECLAREST_E2E_GITEA_TOKEN`
- optional: `DECLAREST_E2E_GITEA_REMOTE_BRANCH`

### Secret Provider (`vault`, remote)

- `DECLAREST_E2E_VAULT_ADDRESS`
- optional: `DECLAREST_E2E_VAULT_MOUNT`, `DECLAREST_E2E_VAULT_PATH_PREFIX`, `DECLAREST_E2E_VAULT_KV_VERSION`
- auth one-of:
  - token: `DECLAREST_E2E_VAULT_TOKEN`
  - userpass: `DECLAREST_E2E_VAULT_USERNAME`, `DECLAREST_E2E_VAULT_PASSWORD`, optional `DECLAREST_E2E_VAULT_AUTH_MOUNT`
  - approle: `DECLAREST_E2E_VAULT_ROLE_ID`, `DECLAREST_E2E_VAULT_SECRET_ID`, optional `DECLAREST_E2E_VAULT_AUTH_MOUNT`

Legacy `E2E_*` remote env vars are still accepted temporarily with a warning.

## Artifacts

Each run writes artifacts under `test/e2e/.runs/<run-id>/`:

- `logs/`
- `state/`
- `contexts.yaml`
- `cases/`
- `declarest-e2e-env.sh`
- `declarest-e2e-env-reset.sh`
