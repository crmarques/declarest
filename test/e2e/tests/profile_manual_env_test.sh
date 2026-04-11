#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_profile_libs() {
  source_e2e_libs common components profile
}

prepare_manual_env_runtime() {
  local tmp=$1

  export E2E_RUN_ID='manual-env-hook-test'
  export E2E_RUNS_DIR="${tmp}/runs"
  export E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  export E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  export E2E_CONTEXT_FILE="${E2E_RUN_DIR}/contexts.yaml"
  export E2E_BIN="${E2E_RUN_DIR}/bin/declarest"
  export E2E_PLATFORM="${E2E_PLATFORM:-compose}"
  export E2E_KUBECONFIG="${E2E_KUBECONFIG:-}"
  export E2E_KIND_CLUSTER_NAME="${E2E_KIND_CLUSTER_NAME:-}"
  export E2E_K8S_NAMESPACE="${E2E_K8S_NAMESPACE:-}"

  mkdir -p "${E2E_STATE_DIR}" "$(dirname -- "${E2E_BIN}")"
  cat >"${E2E_BIN}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == 'context' && "${2:-}" == 'session-hook' && "${3:-}" == 'bash' ]]; then
  cat <<'HOOK'
if [ -z "${DECLAREST_PROMPT_AUTH_SESSION_ID:-}" ]; then
  export DECLAREST_PROMPT_AUTH_SESSION_ID="bash:${BASHPID:-$$}"
fi
declare -f declarest_prompt_auth_cleanup >/dev/null 2>&1 || declarest_prompt_auth_cleanup() {
  command declarest context clean --credentials-in-session >/dev/null 2>&1 || true
}
if [ -z "${__declarest_prompt_auth_bash_hook_installed:-}" ]; then
  __declarest_prompt_auth_prev_exit="$(trap -p EXIT | sed -n "s/^trap -- '\\(.*\\)' EXIT$/\\1/p")"
  declarest_prompt_auth_on_exit() {
    declarest_prompt_auth_cleanup
    if [ -n "${__declarest_prompt_auth_prev_exit:-}" ]; then
      eval "$__declarest_prompt_auth_prev_exit"
    fi
  }
  trap 'declarest_prompt_auth_on_exit' EXIT
  __declarest_prompt_auth_bash_hook_installed=1
fi
HOOK
  exit 0
fi

if [[ "${1:-}" == 'context' && "${2:-}" == 'clean' && "${3:-}" == '--credentials-in-session' ]]; then
  exit 0
fi

exit 0
EOF
  chmod +x "${E2E_BIN}"
}

write_manual_env_scripts() {
  local tmp=$1

  SETUP_SCRIPT="${tmp}/setup.sh"
  RESET_SCRIPT="${tmp}/reset.sh"
  local state_key_list
  state_key_list=$(e2e_manual_collect_state_env_keys | sort -u | tr '\n' ' ' | sed 's/[[:space:]]\+$//')
  e2e_manual_write_env_setup_script "e2e-manual" "${SETUP_SCRIPT}" "${RESET_SCRIPT}" "${state_key_list}"
  e2e_manual_write_env_reset_script "${RESET_SCRIPT}"
}

prepare_manual_env_scripts() {
  local tmp=$1
  prepare_manual_env_runtime "${tmp}"
  write_manual_env_scripts "${tmp}"
}

test_manual_env_scripts_install_and_restore_prompt_hook() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local SETUP_SCRIPT RESET_SCRIPT
  prepare_manual_env_scripts "${tmp}"

  assert_file_contains "${SETUP_SCRIPT}" "__declarest_e2e_prune_deleted_run_bins_from_path"
  assert_file_contains "${SETUP_SCRIPT}" "DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET"
  assert_file_contains "${RESET_SCRIPT}" "unset -f __declarest_e2e_prune_deleted_run_bins_from_path"
  assert_file_contains "${RESET_SCRIPT}" "DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND"

  local output status
  set +e
  output=$(
    SETUP_SCRIPT="${SETUP_SCRIPT}" RESET_SCRIPT="${RESET_SCRIPT}" bash <<'EOF'
set -euo pipefail
PROMPT_COMMAND='printf pre >/dev/null'
export PROMPT_COMMAND
source "${SETUP_SCRIPT}"

case ";${PROMPT_COMMAND};" in
  *";__declarest_e2e_prune_deleted_run_bins_from_path; "*) ;;
  *)
    printf 'missing prompt hook: %s\n' "${PROMPT_COMMAND}" >&2
    exit 1
    ;;
esac

