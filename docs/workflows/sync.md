# Syncing resources

This page walks through a full “real world” sync workflow using the local Keycloak harness.
It provisions:

- **Keycloak** as the managed server (remote REST API)
- **GitLab** as the git-remote repository provider
- **Vault** as the secret store

At the end you will have a working repository, context, and CLI commands you can run against a live API.

## Prerequisites

- `declarest` installed and available in your `PATH` (see [Installation](../getting-started/installation.md))
- Docker (or Podman with Compose support)
- `bash`, `make`, and `jq`

The harness is self-contained and uses a scratch directory under `/tmp`.
See `tests/keycloak/README.md` for the full list of prerequisites and options.

## 1) Start the environment (Keycloak + GitLab + Vault)

From the repository root:

```bash
./tests/keycloak/run-manual.sh --repo-provider gitlab --secret-provider vault
```

If you use Podman:

```bash
CONTAINER_RUNTIME=podman ./tests/keycloak/run-manual.sh --repo-provider gitlab --secret-provider vault
```

When setup completes it prints the work directory path and a few ready-to-run CLI commands.

## 2) Import the generated context into your local DeclaREST config

The harness renders a complete context file (repository + Keycloak auth + GitLab auth + Vault auth).

```bash
# Copy the "Repo:" path from the run-manual output (it ends with "/repo")
REPO_DIR="/tmp/declarest-keycloak-<run-id>/repo"
WORK_DIR="$(dirname "$REPO_DIR")"
CONTEXT_FILE="$WORK_DIR/context.yaml"

declarest config add keycloak-harness "$CONTEXT_FILE"
declarest config use keycloak-harness
declarest config check
```

## 3) Choose a resource path

List resources that exist in the prepared repository:

```bash
declarest resource list --repo
```

Pick one path from the output and use it in the rest of the commands. For example:

```bash
RESOURCE_PATH="/admin/realms/publico/clients/testB"
```

## 4) Pull remote state into the repository

```bash
declarest resource get --path "$RESOURCE_PATH" --save
```

For collection paths, DeclaREST saves each item as a separate resource by default.
To store the full collection response as one file:

```bash
declarest resource get --path "/admin/realms/publico/user-registry/ldap-test/mappers/" --save --save-as-one-resource
```

## 5) Diff and apply repository state back to the API

```bash
declarest resource diff --path "$RESOURCE_PATH"
declarest resource apply --path "$RESOURCE_PATH"
```

Use `--sync` to refresh the local file after apply:

```bash
declarest resource apply --path "$RESOURCE_PATH" --sync
```

## 6) Push repository changes to GitLab

By default, all changes are automatically synced back to the remote repository because `repository.git.remote.auto_sync=true`.

However, if that flag is disabled, you can apply changes to local Git repository and, when desired, push all changes to remote Git repository:

```bash
declarest repo push
```

## 7) Inspect secrets stored in Vault

With `--secret-provider vault`, secrets are stored in Vault instead of the repository.

```bash
declarest secret list --paths-only
```

## 8) Tear down

```bash
./tests/keycloak/run.sh clean

# this will forcefully remove any old kept runs
./tests/keycloak/run.sh clean --all
```
