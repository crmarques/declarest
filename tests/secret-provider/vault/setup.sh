#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"

secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-file}"
secret_store_type="${secret_store_type,,}"
if [[ "$secret_store_type" != "vault" ]]; then
    log_line "Vault setup skipped (secret store type: ${secret_store_type})"
    exit 0
fi

require_cmd "$CONTAINER_RUNTIME"
require_cmd jq

vault_addr_host="http://localhost:${VAULT_HTTP_PORT}"
vault_addr_container="http://127.0.0.1:8200"
vault_env_file="$DECLAREST_WORK_DIR/vault.env"

vault_exec() {
    "$CONTAINER_RUNTIME" exec -e VAULT_ADDR="$vault_addr_container" "$VAULT_CONTAINER_NAME" vault "$@"
}

vault_exec_root() {
    "$CONTAINER_RUNTIME" exec -i -e VAULT_ADDR="$vault_addr_container" -e VAULT_TOKEN="$1" "$VAULT_CONTAINER_NAME" vault "${@:2}"
}

vault_status_json() {
    local output status
    set +e
    output="$(vault_exec status -format=json 2>&1)"
    status=$?
    set -e
    if [[ $status -ne 0 && -z "$output" ]]; then
        return 1
    fi
    if jq -e . >/dev/null 2>&1 <<<"$output"; then
        printf "%s" "$output"
        return 0
    fi
    return 1
}

wait_for_vault() {
    local attempts=${VAULT_WAIT_ATTEMPTS:-60}
    local delay=${VAULT_WAIT_DELAY:-2}

    log_line "Waiting for Vault readiness at $vault_addr_host (${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        if curl -sS --connect-timeout 1 --max-time 2 "$vault_addr_host/v1/sys/health" >/dev/null 2>&1; then
            log_line "Vault API is reachable after attempt ${i}"
            return 0
        fi
        if (( i % 10 == 0 )); then
            log_line "Still waiting for Vault (${i}/${attempts})"
        fi
        sleep "$delay"
    done
    log_line "Vault did not become ready in time"

    local status_output port_output recent_logs
    status_output=$("$CONTAINER_RUNTIME" ps --all --filter "name=${VAULT_CONTAINER_NAME}" 2>&1 || true)
    port_output=$("$CONTAINER_RUNTIME" port "$VAULT_CONTAINER_NAME" 2>&1 || true)
    recent_logs=$("$CONTAINER_RUNTIME" logs "$VAULT_CONTAINER_NAME" 2>&1 | tail -n 100 || true)
    log_block "Vault container status" "$status_output"
    log_block "Vault port mapping" "$port_output"
    log_block "Recent Vault container logs" "$recent_logs"
    return 1
}

wait_for_vault

status_json="$(vault_status_json || true)"
initialized="false"
sealed="true"
if [[ -n "$status_json" ]]; then
    initialized="$(jq -r '.initialized // false' <<<"$status_json" 2>/dev/null || echo "false")"
    sealed="$(jq -r '.sealed // true' <<<"$status_json" 2>/dev/null || echo "true")"
fi

root_token="${DECLAREST_VAULT_TOKEN:-}"
unseal_key="${DECLAREST_VAULT_UNSEAL_KEY:-}"
if [[ "$initialized" != "true" ]]; then
    init_attempts="${VAULT_INIT_ATTEMPTS:-10}"
    init_delay="${VAULT_INIT_DELAY:-2}"
    init_status=1
    init_json=""
    for ((attempt=1; attempt<=init_attempts; attempt++)); do
        set +e
        init_json="$(vault_exec operator init -key-shares=1 -key-threshold=1 -format=json 2>&1)"
        init_status=$?
        set -e
        if [[ $init_status -eq 0 ]]; then
            root_token="$(jq -r '.root_token' <<<"$init_json")"
            unseal_key="$(jq -r '.unseal_keys_b64[0]' <<<"$init_json")"
            sealed="true"
            break
        fi
        if grep -qi "already initialized" <<<"$init_json"; then
            initialized="true"
            break
        fi
        log_line "Vault init failed (attempt ${attempt}/${init_attempts}); retrying..."
        log_block "Vault init output (attempt ${attempt})" "$init_json"
        sleep "$init_delay"
    done
    if [[ "$initialized" != "true" && $init_status -ne 0 ]]; then
        die "Vault init failed after ${init_attempts} attempts"
    fi
