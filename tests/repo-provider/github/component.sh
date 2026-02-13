#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$BASE_DIR/lib/component.sh"
source "$BASE_DIR/lib/github-auth.sh"

REPO_PROVIDER_NAME="github"
REPO_PROVIDER_TYPE="git-remote"
REPO_PROVIDER_PRIMARY_AUTH="pat"
REPO_PROVIDER_SECONDARY_AUTH=(ssh)
REPO_PROVIDER_REMOTE_PROVIDER="github"
REPO_PROVIDER_INTERACTIVE_AUTH="1"

resolve_known_hosts() {
    local candidate="$1"
    if [[ -n "$candidate" ]]; then
        printf "%s" "$candidate"
        return 0
    fi
    local default_known_hosts="$HOME/.ssh/known_hosts"
    if [[ -f "$default_known_hosts" ]]; then
        printf "%s" "$default_known_hosts"
        return 0
    fi
    printf "%s" ""
}

repo_provider_map_env() {
    local resolved_hosts
    export DECLAREST_REPO_PAT_URL="${github_https_url:-}"
    export DECLAREST_REPO_PAT_TOKEN="${github_pat:-}"
    export DECLAREST_REPO_SSH_URL="${github_ssh_url:-}"
    export DECLAREST_REPO_SSH_KEY_FILE="${github_ssh_key_file:-}"
    export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE=""
    export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY=""
    export DECLAREST_REPO_SSH_USER="git"

    if [[ -n "${github_ssh_insecure:-}" ]]; then
        export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY="1"
        return 0
    fi

    if [[ -n "${github_ssh_known_hosts:-}" ]]; then
        export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$github_ssh_known_hosts"
        return 0
    fi

    resolved_hosts="$(resolve_known_hosts "")"
    if [[ -n "$resolved_hosts" ]]; then
        export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$resolved_hosts"
    fi
}

repo_provider_apply_env() {
    export DECLAREST_REPO_TYPE="git-remote"
    export DECLAREST_REPO_PROVIDER="github"
    export DECLAREST_REMOTE_REPO_PROVIDER="github"
    export DECLAREST_GITLAB_ENABLE="0"
    export DECLAREST_GITEA_ENABLE="0"
}

repo_provider_prepare_services() {
    ensure_github_pat_ssh_credentials
    repo_provider_map_env
}

repo_provider_prepare_interactive() {
    ensure_github_pat_credentials
    repo_provider_map_env
}

repo_provider_configure_auth() {
    local auth="$1"
    repo_provider_configure_auth_from_env "$auth" || return 1
    if [[ "${auth,,}" == "ssh" && -z "${DECLAREST_REPO_SSH_USER:-}" ]]; then
        export DECLAREST_REPO_SSH_USER="git"
    fi
}
