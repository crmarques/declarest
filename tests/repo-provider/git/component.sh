#!/usr/bin/env bash

REPO_PROVIDER_NAME="git"
REPO_PROVIDER_TYPE="git-local"
REPO_PROVIDER_PRIMARY_AUTH=""
REPO_PROVIDER_SECONDARY_AUTH=()
REPO_PROVIDER_REMOTE_PROVIDER=""
REPO_PROVIDER_INTERACTIVE_AUTH="0"

repo_provider_apply_env() {
    export DECLAREST_REPO_TYPE="git-local"
    export DECLAREST_REPO_PROVIDER=""
    export DECLAREST_REMOTE_REPO_PROVIDER=""
    export DECLAREST_REPO_REMOTE_URL=""
    export DECLAREST_GITLAB_ENABLE="0"
    export DECLAREST_GITEA_ENABLE="0"
}

repo_provider_prepare_services() {
    return 0
}

repo_provider_prepare_interactive() {
    return 0
}

repo_provider_configure_auth() {
    return 0
}