fi

if [[ "$sealed" == "true" ]]; then
    if [[ -z "$unseal_key" ]]; then
        unseal_key="$(jq -r '.unseal_keys_b64[0] // empty' <<<"$status_json" 2>/dev/null || true)"
    fi
    if [[ -z "$unseal_key" ]]; then
        die "Vault is sealed but no unseal key is available"
    fi
    vault_exec operator unseal "$unseal_key" >/dev/null
fi

if [[ -z "$root_token" ]]; then
    die "Vault root token is required to configure auth"
fi

secrets_list_json="$(vault_exec_root "$root_token" secrets list -format=json 2>/dev/null || echo "{}")"
if ! jq -e --arg mount "${DECLAREST_VAULT_MOUNT}/" '.[$mount]' >/dev/null <<<"$secrets_list_json"; then
    vault_exec_root "$root_token" secrets enable -path="${DECLAREST_VAULT_MOUNT}" -version=2 kv >/dev/null
fi

policy_name="declarest"
vault_exec_root "$root_token" policy write "$policy_name" - <<EOF
path "${DECLAREST_VAULT_MOUNT}/data/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "${DECLAREST_VAULT_MOUNT}/metadata" {
  capabilities = ["list", "read"]
}
path "${DECLAREST_VAULT_MOUNT}/metadata/*" {
  capabilities = ["list", "read", "delete"]
}
EOF

auth_list_json="$(vault_exec_root "$root_token" auth list -format=json 2>/dev/null || echo "{}")"
if ! jq -e '.["userpass/"]' >/dev/null <<<"$auth_list_json"; then
    vault_exec_root "$root_token" auth enable userpass >/dev/null
fi
if ! jq -e '.["approle/"]' >/dev/null <<<"$auth_list_json"; then
    vault_exec_root "$root_token" auth enable approle >/dev/null
fi

vault_exec_root "$root_token" write "auth/userpass/users/${DECLAREST_VAULT_USERNAME}" \
    password="$DECLAREST_VAULT_PASSWORD" \
    policies="$policy_name" >/dev/null

vault_exec_root "$root_token" write "auth/approle/role/declarest" \
    token_policies="$policy_name" \
    token_ttl="1h" \
    token_max_ttl="24h" >/dev/null

role_id_json="$(vault_exec_root "$root_token" read -format=json auth/approle/role/declarest/role-id)"
role_id="$(jq -r '.data.role_id' <<<"$role_id_json")"
secret_id_json="$(vault_exec_root "$root_token" write -format=json -f auth/approle/role/declarest/secret-id)"
secret_id="$(jq -r '.data.secret_id' <<<"$secret_id_json")"

cat <<ENVFILE > "$vault_env_file"
export DECLAREST_VAULT_ADDR=${vault_addr_host@Q}
export DECLAREST_VAULT_TOKEN=${root_token@Q}
export DECLAREST_VAULT_UNSEAL_KEY=${unseal_key@Q}
export DECLAREST_VAULT_USERNAME=${DECLAREST_VAULT_USERNAME@Q}
export DECLAREST_VAULT_PASSWORD=${DECLAREST_VAULT_PASSWORD@Q}
export DECLAREST_VAULT_ROLE_ID=${role_id@Q}
export DECLAREST_VAULT_SECRET_ID=${secret_id@Q}
ENVFILE

log_line "Vault setup complete; credentials written to $vault_env_file"
