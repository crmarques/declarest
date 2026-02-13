#!/usr/bin/env bash

repo_provider_fail() {
    printf "%s\n" "$1" >&2
    return 1
}

repo_provider_reset_auth_env() {
    export DECLAREST_REPO_AUTH_TYPE=""
    export DECLAREST_REPO_AUTH=""
    export DECLAREST_REPO_REMOTE_URL=""
    export DECLAREST_REPO_SSH_USER=""
    export DECLAREST_REPO_SSH_KEY_FILE=""
    export DECLAREST_REPO_SSH_PASSPHRASE=""
    export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE=""
    export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY=""
}

repo_provider_configure_auth_from_env() {
    local auth="$1"
    repo_provider_reset_auth_env
    auth="${auth,,}"

    case "$auth" in
        pat)
            if [[ -z "${DECLAREST_REPO_PAT_URL:-}" || -z "${DECLAREST_REPO_PAT_TOKEN:-}" ]]; then
                repo_provider_fail "PAT credentials are required for repo auth"
                return 1
            fi
            export DECLAREST_REPO_AUTH_TYPE="pat"
            export DECLAREST_REPO_REMOTE_URL="$DECLAREST_REPO_PAT_URL"
            export DECLAREST_REPO_AUTH="$DECLAREST_REPO_PAT_TOKEN"
            ;;
        basic)
            if [[ -z "${DECLAREST_REPO_BASIC_URL:-}" || -z "${DECLAREST_REPO_BASIC_USER:-}" || -z "${DECLAREST_REPO_BASIC_PASSWORD:-}" ]]; then
                repo_provider_fail "Basic credentials are required for repo auth"
                return 1
            fi
            export DECLAREST_REPO_AUTH_TYPE="basic"
            export DECLAREST_REPO_REMOTE_URL="$DECLAREST_REPO_BASIC_URL"
            export DECLAREST_REPO_AUTH="${DECLAREST_REPO_BASIC_USER}:${DECLAREST_REPO_BASIC_PASSWORD}"
            ;;
        ssh)
            if [[ -z "${DECLAREST_REPO_SSH_URL:-}" || -z "${DECLAREST_REPO_SSH_KEY_FILE:-}" ]]; then
                repo_provider_fail "SSH credentials are required for repo auth"
                return 1
            fi
            if [[ ! -f "$DECLAREST_REPO_SSH_KEY_FILE" ]]; then
                repo_provider_fail "SSH key file not found: $DECLAREST_REPO_SSH_KEY_FILE"
                return 1
            fi
            export DECLAREST_REPO_AUTH_TYPE="ssh"
            export DECLAREST_REPO_REMOTE_URL="$DECLAREST_REPO_SSH_URL"
            export DECLAREST_REPO_SSH_KEY_FILE="$DECLAREST_REPO_SSH_KEY_FILE"
            if [[ -n "${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}" ]]; then
                if [[ ! -f "$DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE" ]]; then
                    repo_provider_fail "SSH known hosts file not found: $DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE"
                    return 1
                fi
                export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE"
            fi
            if [[ -n "${DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY:-}" ]]; then
                export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY="$DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY"
            fi
            ;;
        *)
            repo_provider_fail "Unsupported repo auth type: ${auth}"
            return 1
            ;;
    esac
}
