#!/usr/bin/env bash

resolve_repo_type() {
    local repo_type="${DECLAREST_REPO_TYPE:-}"
    if [[ -z "$repo_type" ]]; then
        if [[ -n "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
            repo_type="git-remote"
        else
            repo_type="git-local"
        fi
    fi
    repo_type="${repo_type,,}"
    printf "%s" "$repo_type"
}

valid_repo_type() {
    case "$1" in
        fs|git-local|git-remote)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}
