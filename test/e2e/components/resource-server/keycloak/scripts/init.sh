#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

selected_auth_type=${E2E_RESOURCE_SERVER_AUTH_TYPE:-oauth2}
if [[ "${selected_auth_type}" != 'oauth2' ]]; then
  e2e_die "resource-server keycloak does not support auth-type ${selected_auth_type} (supported: oauth2)"
  exit 1
fi

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  keycloak_port=$(e2e_pick_free_port)
  admin_user=$(e2e_env_optional 'DECLAREST_E2E_KEYCLOAK_ADMIN_USER' 'E2E_KEYCLOAK_ADMIN_USER' || true)
  admin_password=$(e2e_env_optional 'DECLAREST_E2E_KEYCLOAK_ADMIN_PASSWORD' 'E2E_KEYCLOAK_ADMIN_PASSWORD' || true)
  : "${admin_user:=admin}"
  : "${admin_password:=admin}"
  realm='master'
  client_id='declarest-e2e-client'
  client_secret="client-${RANDOM}${RANDOM}${RANDOM}"

  e2e_write_state_value "${state_file}" KEYCLOAK_HOST_PORT "${keycloak_port}"
  e2e_write_state_value "${state_file}" KEYCLOAK_ADMIN_USER "${admin_user}"
  e2e_write_state_value "${state_file}" KEYCLOAK_ADMIN_PASSWORD "${admin_password}"
  e2e_write_state_value "${state_file}" KEYCLOAK_REALM "${realm}"
  e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_ID "${client_id}"
  e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_SECRET "${client_secret}"
  e2e_write_state_value "${state_file}" KEYCLOAK_BASE_URL "http://127.0.0.1:${keycloak_port}"
  e2e_write_state_value "${state_file}" RESOURCE_SERVER_BASE_URL "http://127.0.0.1:${keycloak_port}"
  e2e_write_state_value "${state_file}" KEYCLOAK_TOKEN_URL "http://127.0.0.1:${keycloak_port}/realms/${realm}/protocol/openid-connect/token"
  exit 0
fi

resource_server_base_url=$(e2e_require_env 'DECLAREST_E2E_RESOURCE_SERVER_BASE_URL' 'E2E_RESOURCE_SERVER_BASE_URL') || exit 1
keycloak_token_url=$(e2e_require_env 'DECLAREST_E2E_KEYCLOAK_TOKEN_URL' 'E2E_KEYCLOAK_TOKEN_URL') || exit 1
keycloak_client_id=$(e2e_require_env 'DECLAREST_E2E_KEYCLOAK_CLIENT_ID' 'E2E_KEYCLOAK_CLIENT_ID') || exit 1
keycloak_client_secret=$(e2e_require_env 'DECLAREST_E2E_KEYCLOAK_CLIENT_SECRET' 'E2E_KEYCLOAK_CLIENT_SECRET') || exit 1
keycloak_scope=$(e2e_env_optional 'DECLAREST_E2E_KEYCLOAK_SCOPE' 'E2E_KEYCLOAK_SCOPE' || true)
keycloak_audience=$(e2e_env_optional 'DECLAREST_E2E_KEYCLOAK_AUDIENCE' 'E2E_KEYCLOAK_AUDIENCE' || true)

e2e_write_state_value "${state_file}" KEYCLOAK_BASE_URL "${resource_server_base_url}"
e2e_write_state_value "${state_file}" RESOURCE_SERVER_BASE_URL "${resource_server_base_url}"
e2e_write_state_value "${state_file}" KEYCLOAK_TOKEN_URL "${keycloak_token_url}"
e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_ID "${keycloak_client_id}"
e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_SECRET "${keycloak_client_secret}"

if [[ -n "${keycloak_scope}" ]]; then
  e2e_write_state_value "${state_file}" KEYCLOAK_SCOPE "${keycloak_scope}"
fi
if [[ -n "${keycloak_audience}" ]]; then
  e2e_write_state_value "${state_file}" KEYCLOAK_AUDIENCE "${keycloak_audience}"
fi
