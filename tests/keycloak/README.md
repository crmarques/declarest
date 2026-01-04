# Keycloak End-to-End Test Harness

This directory contains an optional end-to-end smoke test that provisions a temporary Keycloak instance (via Docker/Podman Compose) and drives the declarest CLI through a full lifecycle on the bundled template repository. It is intended for on-demand verification and is **not** run as part of `go test ./...`.

## Prerequisites

- Docker or Podman with Compose support available on your machine (set `CONTAINER_RUNTIME=podman` if needed).
- `make`, `bash`, `jq`, and `git` installed.
- Network ports 18080 (Keycloak) available. For `--repo-provider gitlab`, ports 18081 (HTTP) and 2222 (SSH) are used; for `--repo-provider gitea`, ports 18082 (HTTP) and 2223 (SSH) are used; for `--secret-provider vault`, port 18200 is used.

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
./tests/keycloak/run-e2e.sh --repo-provider file
./tests/keycloak/run-e2e.sh --repo-provider git
./tests/keycloak/run-e2e.sh --repo-provider gitlab
./tests/keycloak/run-e2e.sh --repo-provider gitea
./tests/keycloak/run-e2e.sh --repo-provider github
./tests/keycloak/run-e2e.sh --repo-provider git --secret-provider vault
CONTAINER_RUNTIME=podman ./tests/keycloak/run-e2e.sh --repo-provider file
```

Repository providers:

- `file`: use the bundled template repository directly (no Git).
- `git`: seed an empty local Git repo with the template repo, then run the same tests (prints the git log at the end).
- `gitlab`: bring up a local GitLab service, seed repositories, run the full flow with PAT, then validate basic + SSH auth with read/write checks.
- `gitea`: bring up a local Gitea service, seed repositories, run the full flow with PAT, then validate basic + SSH auth with read/write checks.
- `github`: use an existing GitHub repository; the script prompts for HTTPS/PAT and SSH details, runs the full flow with PAT, then validates SSH auth.

Secret providers:

- `none`: strip secret placeholders from the template and skip secret store checks.
- `file`: use the encrypted file secret store (default).
- `vault`: start Vault, run the full flow with token auth, then validate password + AppRole auth.

Managed server: `keycloak` (default and only option).

For `file` and `vault`, override the seeded values if needed:

```bash
DECLAREST_TEST_CLIENT_SECRET="custom-client-secret" \
DECLAREST_TEST_LDAP_BIND_CREDENTIAL="custom-bind-credential" \
DECLAREST_SECRETS_PASSPHRASE="custom-passphrase" \
./tests/keycloak/run-e2e.sh
```

The script will:

1. Build the declarest CLI (placing the binary under `/tmp/declarest-keycloak-<run-id>/bin`).
2. Launch a disposable Keycloak container with admin `admin/admin` credentials.
3. Prepare the repository for the selected `--repo-provider` and adjust the context file to point at the running Keycloak.
4. Execute a resource lifecycle (create/update/apply/get/list/delete) and secret store checks using the primary auth choices (oauth2/PAT/token), then validate other auth modes with lightweight checks. Remote providers also verify `repo check`/`repo push`/`repo refresh`/`repo reset` on the primary auth.
5. Tear down the Keycloak container.

By default the work directory is removed at the end of the run.
Logs are written under `<work>/logs` by default.
Set `DECLAREST_KEEP_WORK=1` to preserve the work directory (including logs), or set `RUN_LOG=/path/to/log` to write the log elsewhere.

## Remote Repository

When using `--repo-provider gitlab` or `--repo-provider gitea`, the harness provisions a local service and creates repositories for basic, PAT, and SSH authentication. The primary flow uses PAT, and basic/SSH are validated with lightweight read/write checks. For `--repo-provider github`, provide a repository with write access; the script prompts for HTTPS/PAT and SSH details, uses PAT for the full flow, and validates SSH with read/write checks.

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
