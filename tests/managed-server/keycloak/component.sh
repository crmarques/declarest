#!/usr/bin/env bash

MANAGED_SERVER_NAME="keycloak"
MANAGED_SERVER_DESCRIPTION="Keycloak managed server harness"

managed_server_default_repo_provider() {
    printf "%s" "git"
}

managed_server_default_secret_provider() {
    printf "%s" "file"
}

managed_server_validate() {
    local repo_provider="$1"
    local secret_provider="$2"
    if [[ -z "$repo_provider" || -z "$secret_provider" ]]; then
        printf "Missing repo/secret provider for managed server %s\n" "$MANAGED_SERVER_NAME" >&2
        return 1
    fi
}

managed_server_runner() {
    local mode="$1"
    local dir
    dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    case "$mode" in
        e2e)
            printf "%s/run-e2e.sh" "$dir"
            ;;
        interactive)
            printf "%s/run-interactive.sh" "$dir"
            ;;
        *)
            printf "Unsupported mode: %s\n" "$mode" >&2
            return 1
            ;;
    esac
}
