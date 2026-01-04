#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$KEYCLOAK_DIR/scripts"

usage() {
    cat <<EOF
Usage: ./tests/keycloak/run-manual.sh [--managed-server NAME] [--repo-provider NAME] [--secret-provider NAME]

Options:
  --managed-server NAME   Managed server to target (default: keycloak)
                          Available: keycloak
  --repo-provider NAME    Repository provider:
                          file, git, gitlab, gitea, github
  --secret-provider NAME  Secret store provider:
                          none, file, vault
  -h, --help              Show this help message.

Defaults:
  managed-server=keycloak
  repo-provider=git
  secret-provider=file
EOF
}

# shellcheck source=scripts/lib/args.sh
source "$SCRIPTS_DIR/lib/args.sh"
# shellcheck source=scripts/lib/github-auth.sh
source "$SCRIPTS_DIR/lib/github-auth.sh"

managed_server="keycloak"
repo_provider="git"
secret_provider="file"

parse_common_flags "$@"
resolve_repo_provider
apply_repo_provider_env

if [[ "$repo_type" == "git-remote" ]]; then
    case "$remote_repo_provider" in
        github)
            repo_auth_type="${DECLAREST_REPO_AUTH_TYPE:-}"
            repo_auth_type="${repo_auth_type,,}"
            if [[ -z "$repo_auth_type" || "$repo_auth_type" == "pat" ]]; then
                ensure_github_pat_credentials
                if [[ -z "${DECLAREST_REPO_AUTH_TYPE:-}" ]]; then
                    export DECLAREST_REPO_AUTH_TYPE="pat"
                fi
                if [[ -z "${DECLAREST_REPO_AUTH:-}" ]]; then
                    export DECLAREST_REPO_AUTH="$github_pat"
                fi
                if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
                    export DECLAREST_REPO_REMOTE_URL="$github_https_url"
                fi
            fi
            ;;
    esac
    if [[ -z "${DECLAREST_REPO_AUTH_TYPE:-}" ]]; then
        export DECLAREST_REPO_AUTH_TYPE="pat"
    fi
    if [[ -z "${DECLAREST_REPO_REMOTE_URL+x}" && ( "$remote_repo_provider" == "gitlab" || "$remote_repo_provider" == "gitea" ) ]]; then
        export DECLAREST_REPO_REMOTE_URL=""
    fi
else
    clear_remote_repo_env
fi

exec "$KEYCLOAK_DIR/run.sh" setup
