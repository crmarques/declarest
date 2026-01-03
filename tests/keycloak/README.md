# Keycloak End-to-End Test Harness

This directory contains an optional end-to-end smoke test that provisions a temporary Keycloak instance (via Docker/Podman Compose) and drives the declarest CLI through a full lifecycle on the bundled template repository. It is intended for on-demand verification and is **not** run as part of `go test ./...`.

## Prerequisites

- Docker or Podman with Compose support available on your machine (set `CONTAINER_RUNTIME=podman` if needed).
- `make`, `bash`, and `jq` installed.
- Network ports 18080 (Keycloak) available. For `--repo-type git-remote`, ports 18081 (GitLab HTTP) and 2222 (GitLab SSH) are also used.

## Layout

```
/tests/keycloak
├── README.md            – This guide
├── run-e2e.sh           – Main entrypoint
├── scripts
│   ├── lib              – Shared helpers (env/logging/cli/etc.)
│   ├── stack            – Compose stack lifecycle (Keycloak + optional GitLab/Gitea)
│   ├── workspace        – Work directory setup/cleanup
│   ├── repo             – Repository preparation + verification
│   ├── context          – Context rendering + registration
│   ├── declarest        – CLI build + workflow steps
│   └── providers        – GitLab/Gitea bootstrap scripts
├── templates            – Golden copy of repo + context used for each run
│   ├── repo             – Template repository content
│   └── config.yaml      – Declarest context configuration template
└── work                 – Deprecated scratch path (unused by default)
```

## Usage

```bash
# From repository root
./tests/keycloak/run-e2e.sh --repo-type fs
./tests/keycloak/run-e2e.sh --repo-type git-local
./tests/keycloak/run-e2e.sh --repo-type git-remote
CONTAINER_RUNTIME=podman ./tests/keycloak/run-e2e.sh --repo-type fs
```

Repository modes:

- `fs`: use the bundled template repository directly (no Git).
- `git-local`: seed an empty local Git repo with the template repo, then run the same tests (prints the git log at the end).
- `git-remote`: bring up a local GitLab service, seed repositories, and run the tests across basic, PAT, and SSH auth.

Secret-related checks use the encrypted file secrets manager configured in `templates/config.yaml`.
Override the seeded values if needed:

```bash
DECLAREST_TEST_CLIENT_SECRET="custom-client-secret" \
DECLAREST_TEST_LDAP_BIND_CREDENTIAL="custom-bind-credential" \
DECLAREST_SECRETS_PASSPHRASE="custom-passphrase" \
./tests/keycloak/run-e2e.sh
```

The script will:

1. Build the declarest CLI (placing the binary under `/tmp/declarest-keycloak-<run-id>/bin`).
2. Launch a disposable Keycloak container with admin `admin/admin` credentials.
3. Prepare the repository for the selected `--repo-type` and adjust the context file to point at the running Keycloak.
4. Execute a resource lifecycle (create/update/apply/get/list/delete) and secrets manager checks, syncing remote state into the repository as needed. For `git-remote`, it also verifies `repo check`/`repo push`/`repo refresh`/`repo reset`.
5. Tear down the Keycloak container.

By default the work directory is removed at the end of the run.
Logs are written under `<work>/logs` by default.
Set `DECLAREST_KEEP_WORK=1` to preserve the work directory (including logs), or set `RUN_LOG=/path/to/log` to write the log elsewhere.

## Remote Repository

When using `--repo-type git-remote`, the harness provisions a temporary GitLab instance and creates three repositories for basic, PAT, and SSH authentication. The repository URLs and credentials are generated under the work directory and torn down after the run.

## Updating Templates

Edit the contents under `templates/repo` to match the desired test repository. The `run-e2e.sh` script copies this directory on each run when `DECLAREST_REPO_REMOTE_URL` is empty so your templates remain untouched.

The `templates/config.yaml` file is a declarest context definition; the script injects the temporary Keycloak base URL before invoking any commands.

## Caveats

- The test uses Docker. Adapt the scripts if your environment requires `podman` or remote Docker hosts.
- Keycloak boot can take several seconds. The script waits for the admin endpoint to respond before proceeding; adjust the timeout if needed.
- This harness is intentionally self-contained; it does not modify your main workspace beyond the `/tmp/declarest-keycloak-<run-id>` scratch directory.

## Work Directory Overrides

By default the scripts use `/tmp/declarest-keycloak-<run-id>`. Override this if you need a fixed location:

```bash
DECLAREST_WORK_DIR=/tmp/declarest-keycloak ./tests/keycloak/run-e2e.sh
```

To choose a different base directory while keeping the generated run id:

```bash
DECLAREST_WORK_BASE_DIR=/var/tmp ./tests/keycloak/run-e2e.sh
```
