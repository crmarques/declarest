#!/usr/bin/env bash

set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
export CONTAINER_RUNTIME

if [[ -z "${DECLAREST_DEBUG_GROUPS:-}" ]]; then
    export DECLAREST_DEBUG_GROUPS="network"
fi

usage() {
    cat <<EOF
Usage: ./tests/run-tests.sh [--e2e|--interactive] --managed-server <name> --repo-provider <name> --secret-provider <type> [-- <extra args>]

Options:
  --e2e                 Run the managed server's e2e workflow (default).
  --interactive         Run the managed server's interactive workflow.
  --managed-server NAME Managed server to target (keycloak or rundeck).
  --repo-provider NAME  Repository provider (file, git, gitlab, gitea, github).
  --secret-provider TYPE Secret store provider (none, file, vault).
  -h, --help            Show this help message.

Examples:
  ./tests/run-tests.sh --e2e --managed-server keycloak --repo-provider gitea --secret-provider vault
  ./tests/run-tests.sh --interactive --managed-server keycloak --repo-provider git --secret-provider file

Defaults:
  mode=e2e
  managed-server=keycloak
  repo-provider=git
  secret-provider=file (rundeck defaults to none)
EOF
}

mode="e2e"
managed_server="keycloak"
repo_provider="git"
secret_provider="file"
repo_provider_set=0
secret_provider_set=0
extra_args=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --e2e)
            mode="e2e"
            shift
            ;;
        --interactive)
            mode="interactive"
            shift
            ;;
        --managed-server)
            managed_server="${2:-}"
            shift 2
            ;;
        --repo-provider)
            repo_provider="${2:-}"
            repo_provider_set=1
            shift 2
            ;;
        --secret-provider)
            secret_provider="${2:-}"
            secret_provider_set=1
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --)
            shift
            extra_args+=("$@")
            break
            ;;
        *)
            printf "Unknown option: %s\n" "$1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

managed_server="${managed_server,,}"
repo_provider="${repo_provider,,}"
secret_provider="${secret_provider,,}"

check_container_runtime() {
    if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
        printf "Container runtime %s is not installed or not in PATH.\n" "$CONTAINER_RUNTIME" >&2
        printf "Install it or set CONTAINER_RUNTIME to another runtime (docker, podman).\n" >&2
        exit 1
    fi
    local runtime_err
    if ! runtime_err="$("$CONTAINER_RUNTIME" ps 2>&1 >/dev/null)"; then
        if [[ "$runtime_err" == *"alive.lck"* ]]; then
            printf "Container runtime %s cannot acquire its runtime lock:\n" "$CONTAINER_RUNTIME" >&2
            printf "%s\n" "$runtime_err" >&2
            printf "This environment restricts rootless %s; try running with a different runtime\n" "$CONTAINER_RUNTIME" >&2
            printf "or configure %s with the privileges described in %s.\n" "$CONTAINER_RUNTIME" "https://podman.io/" >&2
        else
            printf "Container runtime %s is not usable:\n" "$CONTAINER_RUNTIME" >&2
            printf "%s\n" "$runtime_err" >&2
        fi
        exit 1
    fi
}

case "$managed_server" in
    keycloak|rundeck)
        ;;
    *)
        printf "Unsupported managed server: %s\n" "$managed_server" >&2
        exit 1
        ;;
esac

if [[ "$managed_server" == "rundeck" ]]; then
    if [[ $repo_provider_set -eq 0 ]]; then
        repo_provider="file"
    fi
    if [[ $secret_provider_set -eq 0 ]]; then
        secret_provider="none"
    fi
    if [[ "$repo_provider" != "file" ]]; then
        printf "Rundeck harness supports only repo-provider file (got %s)\n" "$repo_provider" >&2
        exit 1
    fi
    if [[ "$secret_provider" != "none" ]]; then
        printf "Rundeck harness does not support secret providers (got %s)\n" "$secret_provider" >&2
        exit 1
    fi
fi

case "$repo_provider" in
    file|git|gitlab|gitea|github)
        ;;
    *)
        printf "Unsupported repo provider: %s\n" "$repo_provider" >&2
        exit 1
        ;;
esac

case "$secret_provider" in
    none|file|vault)
        ;;
    *)
        printf "Unsupported secret provider: %s\n" "$secret_provider" >&2
        exit 1
        ;;
esac

check_container_runtime

managed_dir="$TESTS_DIR/managed-server/$managed_server"
if [[ ! -d "$managed_dir" ]]; then
    printf "Managed server directory not found: %s\n" "$managed_dir" >&2
    exit 1
fi

case "$mode" in
    e2e)
        runner="$managed_dir/run-e2e.sh"
        ;;
    interactive)
        runner="$managed_dir/run-interactive.sh"
        ;;
    *)
        printf "Unsupported mode: %s\n" "$mode" >&2
        exit 1
        ;;
esac

if [[ ! -x "$runner" ]]; then
    printf "Runner script not found or not executable: %s\n" "$runner" >&2
    exit 1
fi

printf "Running mode=%s, managed-server=%s, repo-provider=%s, secret-provider=%s\n" "$mode" "$managed_server" "$repo_provider" "$secret_provider"

exec "$runner" \
    --managed-server "$managed_server" \
    --repo-provider "$repo_provider" \
    --secret-provider "$secret_provider" \
    "${extra_args[@]}"
