#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

selected_auth_type=${E2E_RESOURCE_SERVER_AUTH_TYPE:-custom-header}
case "${selected_auth_type}" in
  basic|custom-header) ;;
  *)
    e2e_die "resource-server rundeck does not support auth-type ${selected_auth_type} (supported: basic, custom-header)"
    exit 1
    ;;
esac

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  rundeck_port=$(e2e_pick_free_port)
  admin_user='admin'
  # The current rundeck container image keeps the default realm.properties admin password.
  # Use the real local login credential so basic and token bootstrap auth both work.
  admin_password='admin'
  base_url="http://127.0.0.1:${rundeck_port}"

  e2e_write_state_value "${state_file}" RUNDECK_HTTP_PORT "${rundeck_port}"
  e2e_write_state_value "${state_file}" RUNDECK_BASE_URL "${base_url}"
  e2e_write_state_value "${state_file}" RUNDECK_ADMIN_USER "${admin_user}"
  e2e_write_state_value "${state_file}" RUNDECK_ADMIN_PASSWORD "${admin_password}"
  e2e_write_state_value "${state_file}" RUNDECK_API_VERSION "45"
  if [[ "${selected_auth_type}" == 'custom-header' ]]; then
    e2e_write_state_value "${state_file}" RUNDECK_AUTH_MODE "token"
  else
    e2e_write_state_value "${state_file}" RUNDECK_AUTH_MODE "basic"
  fi
  e2e_write_state_value "${state_file}" RUNDECK_AUTH_HEADER "X-Rundeck-Auth-Token"
  exit 0
fi

if [[ "${selected_auth_type}" != 'custom-header' ]]; then
  e2e_die "resource-server rundeck remote connection supports only auth-type custom-header"
  exit 1
fi

rundeck_base_url=$(e2e_require_env 'DECLAREST_E2E_RESOURCE_SERVER_BASE_URL' 'E2E_RESOURCE_SERVER_BASE_URL') || exit 1
rundeck_token=$(e2e_require_env 'DECLAREST_E2E_RESOURCE_SERVER_TOKEN' 'E2E_RESOURCE_SERVER_TOKEN') || exit 1
rundeck_api_version=$(e2e_env_optional 'DECLAREST_E2E_RESOURCE_SERVER_RUNDECK_API_VERSION' 'E2E_RESOURCE_SERVER_RUNDECK_API_VERSION' || true)
rundeck_api_version=${rundeck_api_version:-45}
rundeck_auth_header=$(e2e_env_optional 'DECLAREST_E2E_RESOURCE_SERVER_RUNDECK_AUTH_HEADER' 'E2E_RESOURCE_SERVER_RUNDECK_AUTH_HEADER' || true)
rundeck_auth_header=${rundeck_auth_header:-X-Rundeck-Auth-Token}

e2e_write_state_value "${state_file}" RUNDECK_BASE_URL "${rundeck_base_url}"
e2e_write_state_value "${state_file}" RUNDECK_API_VERSION "${rundeck_api_version}"
e2e_write_state_value "${state_file}" RUNDECK_API_TOKEN "${rundeck_token}"
e2e_write_state_value "${state_file}" RUNDECK_AUTH_MODE "token"
e2e_write_state_value "${state_file}" RUNDECK_AUTH_HEADER "${rundeck_auth_header}"
