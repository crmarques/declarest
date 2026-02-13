#!/usr/bin/env bash

MANAGED_SERVER_NAME="rundeck"
MANAGED_SERVER_DESCRIPTION="Rundeck managed server harness"

managed_server_default_repo_provider() {
    printf "%s" "file"
}

managed_server_default_secret_provider() {
    printf "%s" "none"
}

managed_server_validate() {
    local repo_provider="$1"
    local secret_provider="$2"
    if [[ "$repo_provider" != "file" ]]; then
        printf "Rundeck harness supports only repo-provider file (got %s)\n" "$repo_provider" >&2
        return 1
    fi
    if [[ "$secret_provider" != "none" ]]; then
        printf "Rundeck harness does not support secret providers (got %s)\n" "$secret_provider" >&2
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