source "${RESET_SCRIPT}"
[[ "${PROMPT_COMMAND}" == 'printf pre >/dev/null' ]]
type __declarest_e2e_prune_deleted_run_bins_from_path >/dev/null 2>&1 && exit 1 || true
EOF
  )
  status=$?
  set -e
  assert_status "${status}" "0"
  [[ -z "${output}" ]] || true
}

test_manual_env_scripts_enable_and_reset_prompt_auth_session() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local SETUP_SCRIPT RESET_SCRIPT
  prepare_manual_env_scripts "${tmp}"

  assert_file_contains "${SETUP_SCRIPT}" 'context session-hook bash'
  assert_file_contains "${RESET_SCRIPT}" 'DECLAREST_PROMPT_AUTH_SESSION_ID'
  assert_file_contains "${RESET_SCRIPT}" 'DECLAREST_E2E_INSTALLED_PROMPT_AUTH_HOOK'

  local output status
  set +e
  output=$(
    SETUP_SCRIPT="${SETUP_SCRIPT}" RESET_SCRIPT="${RESET_SCRIPT}" bash <<'EOF'
set -euo pipefail
source "${SETUP_SCRIPT}"

[[ -n "${DECLAREST_PROMPT_AUTH_SESSION_ID:-}" ]]
[[ "${DECLAREST_E2E_INSTALLED_PROMPT_AUTH_HOOK:-}" == '1' ]]
[[ "${__declarest_prompt_auth_bash_hook_installed:-}" == '1' ]]

source "${RESET_SCRIPT}"

[[ -z "${DECLAREST_PROMPT_AUTH_SESSION_ID:-}" ]]
[[ -z "${DECLAREST_E2E_INSTALLED_PROMPT_AUTH_HOOK:-}" ]]
[[ -z "${__declarest_prompt_auth_bash_hook_installed:-}" ]]
type declarest_prompt_auth_on_exit >/dev/null 2>&1 && exit 1 || true
EOF
  )
  status=$?
  set -e
  assert_status "${status}" "0"
  [[ -z "${output}" ]] || true
}

test_manual_env_scripts_skip_local_prompt_proxy_auth_exports() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"; unset E2E_PROXY_MODE E2E_PROXY_AUTH_TYPE' RETURN

  export E2E_PROXY_MODE='local'
  export E2E_PROXY_AUTH_TYPE='prompt'

  prepare_manual_env_runtime "${tmp}"

  local proxy_state="${E2E_STATE_DIR}/proxy-forward-proxy.env"
  : >"${proxy_state}"
  e2e_write_state_value "${proxy_state}" 'PROXY_HTTP_URL' 'http://127.0.0.1:3128'
  e2e_write_state_value "${proxy_state}" 'PROXY_AUTH_TYPE' 'prompt'
  e2e_write_state_value "${proxy_state}" 'PROXY_SERVER_AUTH_MODE' 'basic'
  e2e_write_state_value "${proxy_state}" 'PROXY_AUTH_USERNAME' 'proxy-user'
  e2e_write_state_value "${proxy_state}" 'PROXY_AUTH_PASSWORD' 'proxy-pass'

  write_manual_env_scripts "${tmp}"

  assert_not_contains "$(cat "${SETUP_SCRIPT}")" "PROXY_AUTH_TYPE="
  assert_not_contains "$(cat "${SETUP_SCRIPT}")" "PROXY_SERVER_AUTH_MODE="
  assert_not_contains "$(cat "${SETUP_SCRIPT}")" "PROXY_AUTH_USERNAME="
  assert_not_contains "$(cat "${SETUP_SCRIPT}")" "PROXY_AUTH_PASSWORD="

  local output status
  set +e
  output=$(
    SETUP_SCRIPT="${SETUP_SCRIPT}" bash <<'EOF'
set -euo pipefail
source "${SETUP_SCRIPT}"

[[ "${PROXY_HTTP_URL}" == 'http://127.0.0.1:3128' ]]
[[ -z "${PROXY_AUTH_TYPE:-}" ]]
[[ -z "${PROXY_AUTH_USERNAME:-}" ]]
[[ -z "${PROXY_AUTH_PASSWORD:-}" ]]

case " ${DECLAREST_E2E_STATE_ENV_KEYS} " in
  *" PROXY_AUTH_TYPE "*|*" PROXY_SERVER_AUTH_MODE "*|*" PROXY_AUTH_USERNAME "*|*" PROXY_AUTH_PASSWORD "*)
    printf 'unexpected proxy auth state key export list: %s\n' "${DECLAREST_E2E_STATE_ENV_KEYS}" >&2
    exit 1
    ;;
