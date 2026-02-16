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

: "${E2E_VAULT_ADDRESS:?missing env E2E_VAULT_ADDRESS}"

e2e_write_state_value "${state_file}" VAULT_ADDRESS "${E2E_VAULT_ADDRESS}"
e2e_write_state_value "${state_file}" VAULT_MOUNT "${E2E_VAULT_MOUNT:-secret}"
e2e_write_state_value "${state_file}" VAULT_PATH_PREFIX "${E2E_VAULT_PATH_PREFIX:-declarest-e2e}"
e2e_write_state_value "${state_file}" VAULT_KV_VERSION "${E2E_VAULT_KV_VERSION:-2}"

if [[ -n "${E2E_VAULT_TOKEN:-}" ]]; then
  e2e_write_state_value "${state_file}" VAULT_AUTH_MODE "token"
  e2e_write_state_value "${state_file}" VAULT_TOKEN "${E2E_VAULT_TOKEN}"
  exit 0
fi

if [[ -n "${E2E_VAULT_USERNAME:-}" && -n "${E2E_VAULT_PASSWORD:-}" ]]; then
  e2e_write_state_value "${state_file}" VAULT_AUTH_MODE "password"
  e2e_write_state_value "${state_file}" VAULT_USERNAME "${E2E_VAULT_USERNAME}"
  e2e_write_state_value "${state_file}" VAULT_PASSWORD "${E2E_VAULT_PASSWORD}"
  e2e_write_state_value "${state_file}" VAULT_AUTH_MOUNT "${E2E_VAULT_AUTH_MOUNT:-userpass}"
  exit 0
fi

if [[ -n "${E2E_VAULT_ROLE_ID:-}" && -n "${E2E_VAULT_SECRET_ID:-}" ]]; then
  e2e_write_state_value "${state_file}" VAULT_AUTH_MODE "approle"
  e2e_write_state_value "${state_file}" VAULT_ROLE_ID "${E2E_VAULT_ROLE_ID}"
  e2e_write_state_value "${state_file}" VAULT_SECRET_ID "${E2E_VAULT_SECRET_ID}"
  e2e_write_state_value "${state_file}" VAULT_AUTH_MOUNT "${E2E_VAULT_AUTH_MOUNT:-approle}"
  exit 0
fi

printf 'vault remote connection requires token, userpass, or approle credentials\n' >&2
exit 1
