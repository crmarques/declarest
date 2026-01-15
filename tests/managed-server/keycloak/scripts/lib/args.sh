#!/usr/bin/env bash

ARGS_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$ARGS_LIB_DIR/shell.sh"

require_arg() {
    local opt="$1"
    local value="${2:-}"
    if [[ -z "$value" ]]; then
        die "Missing value for ${opt}"
    fi
}

parse_common_flags() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --managed-server)
                require_arg "$1" "${2:-}"
                managed_server="${2:-}"
                shift 2
                ;;
            --repo-provider)
                require_arg "$1" "${2:-}"
                repo_provider="${2:-}"
                shift 2
                ;;
            --secret-provider)
                require_arg "$1" "${2:-}"
                secret_provider="${2:-}"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                printf "Unknown option: %s\n" "$1" >&2
                usage >&2
                exit 1
                ;;
        esac
    done

    managed_server="${managed_server,,}"
    case "$managed_server" in
        keycloak)
            ;;
        *)
            die "Invalid --managed-server: ${managed_server} (expected keycloak)"
            ;;
    esac

    repo_provider="${repo_provider,,}"
    case "$repo_provider" in
        file|git|gitlab|gitea|github)
            ;;
        *)
            die "Invalid --repo-provider: ${repo_provider} (expected file, git, gitlab, gitea, or github)"
            ;;
    esac

    secret_provider="${secret_provider,,}"
    case "$secret_provider" in
        none|file|vault)
            ;;
        *)
            die "Invalid --secret-provider: ${secret_provider} (expected none, file, or vault)"
            ;;
    esac
}

resolve_repo_provider() {
    repo_type=""
    remote_repo_provider=""
    case "$repo_provider" in
        file)
            repo_type="fs"
            ;;
        git)
            repo_type="git-local"
            ;;
        gitlab|gitea|github)
            repo_type="git-remote"
            remote_repo_provider="$repo_provider"
            ;;
        *)
            die "Unsupported repo provider: ${repo_provider}"
            ;;
    esac
}

apply_repo_provider_env() {
    export DECLAREST_REPO_TYPE="$repo_type"
    export DECLAREST_SECRET_STORE_TYPE="$secret_provider"
    export DECLAREST_REMOTE_REPO_PROVIDER=""
    export DECLAREST_REPO_PROVIDER=""
    export DECLAREST_GITLAB_ENABLE="0"
    export DECLAREST_GITEA_ENABLE="0"

    if [[ "$repo_type" == "git-remote" ]]; then
        export DECLAREST_REPO_PROVIDER="$remote_repo_provider"
        export DECLAREST_REMOTE_REPO_PROVIDER="$remote_repo_provider"
        case "$remote_repo_provider" in
            gitlab)
                export DECLAREST_GITLAB_ENABLE="1"
                ;;
            gitea)
                export DECLAREST_GITEA_ENABLE="1"
                ;;
        esac
    fi
}

clear_remote_repo_env() {
    export DECLAREST_REPO_REMOTE_URL=""
    export DECLAREST_REMOTE_REPO_PROVIDER=""
    export DECLAREST_REPO_PROVIDER=""
}
