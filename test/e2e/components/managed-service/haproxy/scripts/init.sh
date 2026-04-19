#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

selected_auth_type=${E2E_MANAGED_SERVICE_AUTH_TYPE:-basic}
case "${selected_auth_type}" in
  basic|prompt) ;;
  *)
    e2e_die "managed-service haproxy does not support auth-type ${selected_auth_type} (supported: basic, prompt)"
    exit 1
    ;;
esac

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  haproxy_port=$(e2e_pick_free_port)
  admin_user='admin'
  admin_password='admin'
  base_url="http://127.0.0.1:${haproxy_port}"

  e2e_write_state_value "${state_file}" HAPROXY_DPA_PORT "${haproxy_port}"
  e2e_write_state_value "${state_file}" HAPROXY_BASE_URL "${base_url}"
  e2e_write_state_value "${state_file}" HAPROXY_ADMIN_USER "${admin_user}"
  e2e_write_state_value "${state_file}" HAPROXY_ADMIN_PASSWORD "${admin_password}"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_BASE_URL "${base_url}/v3"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_AUTH_KIND "basic"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_BASIC_USERNAME "${admin_user}"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_BASIC_PASSWORD "${admin_password}"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_BASE_URL "${base_url}"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_API_BASE_URL "${base_url}/v3"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_USERNAME "${admin_user}"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_PASSWORD "${admin_password}"
  e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_AUTH_MODE "basic"
  exit 0
fi

haproxy_base_url=$(e2e_require_env 'DECLAREST_E2E_MANAGED_SERVICE_BASE_URL') || exit 1
haproxy_user=$(e2e_require_env 'DECLAREST_E2E_MANAGED_SERVICE_USERNAME') || exit 1
haproxy_password=$(e2e_require_env 'DECLAREST_E2E_MANAGED_SERVICE_PASSWORD') || exit 1

e2e_write_state_value "${state_file}" HAPROXY_BASE_URL "${haproxy_base_url%/}"
e2e_write_state_value "${state_file}" HAPROXY_ADMIN_USER "${haproxy_user}"
e2e_write_state_value "${state_file}" HAPROXY_ADMIN_PASSWORD "${haproxy_password}"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_BASE_URL "${haproxy_base_url%/}/v3"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_AUTH_KIND "basic"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_BASIC_USERNAME "${haproxy_user}"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_BASIC_PASSWORD "${haproxy_password}"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_BASE_URL "${haproxy_base_url%/}"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_API_BASE_URL "${haproxy_base_url%/}/v3"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_USERNAME "${haproxy_user}"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_PASSWORD "${haproxy_password}"
e2e_write_state_value "${state_file}" MANAGED_SERVICE_ACCESS_AUTH_MODE "basic"
