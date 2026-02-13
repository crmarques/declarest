#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$BASE_DIR/lib/component.sh"

REPO_PROVIDER_NAME="gitea"
REPO_PROVIDER_TYPE="git-remote"
REPO_PROVIDER_PRIMARY_AUTH="pat"
REPO_PROVIDER_SECONDARY_AUTH=(basic ssh)
REPO_PROVIDER_REMOTE_PROVIDER="gitea"
REPO_PROVIDER_INTERACTIVE_AUTH="0"

repo_provider_setup_script() {
    printf "%s/setup.sh" "$SCRIPT_DIR"
}

repo_provider_env_file() {
    printf "%s/gitea.env" "$DECLAREST_WORK_DIR"
}

repo_provider_map_env() {
    export DECLAREST_REPO_BASIC_URL="${DECLAREST_GITEA_BASIC_URL:-}"
    export DECLAREST_REPO_PAT_URL="${DECLAREST_GITEA_PAT_URL:-}"
    export DECLAREST_REPO_SSH_URL="${DECLAREST_GITEA_SSH_URL:-}"
    export DECLAREST_REPO_BASIC_USER="${DECLAREST_GITEA_USER:-}"
    export DECLAREST_REPO_BASIC_PASSWORD="${DECLAREST_GITEA_PASSWORD:-}"
    export DECLAREST_REPO_PAT_TOKEN="${DECLAREST_GITEA_PAT:-}"
    export DECLAREST_REPO_SSH_KEY_FILE="${DECLAREST_GITEA_SSH_KEY_FILE:-}"
    export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="${DECLAREST_GITEA_KNOWN_HOSTS_FILE:-}"
    export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY=""
}

repo_provider_apply_env() {
    export DECLAREST_REPO_TYPE="git-remote"
    export DECLAREST_REPO_PROVIDER="gitea"
    export DECLAREST_REMOTE_REPO_PROVIDER="gitea"
    export DECLAREST_GITLAB_ENABLE="0"
    export DECLAREST_GITEA_ENABLE="1"
}

repo_provider_prepare_services() {
    local setup_script env_file
    setup_script="$(repo_provider_setup_script)"
    if [[ -n "$setup_script" ]]; then
        "$setup_script"
    fi
    env_file="$(repo_provider_env_file)"
    if [[ ! -f "$env_file" ]]; then
        repo_provider_fail "Repo provider env file missing: $env_file"
        return 1
    fi
    source "$env_file"
    repo_provider_map_env
}

repo_provider_prepare_interactive() {
    return 0
}

repo_provider_configure_auth() {
    local auth="$1"
    repo_provider_configure_auth_from_env "$auth" || return 1
    if [[ "${auth,,}" == "ssh" && -z "${DECLAREST_REPO_SSH_USER:-}" ]]; then
        export DECLAREST_REPO_SSH_USER="git"
    fi
}
