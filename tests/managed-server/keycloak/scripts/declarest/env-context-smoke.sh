#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/cli.sh
source "$SCRIPTS_DIR/lib/cli.sh"

log_line "Validating context overrides via environment"

env_name="keycloak-env-overrides"
export DECLAREST_CTX_NAME="$env_name"
export DECLAREST_CTX_REPOSITORY_FILESYSTEM_BASE_DIR="$DECLAREST_REPO_DIR"

base_url="http://localhost:${KEYCLOAK_HTTP_PORT}"
export DECLAREST_CTX_MANAGED_SERVER_HTTP_BASE_URL="$base_url"
export DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_TOKEN_URL="${base_url}/realms/master/protocol/openid-connect/token"
export DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_GRANT_TYPE="password"
export DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_CLIENT_ID="admin-cli"
export DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_USERNAME="$KEYCLOAK_ADMIN_USER"
export DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_PASSWORD="$KEYCLOAK_ADMIN_PASS"

run_cli "env context overrides" config check
