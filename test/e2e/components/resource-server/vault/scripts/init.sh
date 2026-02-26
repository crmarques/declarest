#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

selected_auth_type=${E2E_RESOURCE_SERVER_AUTH_TYPE:-custom-header}
if [[ "${selected_auth_type}" != 'custom-header' ]]; then
  e2e_die "resource-server vault does not support auth-type ${selected_auth_type} (supported: custom-header)"
  exit 1
fi

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

vault_address=$(e2e_require_env 'DECLAREST_E2E_RESOURCE_SERVER_BASE_URL' 'E2E_RESOURCE_SERVER_BASE_URL') || exit 1
vault_token=$(e2e_require_env 'DECLAREST_E2E_RESOURCE_SERVER_TOKEN' 'E2E_RESOURCE_SERVER_TOKEN') || exit 1
vault_mount=$(e2e_env_optional 'DECLAREST_E2E_RESOURCE_SERVER_VAULT_MOUNT' 'E2E_RESOURCE_SERVER_VAULT_MOUNT' || true)
vault_mount=${vault_mount:-secret}
vault_path_prefix=$(e2e_env_optional 'DECLAREST_E2E_RESOURCE_SERVER_VAULT_PATH_PREFIX' 'E2E_RESOURCE_SERVER_VAULT_PATH_PREFIX' || true)
vault_path_prefix=${vault_path_prefix:-declarest-e2e}
vault_kv_version=$(e2e_env_optional 'DECLAREST_E2E_RESOURCE_SERVER_VAULT_KV_VERSION' 'E2E_RESOURCE_SERVER_VAULT_KV_VERSION' || true)
vault_kv_version=${vault_kv_version:-2}

e2e_write_state_value "${state_file}" VAULT_ADDRESS "${vault_address}"
e2e_write_state_value "${state_file}" VAULT_TOKEN "${vault_token}"
e2e_write_state_value "${state_file}" VAULT_MOUNT "${vault_mount}"
e2e_write_state_value "${state_file}" VAULT_PATH_PREFIX "${vault_path_prefix}"
e2e_write_state_value "${state_file}" VAULT_KV_VERSION "${vault_kv_version}"
