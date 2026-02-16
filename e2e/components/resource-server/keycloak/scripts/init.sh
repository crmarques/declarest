#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  keycloak_port=$(e2e_pick_free_port)
  api_port=$(e2e_pick_free_port)

  admin_user='admin'
  admin_password="admin-${RANDOM}${RANDOM}"
  realm='declarest-e2e'
  client_id='declarest-e2e-client'
  client_secret="client-${RANDOM}${RANDOM}${RANDOM}"

  e2e_write_state_value "${state_file}" KEYCLOAK_HOST_PORT "${keycloak_port}"
  e2e_write_state_value "${state_file}" RESOURCE_API_HOST_PORT "${api_port}"
  e2e_write_state_value "${state_file}" KEYCLOAK_ADMIN_USER "${admin_user}"
  e2e_write_state_value "${state_file}" KEYCLOAK_ADMIN_PASSWORD "${admin_password}"
  e2e_write_state_value "${state_file}" KEYCLOAK_REALM "${realm}"
  e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_ID "${client_id}"
  e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_SECRET "${client_secret}"
  e2e_write_state_value "${state_file}" KEYCLOAK_BASE_URL "http://127.0.0.1:${keycloak_port}"
  e2e_write_state_value "${state_file}" RESOURCE_API_BASE_URL "http://127.0.0.1:${api_port}"
  e2e_write_state_value "${state_file}" KEYCLOAK_TOKEN_URL "http://127.0.0.1:${keycloak_port}/realms/${realm}/protocol/openid-connect/token"
  exit 0
fi

: "${E2E_RESOURCE_SERVER_BASE_URL:?missing env E2E_RESOURCE_SERVER_BASE_URL}"
: "${E2E_KEYCLOAK_TOKEN_URL:?missing env E2E_KEYCLOAK_TOKEN_URL}"
: "${E2E_KEYCLOAK_CLIENT_ID:?missing env E2E_KEYCLOAK_CLIENT_ID}"
: "${E2E_KEYCLOAK_CLIENT_SECRET:?missing env E2E_KEYCLOAK_CLIENT_SECRET}"

e2e_write_state_value "${state_file}" RESOURCE_API_BASE_URL "${E2E_RESOURCE_SERVER_BASE_URL}"
e2e_write_state_value "${state_file}" KEYCLOAK_TOKEN_URL "${E2E_KEYCLOAK_TOKEN_URL}"
e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_ID "${E2E_KEYCLOAK_CLIENT_ID}"
e2e_write_state_value "${state_file}" KEYCLOAK_CLIENT_SECRET "${E2E_KEYCLOAK_CLIENT_SECRET}"

if [[ -n "${E2E_KEYCLOAK_SCOPE:-}" ]]; then
  e2e_write_state_value "${state_file}" KEYCLOAK_SCOPE "${E2E_KEYCLOAK_SCOPE}"
fi
if [[ -n "${E2E_KEYCLOAK_AUDIENCE:-}" ]]; then
  e2e_write_state_value "${state_file}" KEYCLOAK_AUDIENCE "${E2E_KEYCLOAK_AUDIENCE}"
fi
