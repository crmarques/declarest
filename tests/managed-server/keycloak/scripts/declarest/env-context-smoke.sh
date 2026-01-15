#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
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

if [[ "${DECLAREST_SECRET_STORE_TYPE:-}" == "file" ]]; then
    log_line "Validating context file placeholders"
    placeholder_context="keycloak-env-placeholders"
    placeholder_context_path="$DECLAREST_WORK_DIR/context-env-placeholders.yaml"
    token_url="${base_url}/realms/master/protocol/openid-connect/token"
    cat <<EOF > "$placeholder_context_path"
managed_server:
  http:
    base_url: "$base_url"
    auth:
      oauth2:
        token_url: "$token_url"
        grant_type: password
        client_id: admin-cli
        username: "$KEYCLOAK_ADMIN_USER"
        password: "$KEYCLOAK_ADMIN_PASS"
    tls:
      insecure_skip_verify: true
repository:
  filesystem:
    base_dir: "\${DECLAREST_ENV_CONTEXT_REPO_DIR}"
secret_store:
  file:
    path: "\${DECLAREST_ENV_CONTEXT_SECRETS_FILE}"
    passphrase: "\${DECLAREST_ENV_CONTEXT_SECRETS_PASSPHRASE}"
EOF

    run_cli "register context with placeholders" config add "$placeholder_context" "$placeholder_context_path"

    export DECLAREST_ENV_CONTEXT_REPO_DIR="$DECLAREST_REPO_DIR"
    export DECLAREST_ENV_CONTEXT_SECRETS_FILE="$DECLAREST_SECRETS_FILE"
    export DECLAREST_ENV_CONTEXT_SECRETS_PASSPHRASE="$DECLAREST_SECRETS_PASSPHRASE"
    export DECLAREST_CTX_NAME="$placeholder_context"

    run_cli "config check with placeholders" config check
else
    log_line "Skipping context placeholder validation (file secret store disabled)"
fi
