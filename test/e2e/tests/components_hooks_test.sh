#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_hook_libs() {
  source_e2e_lib "common"
  source_e2e_lib "components"
}

prepare_runtime_globals() {
  local tmp=$1
  E2E_RUN_ID=test-hooks
  E2E_RUN_DIR="${tmp}/run"
  E2E_STATE_DIR="${tmp}/state"
  E2E_LOG_DIR="${tmp}/logs"
  E2E_CONTEXT_DIR="${tmp}/context"
  E2E_CONTEXT_FILE="${tmp}/contexts.yaml"
  E2E_METADATA_DIR=''
  E2E_RESOURCE_SERVER='demo'
  E2E_RESOURCE_SERVER_CONNECTION='local'
  E2E_RESOURCE_SERVER_BASIC_AUTH='false'
  E2E_RESOURCE_SERVER_OAUTH2='true'
  E2E_RESOURCE_SERVER_MTLS='false'
  E2E_REPO_TYPE='filesystem'
  E2E_GIT_PROVIDER=''
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_SECRET_PROVIDER='none'
  E2E_SECRET_PROVIDER_CONNECTION='local'
  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}"
}

create_hook_component() {
  local root=$1
  local key=$2
  local hook_body=$3
  local type=${key%%:*}
  local name=${key#*:}
  local dir="${root}/${type}/${name}"
  mkdir -p "${dir}/scripts"
  cat >"${dir}/scripts/init.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
${hook_body}
EOF
  chmod +x "${dir}/scripts/init.sh"
  printf '%s\n' "${dir}"
}

test_dependency_ordering_respects_dependencies() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local order_file="${tmp}/order.log"
  local path_a path_b
  path_a=$(create_hook_component "${tmp}/components" "resource-server:alpha" "printf '%s\\n' \"\${E2E_COMPONENT_KEY}\" >> ${order_file@Q}")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "printf '%s\\n' \"\${E2E_COMPONENT_KEY}\" >> ${order_file@Q}")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("resource-server:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["resource-server:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["resource-server:alpha"]=""
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]="resource-server:alpha"
  E2E_COMPONENT_RUNTIME_KIND["resource-server:alpha"]='native'
  E2E_COMPONENT_RUNTIME_KIND["repo-type:filesystem"]='native'

  e2e_components_run_hook_for_keys init false "${E2E_SELECTED_COMPONENT_KEYS[@]}"

  local actual
  actual=$(tr '\n' ' ' <"${order_file}" | sed 's/[[:space:]]\+$//')
  assert_eq "${actual}" "resource-server:alpha repo-type:filesystem" "expected dependency-ordered hook execution"
}

test_cycle_detection_fails_with_actionable_message() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local path_a path_b
  path_a=$(create_hook_component "${tmp}/components" "resource-server:alpha" "exit 0")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "exit 0")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("resource-server:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["resource-server:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["resource-server:alpha"]="repo-type:filesystem"
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]="resource-server:alpha"
  E2E_COMPONENT_RUNTIME_KIND["resource-server:alpha"]='native'
  E2E_COMPONENT_RUNTIME_KIND["repo-type:filesystem"]='native'

  local output status
  set +e
  output=$(e2e_components_run_hook_for_keys init false "${E2E_SELECTED_COMPONENT_KEYS[@]}" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "dependency cycle detected while running hook init"
}

test_parallel_hook_failures_retain_component_logs_in_run_artifacts() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local path_a path_b
  path_a=$(create_hook_component "${tmp}/components" "resource-server:alpha" "printf 'alpha hook ran\\n'; exit 0")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "printf 'beta hook failed\\n' >&2; exit 23")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("resource-server:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["resource-server:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["resource-server:alpha"]=""
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]=""
  E2E_COMPONENT_RUNTIME_KIND["resource-server:alpha"]='native'
  E2E_COMPONENT_RUNTIME_KIND["repo-type:filesystem"]='native'

  local output status
  set +e
  output=$(e2e_components_run_hook_for_keys init true "${E2E_SELECTED_COMPONENT_KEYS[@]}" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "parallel hook logs retained dir="

  local hook_logs_root="${E2E_LOG_DIR}/component-hooks"
  assert_path_exists "${hook_logs_root}"

  local log_count
  log_count=$(find "${hook_logs_root}" -type f -name '*.log' | wc -l | tr -d '[:space:]')
  if ((log_count < 2)); then
    fail "expected at least 2 retained hook logs, got ${log_count}"
  fi

  local beta_log
  beta_log=$(find "${hook_logs_root}" -type f -name 'repo-type-filesystem.log' | head -n 1)
  [[ -n "${beta_log}" ]] || fail "expected retained log for repo-type:filesystem"
  assert_file_contains "${beta_log}" "beta hook failed"
}

test_dependency_ordering_respects_dependencies
test_cycle_detection_fails_with_actionable_message
test_parallel_hook_failures_retain_component_logs_in_run_artifacts
