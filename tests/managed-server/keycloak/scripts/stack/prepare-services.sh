#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TESTS_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$TESTS_ROOT/scripts/components.sh"

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

secret_provider="${DECLAREST_SECRET_STORE_TYPE:-file}"
secret_provider="${secret_provider,,}"
load_secret_provider_component "$secret_provider"
secret_provider_prepare_services

wait_for_keycloak
