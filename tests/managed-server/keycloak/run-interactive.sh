#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$KEYCLOAK_DIR/scripts"

usage() {
    local managed_servers repo_providers secret_providers
    managed_servers="$(list_components "managed-server" | paste -sd ", " -)"
    repo_providers="$(list_components "repo-provider" | paste -sd ", " -)"
    secret_providers="$(list_components "secret-provider" | paste -sd ", " -)"
    managed_servers="${managed_servers:-none}"
    repo_providers="${repo_providers:-none}"
    secret_providers="${secret_providers:-none}"
    cat <<EOF
Usage: ./tests/managed-server/keycloak/run-interactive.sh [--managed-server NAME] [--repo-provider NAME] [--secret-provider NAME]

Options:
  --managed-server NAME   Managed server to target (default: keycloak)
                          Available: ${managed_servers}
  --repo-provider NAME    Repository provider:
                          ${repo_providers}
  --secret-provider NAME  Secret store provider:
                          ${secret_providers}
  -h, --help              Show this help message.

Defaults:
  managed-server=keycloak
  repo-provider=managed-server default
  secret-provider=managed-server default
EOF
}

source "$SCRIPTS_DIR/lib/args.sh"

managed_server="keycloak"
repo_provider="git"
secret_provider="file"

parse_common_flags "$@"
load_repo_provider_component "$repo_provider"
load_secret_provider_component "$secret_provider"
repo_provider_apply_env
secret_provider_apply_env
repo_type="${REPO_PROVIDER_TYPE}"

if [[ "$repo_type" == "git-remote" && "${REPO_PROVIDER_INTERACTIVE_AUTH:-0}" -eq 1 ]]; then
    repo_provider_prepare_interactive
    repo_auth_type="${DECLAREST_REPO_AUTH_TYPE:-$REPO_PROVIDER_PRIMARY_AUTH}"
    if [[ -n "$repo_auth_type" ]]; then
        repo_provider_configure_auth "$repo_auth_type"
    fi
fi

exec "$KEYCLOAK_DIR/run.sh" setup