esac
EOF
  )
  status=$?
  set -e
  assert_status "${status}" "0"
  [[ -z "${output}" ]] || true
}

test_manual_env_prompt_hook_prunes_deleted_run_bin_path_and_alias() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local SETUP_SCRIPT RESET_SCRIPT run_dir run_bin
  prepare_manual_env_scripts "${tmp}"
  run_dir="${E2E_RUN_DIR}"
  run_bin="${run_dir}/bin"

  local output status
  set +e
  output=$(
    SETUP_SCRIPT="${SETUP_SCRIPT}" RUN_DIR="${run_dir}" RUN_BIN="${run_bin}" bash <<'EOF'
set -euo pipefail
source "${SETUP_SCRIPT}"

case ":${PATH}:" in
  *":${RUN_BIN}:"*) ;;
  *)
    printf 'expected run bin in PATH before cleanup: %s\n' "${PATH}" >&2
    exit 1
    ;;
esac

alias declarest-e2e >/dev/null 2>&1
rm -rf "${RUN_DIR}"
__declarest_e2e_prune_deleted_run_bins_from_path

case ":${PATH}:" in
  *":${RUN_BIN}:"*)
    printf 'expected run bin to be pruned from PATH: %s\n' "${PATH}" >&2
    exit 1
    ;;
esac

if alias declarest-e2e >/dev/null 2>&1; then
  printf 'expected declarest-e2e alias to be removed after run cleanup\n' >&2
  exit 1
fi
EOF
  )
  status=$?
  set -e
  assert_status "${status}" "0"
  [[ -z "${output}" ]] || true
}

test_manual_env_scripts_export_kubernetes_runtime_and_restore_kubeconfig() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_PLATFORM='kubernetes'
  export E2E_KUBECONFIG="${tmp}/manual-kubeconfig"
  export E2E_KIND_CLUSTER_NAME='declarest-e2e-manual'
  export E2E_K8S_NAMESPACE='declarest-manual'
  : >"${E2E_KUBECONFIG}"

  local SETUP_SCRIPT RESET_SCRIPT
  prepare_manual_env_scripts "${tmp}"

  assert_file_contains "${SETUP_SCRIPT}" "export DECLAREST_E2E_PLATFORM='kubernetes'"
  assert_file_contains "${SETUP_SCRIPT}" "export DECLAREST_E2E_KUBECONFIG="
  assert_file_contains "${SETUP_SCRIPT}" "export DECLAREST_E2E_KIND_CLUSTER='declarest-e2e-manual'"
  assert_file_contains "${SETUP_SCRIPT}" "export DECLAREST_E2E_K8S_NAMESPACE='declarest-manual'"
  assert_file_contains "${RESET_SCRIPT}" "unset DECLAREST_E2E_KUBECONFIG"
  assert_file_contains "${RESET_SCRIPT}" "unset DECLAREST_E2E_KIND_CLUSTER"
  assert_file_contains "${RESET_SCRIPT}" "unset DECLAREST_E2E_K8S_NAMESPACE"

  local output status
  set +e
  output=$(
    SETUP_SCRIPT="${SETUP_SCRIPT}" RESET_SCRIPT="${RESET_SCRIPT}" ORIGINAL_KUBECONFIG="${tmp}/original-kubeconfig" bash <<'EOF'
set -euo pipefail
export KUBECONFIG="${ORIGINAL_KUBECONFIG}"
source "${SETUP_SCRIPT}"

[[ "${DECLAREST_E2E_PLATFORM}" == 'kubernetes' ]]
[[ "${KUBECONFIG}" == "${DECLAREST_E2E_KUBECONFIG}" ]]

source "${RESET_SCRIPT}"
[[ "${KUBECONFIG}" == "${ORIGINAL_KUBECONFIG}" ]]
[[ -z "${DECLAREST_E2E_PLATFORM:-}" ]]
EOF
  )
  status=$?
  set -e
  assert_status "${status}" "0"
  [[ -z "${output}" ]] || true
}

