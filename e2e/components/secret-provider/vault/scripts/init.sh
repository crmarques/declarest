#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  vault_port=$(e2e_pick_free_port)
  vault_token="root-${RANDOM}${RANDOM}${RANDOM}"
  vault_address="http://127.0.0.1:${vault_port}"

  e2e_write_state_value "${state_file}" VAULT_PORT "${vault_port}"
  e2e_write_state_value "${state_file}" VAULT_ADDRESS "${vault_address}"
  e2e_write_state_value "${state_file}" VAULT_TOKEN "${vault_token}"
  e2e_write_state_value "${state_file}" VAULT_MOUNT "secret"
  e2e_write_state_value "${state_file}" VAULT_PATH_PREFIX "declarest-e2e"
  e2e_write_state_value "${state_file}" VAULT_KV_VERSION "2"
  exit 0
fi

vault_address=$(e2e_require_env 'DECLAREST_E2E_VAULT_ADDRESS' 'E2E_VAULT_ADDRESS') || exit 1
vault_mount=$(e2e_env_optional 'DECLAREST_E2E_VAULT_MOUNT' 'E2E_VAULT_MOUNT' || true)
vault_mount=${vault_mount:-secret}
vault_path_prefix=$(e2e_env_optional 'DECLAREST_E2E_VAULT_PATH_PREFIX' 'E2E_VAULT_PATH_PREFIX' || true)
vault_path_prefix=${vault_path_prefix:-declarest-e2e}
vault_kv_version=$(e2e_env_optional 'DECLAREST_E2E_VAULT_KV_VERSION' 'E2E_VAULT_KV_VERSION' || true)
vault_kv_version=${vault_kv_version:-2}

vault_token=$(e2e_env_optional 'DECLAREST_E2E_VAULT_TOKEN' 'E2E_VAULT_TOKEN' || true)
vault_username=$(e2e_env_optional 'DECLAREST_E2E_VAULT_USERNAME' 'E2E_VAULT_USERNAME' || true)
vault_password=$(e2e_env_optional 'DECLAREST_E2E_VAULT_PASSWORD' 'E2E_VAULT_PASSWORD' || true)
vault_role_id=$(e2e_env_optional 'DECLAREST_E2E_VAULT_ROLE_ID' 'E2E_VAULT_ROLE_ID' || true)
vault_secret_id=$(e2e_env_optional 'DECLAREST_E2E_VAULT_SECRET_ID' 'E2E_VAULT_SECRET_ID' || true)
vault_auth_mount=$(e2e_env_optional 'DECLAREST_E2E_VAULT_AUTH_MOUNT' 'E2E_VAULT_AUTH_MOUNT' || true)

e2e_write_state_value "${state_file}" VAULT_ADDRESS "${vault_address}"
e2e_write_state_value "${state_file}" VAULT_MOUNT "${vault_mount}"
e2e_write_state_value "${state_file}" VAULT_PATH_PREFIX "${vault_path_prefix}"
e2e_write_state_value "${state_file}" VAULT_KV_VERSION "${vault_kv_version}"

if [[ -n "${vault_token}" ]]; then
  e2e_write_state_value "${state_file}" VAULT_AUTH_MODE "token"
  e2e_write_state_value "${state_file}" VAULT_TOKEN "${vault_token}"
  exit 0
fi

if [[ -n "${vault_username}" && -n "${vault_password}" ]]; then
  e2e_write_state_value "${state_file}" VAULT_AUTH_MODE "password"
  e2e_write_state_value "${state_file}" VAULT_USERNAME "${vault_username}"
  e2e_write_state_value "${state_file}" VAULT_PASSWORD "${vault_password}"
  e2e_write_state_value "${state_file}" VAULT_AUTH_MOUNT "${vault_auth_mount:-userpass}"
  exit 0
fi

if [[ -n "${vault_role_id}" && -n "${vault_secret_id}" ]]; then
  e2e_write_state_value "${state_file}" VAULT_AUTH_MODE "approle"
  e2e_write_state_value "${state_file}" VAULT_ROLE_ID "${vault_role_id}"
  e2e_write_state_value "${state_file}" VAULT_SECRET_ID "${vault_secret_id}"
  e2e_write_state_value "${state_file}" VAULT_AUTH_MOUNT "${vault_auth_mount:-approle}"
  exit 0
fi

printf 'vault remote connection requires token, userpass, or approle credentials\n' >&2
exit 1
