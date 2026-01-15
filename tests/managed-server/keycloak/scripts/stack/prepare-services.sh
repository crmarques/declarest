#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"

SECRET_PROVIDER_DIR="$DECLAREST_TESTS_ROOT/secret-provider"

wait_for_keycloak() {
    local url="http://localhost:${KEYCLOAK_HTTP_PORT}/realms/master"
    local attempts=${KEYCLOAK_WAIT_ATTEMPTS:-150}
    local delay=${KEYCLOAK_WAIT_DELAY:-2}

    log_line "Waiting for Keycloak to become ready at $url (${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        if curl -sk --fail "$url" >/dev/null 2>&1; then
            log_line "Keycloak is up after attempt ${i}"
            return 0
        fi
        if (( i % 10 == 0 )); then
            log_line "Still waiting for Keycloak (${i}/${attempts})"
        fi
        sleep "$delay"
    done

    log_line "Keycloak did not become ready in time"
    local status_output recent_logs
    status_output=$("$CONTAINER_RUNTIME" ps --all --filter "name=${KEYCLOAK_CONTAINER_NAME}" || true)
    recent_logs=$("$CONTAINER_RUNTIME" logs "${KEYCLOAK_CONTAINER_NAME}" 2>&1 | tail -n 50 || true)
    log_block "Container status" "$status_output"
    log_block "Recent container logs" "$recent_logs"
    return 1
}

secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-file}"
secret_store_type="${secret_store_type,,}"
vault_enabled=0
case "$secret_store_type" in
    vault)
        vault_enabled=1
        ;;
    none|file|"")
        ;;
    *)
        log_line "Unsupported secret store type: ${secret_store_type}"
        exit 1
        ;;
esac

if [[ $vault_enabled -eq 1 ]]; then
    log_line "Configuring Vault instance"
    "$SECRET_PROVIDER_DIR/vault/setup.sh"
fi

wait_for_keycloak
