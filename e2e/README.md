# E2E Test Runner

This repository uses a componentized Bash e2e harness.

## Entrypoint

```bash
./run-e2e.sh --profile basic
```

## Profiles

- `basic` (default): runs all `main` cases that match the selected stack.
- `full`: runs all `main` and `corner` cases that match the selected stack.
- `manual`: starts local-instantiable components, writes a temporary context catalog, and waits for manual interaction.
  - with no component flags, it uses the maximal local stack (`keycloak` + `git/gitlab` + `vault`)
  - remote component selections are rejected in Step 1

## Main Flags

- `--profile <basic|full|manual>`
- `--resource-server <keycloak|none>`
- `--resource-server-connection <local|remote>`
- `--repo-type <filesystem|git>`
- `--git-provider <git|gitlab|github>`
- `--git-provider-connection <local|remote>`
- `--secret-provider <file|vault|none>`
- `--secret-provider-connection <local|remote>`
- `--list-components`
- `--keep-runtime`
- `--verbose`

## Runtime Steps

The runner reports progress in 7 grouped steps:

1. `Initializing`
2. `Preparing Runtime`
3. `Preparing Components`
4. `Starting Components`
5. `Configuring Access`
6. `Running Workload`
7. `Finalizing`

TTY mode renders dynamic spinner/status updates. Non-TTY mode emits structured plain step lines.

## Case Model

Cases live under:

- `e2e/cases/main/*.sh`
- `e2e/cases/corner/*.sh`

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

Docker-backed local components should also provide:

- `compose.yaml`
- `scripts/health.sh`

## Remote Environment Variables

### Resource Server (`keycloak`, remote)

- `E2E_RESOURCE_SERVER_BASE_URL`
- `E2E_KEYCLOAK_TOKEN_URL`
- `E2E_KEYCLOAK_CLIENT_ID`
- `E2E_KEYCLOAK_CLIENT_SECRET`
- optional: `E2E_KEYCLOAK_SCOPE`, `E2E_KEYCLOAK_AUDIENCE`

### Git Provider (`gitlab`, remote)

- `E2E_GITLAB_REMOTE_URL`
- `E2E_GITLAB_TOKEN`
- optional: `E2E_GITLAB_REMOTE_BRANCH`

### Git Provider (`github`, remote)

- `E2E_GITHUB_REMOTE_URL`
- `E2E_GITHUB_TOKEN`
- optional: `E2E_GITHUB_REMOTE_BRANCH`

### Secret Provider (`vault`, remote)

- `E2E_VAULT_ADDRESS`
- optional: `E2E_VAULT_MOUNT`, `E2E_VAULT_PATH_PREFIX`, `E2E_VAULT_KV_VERSION`
- auth one-of:
  - token: `E2E_VAULT_TOKEN`
  - userpass: `E2E_VAULT_USERNAME`, `E2E_VAULT_PASSWORD`, optional `E2E_VAULT_AUTH_MOUNT`
  - approle: `E2E_VAULT_ROLE_ID`, `E2E_VAULT_SECRET_ID`, optional `E2E_VAULT_AUTH_MOUNT`

## Artifacts

Each run writes artifacts under `e2e/.runs/<run-id>/`:

- `logs/`
- `state/`
- `contexts.yaml`
- `cases/`
