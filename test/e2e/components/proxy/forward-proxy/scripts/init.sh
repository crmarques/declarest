#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

kind_node_path_for_host_path() {
  local host_path=$1
  local host_root="${E2E_ROOT_DIR%/}"
  local node_root="${E2E_KIND_NODE_ROOT%/}"

  case "${host_path}" in
    "${host_root}")
      printf '%s\n' "${node_root}"
      ;;
    "${host_root}/"*)
      printf '%s/%s\n' "${node_root}" "${host_path#${host_root}/}"
      ;;
    *)
      printf '%s\n' "${host_path}"
      ;;
  esac
}

effective_proxy_auth_type() {
  if [[ -n "${E2E_PROXY_AUTH_TYPE:-}" ]]; then
    printf '%s\n' "${E2E_PROXY_AUTH_TYPE}"
    return 0
  fi

  if [[ -n "${E2E_PROXY_AUTH_USERNAME:-}" || -n "${E2E_PROXY_AUTH_PASSWORD:-}" ]]; then
    printf 'basic\n'
    return 0
  fi

  case "${E2E_PROXY_MODE:-none}" in
    local)
      printf 'basic\n'
      ;;
    *)
      printf 'none\n'
      ;;
  esac
}

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  e2e_die 'proxy forward-proxy supports only local connection mode'
  exit 1
fi

proxy_host_port=$(e2e_pick_free_port)
proxy_runtime_dir="${E2E_RUN_DIR}/proxy"
proxy_access_log="${proxy_runtime_dir}/access.log"
proxy_runtime_node_dir=$(kind_node_path_for_host_path "${proxy_runtime_dir}")
proxy_auth_type=$(effective_proxy_auth_type)
proxy_server_auth_mode='none'
proxy_auth_username="${E2E_PROXY_AUTH_USERNAME:-}"
proxy_auth_password="${E2E_PROXY_AUTH_PASSWORD:-}"

mkdir -p "${proxy_runtime_dir}"
: >"${proxy_access_log}"

case "${proxy_auth_type}" in
  none)
    ;;
  basic|prompt)
    proxy_server_auth_mode='basic'
    : "${proxy_auth_username:=declarest-e2e-proxy}"
    : "${proxy_auth_password:=proxy-${RANDOM}${RANDOM}${RANDOM}}"
    ;;
  *)
    e2e_die "unsupported proxy auth type: ${proxy_auth_type}"
    exit 1
    ;;
esac

e2e_write_state_value "${state_file}" PROXY_HOST_PORT "${proxy_host_port}"
e2e_write_state_value "${state_file}" PROXY_HTTP_URL "http://127.0.0.1:${proxy_host_port}"
e2e_write_state_value "${state_file}" PROXY_HTTPS_URL "http://127.0.0.1:${proxy_host_port}"
e2e_write_state_value "${state_file}" PROXY_INTERNAL_PORT '3128'
e2e_write_state_value "${state_file}" PROXY_RUNTIME_DIR "${proxy_runtime_dir}"
e2e_write_state_value "${state_file}" PROXY_RUNTIME_NODE_DIR "${proxy_runtime_node_dir}"
e2e_write_state_value "${state_file}" PROXY_ACCESS_LOG "${proxy_access_log}"
e2e_write_state_value "${state_file}" PROXY_REPO_ROOT "${E2E_ROOT_DIR}"
e2e_write_state_value "${state_file}" PROXY_AUTH_TYPE "${proxy_auth_type}"
e2e_write_state_value "${state_file}" PROXY_SERVER_AUTH_MODE "${proxy_server_auth_mode}"

if [[ -n "${proxy_auth_username}" ]]; then
  e2e_write_state_value "${state_file}" PROXY_AUTH_USERNAME "${proxy_auth_username}"
fi
if [[ -n "${proxy_auth_password}" ]]; then
  e2e_write_state_value "${state_file}" PROXY_AUTH_PASSWORD "${proxy_auth_password}"
fi