test_manual_handoff_prints_kubectl_and_repo_provider_access() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_PLATFORM='kubernetes'
  export E2E_KUBECONFIG="${tmp}/manual-kubeconfig"
  export E2E_KIND_CLUSTER_NAME='declarest-e2e-manual'
  export E2E_K8S_NAMESPACE='declarest-manual'
  export E2E_REPO_TYPE='git'
  export E2E_GIT_PROVIDER='gitea'
  export E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_COMPONENT_REPO_PROVIDER_LOGIN_PATH=()
  E2E_COMPONENT_REPO_PROVIDER_LOGIN_PATH['git-provider:gitea']='/user/login'
  : >"${E2E_KUBECONFIG}"

  local SETUP_SCRIPT RESET_SCRIPT
  prepare_manual_env_scripts "${tmp}"

  local provider_state="${E2E_STATE_DIR}/git-provider-gitea.env"
  : >"${provider_state}"
  e2e_write_state_value "${provider_state}" 'GIT_REMOTE_URL' 'http://127.0.0.1:3000/declarest-e2e/declarest-e2e.git'
  e2e_write_state_value "${provider_state}" 'REPO_PROVIDER_BASE_URL' 'http://127.0.0.1:3000'
  e2e_write_state_value "${provider_state}" 'GIT_AUTH_USERNAME' 'gitea-admin'
  e2e_write_state_value "${provider_state}" 'GIT_AUTH_PASSWORD' 'gitea-pass'
  export E2E_MANUAL_COMPONENT_ACCESS_OUTPUT=$'managed-server:simple-api-server\n  Base URL: http://127.0.0.1:20890/api\n  Auth Mode: oauth2'

  local output
  output=$(e2e_manual_handoff_print 'e2e-manual')

  assert_contains "${output}" "How to connect kubectl to this kind cluster:"
  assert_contains "${output}" "export KUBECONFIG=\"${E2E_KUBECONFIG}\""
  assert_contains "${output}" "Manual Component Access:"
  assert_contains "${output}" "managed-server:simple-api-server"
  assert_contains "${output}" "Base URL: http://127.0.0.1:20890/api"
  assert_contains "${output}" "Repository provider access:"
  assert_contains "${output}" "provider: gitea (local)"
  assert_contains "${output}" "web login: http://127.0.0.1:3000/user/login"
  assert_contains "${output}" "username: gitea-admin"
  assert_contains "${output}" "password: gitea-pass"

  local manual_line repo_line
  manual_line=$(printf '%s\n' "${output}" | grep -n 'Manual Component Access:' | head -n 1 | cut -d: -f1 || true)
  repo_line=$(printf '%s\n' "${output}" | grep -n 'Repository provider access:' | head -n 1 | cut -d: -f1 || true)
  if [[ -z "${manual_line}" || -z "${repo_line}" ]] || ((manual_line >= repo_line)); then
    fail 'expected Manual Component Access section before Repository provider access'
  fi
}

test_managed_server_access_details_formats_generic_state() {
  load_profile_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_STATE_DIR="${tmp}/state"
  export E2E_MANAGED_SERVER='rundeck'
  export E2E_MANAGED_SERVER_CONNECTION='local'
  mkdir -p "${E2E_STATE_DIR}"

  local state_file="${E2E_STATE_DIR}/managed-server-rundeck.env"
  : >"${state_file}"
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_BASE_URL' 'http://127.0.0.1:24444'
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_API_BASE_URL' 'http://127.0.0.1:24444/api/45'
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_WEB_LOGIN_URL' 'http://127.0.0.1:24444/user/login'
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_AUTH_MODE' 'custom-header'
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_USERNAME' 'admin'
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_PASSWORD' 'admin-pass'
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_HEADER' 'X-Rundeck-Auth-Token'
  e2e_write_state_value "${state_file}" 'MANAGED_SERVER_ACCESS_TOKEN' 'rundeck-token'

  local output
  output=$(e2e_profile_managed_server_access_details)

  assert_contains "${output}" "Base URL: http://127.0.0.1:24444"
  assert_contains "${output}" "API Base URL: http://127.0.0.1:24444/api/45"
  assert_contains "${output}" "Web Login: http://127.0.0.1:24444/user/login"
  assert_contains "${output}" "Auth Mode: custom-header"
  assert_contains "${output}" "Username: admin"
  assert_contains "${output}" "Password: admin-pass"
  assert_contains "${output}" "Header: X-Rundeck-Auth-Token"
  assert_contains "${output}" "Token: rundeck-token"
}

test_manual_env_scripts_install_and_restore_prompt_hook
test_manual_env_scripts_enable_and_reset_prompt_auth_session
test_manual_env_scripts_skip_local_prompt_proxy_auth_exports
test_manual_env_prompt_hook_prunes_deleted_run_bin_path_and_alias
test_manual_env_scripts_export_kubernetes_runtime_and_restore_kubeconfig
test_manual_handoff_prints_kubectl_and_repo_provider_access
test_managed_server_access_details_formats_generic_state
