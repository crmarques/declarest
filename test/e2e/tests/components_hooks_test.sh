#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_hook_libs() {
  source_e2e_lib "common"
  source_e2e_lib "components"
  source_e2e_lib "components_hooks"
}

prepare_runtime_globals() {
  local tmp=$1
  E2E_RUN_ID=test-hooks
  E2E_RUN_DIR="${tmp}/run"
  E2E_STATE_DIR="${tmp}/state"
  E2E_LOG_DIR="${tmp}/logs"
  E2E_CONTEXT_DIR="${tmp}/context"
  E2E_CONTEXT_FILE="${tmp}/contexts.yaml"
  E2E_BUILD_CACHE_DIR="${tmp}/build-cache"
  E2E_LOCKS_DIR="${tmp}/locks"
  E2E_METADATA_DIR=''
  E2E_METADATA_BUNDLE=''
  E2E_METADATA='bundle'
  E2E_MANAGED_SERVICE='demo'
  E2E_MANAGED_SERVICE_CONNECTION='local'
  E2E_MANAGED_SERVICE_AUTH_TYPE='oauth2'
  E2E_MANAGED_SERVICE_MTLS='false'
  E2E_REPO_TYPE='filesystem'
  E2E_GIT_PROVIDER=''
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_SECRET_PROVIDER='none'
  E2E_SECRET_PROVIDER_CONNECTION='local'
  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}" "${E2E_BUILD_CACHE_DIR}" "${E2E_LOCKS_DIR}"
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
printf 'COMPONENT_KEY=%s\n' "\${E2E_COMPONENT_KEY}" >"\${E2E_COMPONENT_STATE_FILE}"
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
  path_a=$(create_hook_component "${tmp}/components" "managed-service:alpha" "printf '%s\\n' \"\${E2E_COMPONENT_KEY}\" >> ${order_file@Q}")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "printf '%s\\n' \"\${E2E_COMPONENT_KEY}\" >> ${order_file@Q}")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["managed-service:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["managed-service:alpha"]=""
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]="managed-service:alpha"
  E2E_COMPONENT_RUNTIME_KIND["managed-service:alpha"]='native'
  E2E_COMPONENT_RUNTIME_KIND["repo-type:filesystem"]='native'

  e2e_components_run_hook_for_keys init false "${E2E_SELECTED_COMPONENT_KEYS[@]}"

  local actual
  actual=$(tr '\n' ' ' <"${order_file}" | sed 's/[[:space:]]\+$//')
  assert_eq "${actual}" "managed-service:alpha repo-type:filesystem" "expected dependency-ordered hook execution"
}

test_cycle_detection_fails_with_actionable_message() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local path_a path_b
  path_a=$(create_hook_component "${tmp}/components" "managed-service:alpha" "exit 0")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "exit 0")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["managed-service:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["managed-service:alpha"]="repo-type:filesystem"
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]="managed-service:alpha"
  E2E_COMPONENT_RUNTIME_KIND["managed-service:alpha"]='native'
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
  path_a=$(create_hook_component "${tmp}/components" "managed-service:alpha" "printf 'alpha hook ran\\n'; exit 0")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "printf 'beta hook failed\\n' >&2; exit 23")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["managed-service:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["managed-service:alpha"]=""
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]=""
  E2E_COMPONENT_RUNTIME_KIND["managed-service:alpha"]='native'
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

test_prepare_metadata_workspace_uses_component_metadata_for_dir_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_MANAGED_SERVICE='keycloak'
  E2E_METADATA='dir'
  E2E_COMPONENT_PATH=()
  local component_dir="${tmp}/components/managed-service/keycloak"
  local component_metadata="${component_dir}/metadata"
  mkdir -p "${component_metadata}"
  E2E_COMPONENT_PATH['managed-service:keycloak']="${component_dir}"

  e2e_prepare_metadata_workspace

  assert_eq "${E2E_METADATA_BUNDLE:-}" "" "expected metadata bundle to stay unset for dir mode"
  assert_eq "${E2E_METADATA_DIR}" "${E2E_RUN_DIR}/managed-service-metadata" "expected metadata dir to use run workspace copy"
  assert_path_exists "${E2E_METADATA_DIR}"
  assert_eq "$(cd "${E2E_METADATA_DIR}" && pwd)" "$(cd "${E2E_RUN_DIR}/managed-service-metadata" && pwd)" "expected deterministic metadata workspace path"
}

test_prepare_metadata_workspace_uses_keycloak_bundle_for_bundle_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_MANAGED_SERVICE='keycloak'
  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_METADATA_BUNDLE_REF=()
  E2E_COMPONENT_PATH['managed-service:keycloak']="${tmp}/components/managed-service/keycloak"
  E2E_COMPONENT_METADATA_BUNDLE_REF['managed-service:keycloak']='keycloak-bundle:0.0.1'

  e2e_prepare_metadata_workspace

  assert_eq "${E2E_METADATA_BUNDLE}" "keycloak-bundle:0.0.1" "expected keycloak shorthand metadata bundle"
  assert_eq "${E2E_METADATA_DIR:-}" "" "expected metadata workspace dir to stay unset when bundle is selected"
}

test_prepare_metadata_workspace_falls_back_to_component_metadata_when_bundle_mapping_is_missing() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_MANAGED_SERVICE='rundeck'
  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_METADATA_BUNDLE_REF=()
  local component_dir="${tmp}/components/managed-service/rundeck"
  local component_metadata="${component_dir}/metadata"
  mkdir -p "${component_metadata}"
  E2E_COMPONENT_PATH['managed-service:rundeck']="${component_dir}"

  e2e_prepare_metadata_workspace

  assert_eq "${E2E_METADATA_BUNDLE:-}" "" "expected unsupported bundle mode to keep metadata bundle unset"
  assert_eq "${E2E_METADATA_DIR}" "${E2E_RUN_DIR}/managed-service-metadata" "expected unsupported bundle mode to fall back to run workspace copy"
  assert_path_exists "${E2E_METADATA_DIR}"
}

test_prepare_metadata_workspace_allows_bundle_mode_without_mapping() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_MANAGED_SERVICE='simple-api-server'
  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_METADATA_BUNDLE_REF=()
  E2E_COMPONENT_PATH['managed-service:simple-api-server']="${tmp}/components/managed-service/simple-api-server"

  e2e_prepare_metadata_workspace
  assert_eq "${E2E_METADATA_BUNDLE:-}" "" "expected unsupported bundle mode to keep metadata bundle unset"
  assert_eq "${E2E_METADATA_DIR:-}" "" "expected unsupported bundle mode to keep metadata dir unset"
}

context_metadata_line() {
  local fragment=$1
  awk '/^metadata:/{getline; print; exit}' "${fragment}"
}

run_repo_context_script() {
  local script_path=$1
  local state_file=$2
  local fragment_file=$3
  local metadata_bundle=${4:-}
  local metadata_dir=${5:-}

  E2E_COMPONENT_STATE_FILE="${state_file}" \
    E2E_COMPONENT_CONTEXT_FRAGMENT="${fragment_file}" \
    E2E_METADATA_BUNDLE="${metadata_bundle}" \
    E2E_METADATA_DIR="${metadata_dir}" \
    bash "${script_path}"
}

test_repo_context_scripts_emit_metadata_bundle_when_set() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local state_file="${tmp}/state.env"
  local repo_dir="${tmp}/repo"
  local metadata_dir="${tmp}/metadata"
  mkdir -p "${repo_dir}" "${metadata_dir}"

  cat >"${state_file}" <<EOF
REPO_BASE_DIR=${repo_dir}
GIT_REMOTE_URL=https://example.com/acme/repo.git
GIT_REMOTE_PROVIDER=github
GIT_REMOTE_BRANCH=main
EOF

  local script_path
  for script_path in \
    "${E2E_SCRIPT_DIR}/components/repo-type/filesystem/scripts/context.sh" \
    "${E2E_SCRIPT_DIR}/components/repo-type/git/scripts/context.sh"; do
    local component_name
    component_name=$(basename "$(dirname "$(dirname "${script_path}")")")
    local fragment_file="${tmp}/${component_name}.yaml"

    run_repo_context_script "${script_path}" "${state_file}" "${fragment_file}" "keycloak-bundle:0.0.1" ""
    assert_eq "$(context_metadata_line "${fragment_file}")" "  bundle: keycloak-bundle:0.0.1" "expected ${script_path} to emit metadata.bundle"

    run_repo_context_script "${script_path}" "${state_file}" "${fragment_file}" "" "${metadata_dir}"
    assert_eq "$(context_metadata_line "${fragment_file}")" "  baseDir: ${metadata_dir}" "expected ${script_path} to emit metadata.baseDir fallback"
  done
}

test_forward_proxy_prompt_auth_prints_credentials_without_helper_file() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_PLATFORM='compose'
  E2E_CONTAINER_ENGINE='podman'
  E2E_PROXY_MODE='local'
  E2E_PROXY_AUTH_TYPE='prompt'
  E2E_PROXY_AUTH_USERNAME=''
  E2E_PROXY_AUTH_PASSWORD=''

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=('proxy:forward-proxy')
  E2E_COMPONENT_PATH['proxy:forward-proxy']="${E2E_SCRIPT_DIR}/components/proxy/forward-proxy"
  E2E_COMPONENT_DEPENDS_ON['proxy:forward-proxy']=''
  E2E_COMPONENT_RUNTIME_KIND['proxy:forward-proxy']='compose'

  e2e_component_run_hook 'proxy:forward-proxy' init

  local state_file username password helper_file output
  state_file=$(e2e_component_state_file 'proxy:forward-proxy')
  username=$(e2e_state_get "${state_file}" 'PROXY_AUTH_USERNAME')
  password=$(e2e_state_get "${state_file}" 'PROXY_AUTH_PASSWORD')
  helper_file=$(e2e_state_get "${state_file}" 'PROXY_PROMPT_HELPER_FILE' 2>/dev/null || true)

  assert_eq "$(e2e_state_get "${state_file}" 'PROXY_AUTH_TYPE')" "prompt" "expected prompt proxy auth type to persist in state"
  assert_eq "${username}" "declarest-e2e-proxy" "expected generated prompt username"
  [[ -n "${password}" ]] || fail 'expected generated prompt password'
  [[ -z "${helper_file}" ]] || fail "expected no prompt helper file, got ${helper_file}"

  output=$(
    E2E_COMPONENT_STATE_FILE="${state_file}" \
      bash "${E2E_SCRIPT_DIR}/components/proxy/forward-proxy/scripts/manual-info.sh"
  )
  assert_contains "${output}" "Proxy auth username: ${username}"
  assert_contains "${output}" "Proxy auth password: ${password}"
  assert_not_contains "${output}" "Prompt helper:"
}

create_openapi_component() {
  local root=$1
  local key=$2
  local type=${key%%:*}
  local name=${key#*:}
  local dir="${root}/${type}/${name}"
  mkdir -p "${dir}"
  cat >"${dir}/openapi.yaml" <<'EOF'
openapi: 3.0.0
paths: {}
EOF
  printf '%s\n' "${dir}"
}

test_prepare_component_openapi_specs_skips_local_openapi_for_bundle_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local component_dir
  component_dir=$(create_openapi_component "${tmp}/components" "managed-service:demo")

  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:demo")
  E2E_COMPONENT_PATH['managed-service:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC['managed-service:demo']:-}" ]]; then
    fail "expected bundle mode to skip local openapi spec wiring"
  fi
}

test_prepare_component_openapi_specs_keeps_local_openapi_for_dir_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local component_dir
  component_dir=$(create_openapi_component "${tmp}/components" "managed-service:demo")

  E2E_METADATA='dir'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:demo")
  E2E_COMPONENT_PATH['managed-service:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  local copied_spec="${E2E_COMPONENT_OPENAPI_SPEC['managed-service:demo']:-}"
  [[ -n "${copied_spec}" ]] || fail "expected dir mode to wire local openapi spec"
  assert_path_exists "${copied_spec}"
}

test_keycloak_configure_auth_script_uses_current_client_id_fields() {
  local script_path="${E2E_SCRIPT_DIR}/components/managed-service/keycloak/scripts/configure-auth.sh"
  local content
  content=$(<"${script_path}")

  assert_contains "${content}" 'clients?clientId=${client_id}' "expected keycloak configure-auth query filter to use clientId"
  assert_contains "${content}" 'clientId:$client_id' "expected keycloak configure-auth payload to use clientId"
}

test_prepare_component_openapi_specs_defaults_to_bundle_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local component_dir
  component_dir=$(create_openapi_component "${tmp}/components" "managed-service:demo")

  unset E2E_METADATA
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:demo")
  E2E_COMPONENT_PATH['managed-service:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC['managed-service:demo']:-}" ]]; then
    fail "expected default metadata type to skip local openapi spec wiring"
  fi
}

test_component_compose_file_resolves_compose_subdir() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-service:demo']="${tmp}/components/managed-service/demo"

  local compose_file
  compose_file=$(e2e_component_compose_file 'managed-service:demo')
  assert_eq "${compose_file}" "${tmp}/components/managed-service/demo/compose/compose.yaml" "expected compose artifact path to use compose/compose.yaml"
}

test_k8s_port_forward_pid_tracking_and_stop() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"

  local fake_log="${tmp}/kubectl.log"
  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == *" get svc "* ]]; then
  cat <<'JSON'
{"items":[{"metadata":{"name":"demo-service","annotations":{"declarest.e2e/port-forward":"18081:8080,18082:8081"}}}]}
JSON
  exit 0
fi

if [[ "$*" == *" port-forward "* ]]; then
  if [[ -n "${FAKE_KUBECTL_LOG:-}" ]]; then
    printf '%s\n' "$*" >>"${FAKE_KUBECTL_LOG}"
  fi
  trap 'exit 0' TERM INT
  while true; do
    sleep 1
  done
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-service:demo']="${tmp}/components/managed-service/demo"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-service:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KUBECTL_LOG="${fake_log}"

  e2e_component_start_k8s_port_forwards 'managed-service:demo' "${state_file}"

  local pids
  pids=$(e2e_state_get "${state_file}" 'K8S_PORT_FORWARD_PIDS')
  [[ -n "${pids}" ]] || fail "expected k8s port-forward pids to be persisted"

  local -a pid_array=()
  read -r -a pid_array <<<"${pids}"
  if ((${#pid_array[@]} != 2)); then
    fail "expected exactly 2 port-forward pids, got ${#pid_array[@]}"
  fi

  local pid
  for pid in "${pid_array[@]}"; do
    kill -0 "${pid}" >/dev/null 2>&1 || fail "expected port-forward pid to be alive: ${pid}"
  done

  e2e_component_builtin_stop_kubernetes 'managed-service:demo'

  for pid in "${pid_array[@]}"; do
    for _ in $(seq 1 20); do
      if ! kill -0 "${pid}" >/dev/null 2>&1; then
        break
      fi
      sleep 0.1
    done
    if kill -0 "${pid}" >/dev/null 2>&1; then
      fail "expected port-forward pid to stop: ${pid}"
    fi
  done

  assert_file_contains "${fake_log}" "port-forward service/demo-service 18081:8080"
  assert_file_contains "${fake_log}" "port-forward service/demo-service 18082:8081"
}

test_k8s_port_forward_retries_after_runtime_disconnect() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"

  local fake_log="${tmp}/kubectl.log"
  local counter_file="${tmp}/port-forward-count"
  printf '0\n' >"${counter_file}"

  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == *" get svc "* ]]; then
  cat <<'JSON'
{"items":[{"metadata":{"name":"demo-service","annotations":{"declarest.e2e/port-forward":"18081:8080"}}}]}
JSON
  exit 0
fi

if [[ "$*" == *" port-forward "* ]]; then
  count=$(cat "${FAKE_KUBECTL_COUNTER}")
  count=$((count + 1))
  printf '%s\n' "${count}" >"${FAKE_KUBECTL_COUNTER}"
  if ((count == 1)); then
    sleep 2
    exit 1
  fi

  if [[ -n "${FAKE_KUBECTL_LOG:-}" ]]; then
    printf '%s\n' "$*" >>"${FAKE_KUBECTL_LOG}"
  fi
  trap 'exit 0' TERM INT
  while true; do
    sleep 1
  done
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-service:demo']="${tmp}/components/managed-service/demo"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-service:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KUBECTL_LOG="${fake_log}"
  export FAKE_KUBECTL_COUNTER="${counter_file}"

  e2e_component_start_k8s_port_forwards 'managed-service:demo' "${state_file}"

  local pids
  pids=$(e2e_state_get "${state_file}" 'K8S_PORT_FORWARD_PIDS')
  [[ -n "${pids}" ]] || fail "expected k8s port-forward pid to be persisted"

  local wrapper_script
  wrapper_script=$(find "${E2E_LOG_DIR}" -maxdepth 1 -name 'port-forward-*.sh' | head -n 1)
  [[ -n "${wrapper_script}" ]] || fail "expected generated port-forward wrapper script"
  assert_file_contains "${wrapper_script}" 'port_forward_pid=$!'
  assert_file_contains "${wrapper_script}" 'rc=$?'

  local -a pid_array=()
  read -r -a pid_array <<<"${pids}"
  if ((${#pid_array[@]} != 1)); then
    fail "expected exactly 1 port-forward pid, got ${#pid_array[@]}"
  fi

  local restart_count=0
  local _
  for _ in $(seq 1 80); do
    restart_count=$(cat "${counter_file}")
    if ((restart_count >= 2)); then
      break
    fi
    sleep 0.1
  done
  if ((restart_count < 2)); then
    fail "expected port-forward wrapper to retry after disconnect, attempts=${restart_count}"
  fi

  kill -0 "${pid_array[0]}" >/dev/null 2>&1 || fail "expected restarted port-forward pid to stay alive"

  e2e_component_builtin_stop_kubernetes 'managed-service:demo'
  for _ in $(seq 1 20); do
    if ! kill -0 "${pid_array[0]}" >/dev/null 2>&1; then
      break
    fi
    sleep 0.1
  done
  if kill -0 "${pid_array[0]}" >/dev/null 2>&1; then
    fail "expected restarted port-forward pid to stop"
  fi
}

test_k8s_component_start_preloads_unique_images_before_apply() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local podman_log="${tmp}/podman.log"
  local kind_log="${tmp}/kind.log"
  local kubectl_log="${tmp}/kubectl.log"

  cat >"${fake_bin}/podman" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == image\ exists* ]]; then
  exit 1
fi

if [[ "$*" == image\ inspect* ]]; then
  printf '%s\n' 'sha256:gitea-test-id'
  exit 0
fi

if [[ "${1:-}" == 'pull' ]]; then
  printf 'pull %s\n' "${2:-}" >>"${FAKE_PODMAN_LOG}"
  exit 0
fi

if [[ "${1:-}" == 'save' ]]; then
  out=''
  image=''
  shift
  while (($# > 0)); do
    case "$1" in
      -o)
        out=$2
        shift 2
        ;;
      *)
        image=$1
        shift
        ;;
    esac
  done
  [[ -n "${out}" ]] || exit 1
  : >"${out}"
  printf 'save %s %s\n' "${image}" "${out}" >>"${FAKE_PODMAN_LOG}"
  exit 0
fi

printf 'unexpected podman invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/podman"

  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${FAKE_KIND_LOG}"
exit 0
EOF
  chmod +x "${fake_bin}/kind"

  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${FAKE_KUBECTL_LOG}"

if [[ "$*" == *" get svc "* ]]; then
  cat <<'JSON'
{"items":[]}
JSON
  exit 0
fi

if [[ "$*" == *" apply "* || "$*" == *" wait "* ]]; then
  exit 0
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  local component_dir_one="${tmp}/components/managed-service/demo-a"
  local component_dir_two="${tmp}/components/secret-provider/demo-b"
  mkdir -p "${component_dir_one}/k8s" "${component_dir_two}/k8s"

  cat >"${component_dir_one}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: managed-service-demo-a
spec:
  template:
    spec:
      containers:
        - name: app
          image: docker.io/gitea/gitea:1.25.4
EOF

  cat >"${component_dir_two}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secret-provider-demo-b
spec:
  template:
    spec:
      containers:
        - name: app
          image: docker.io/gitea/gitea:1.25.4
EOF

  E2E_PLATFORM='kubernetes'
  E2E_CONTAINER_ENGINE='podman'
  E2E_KIND_CLUSTER_NAME='declarest-e2e-hooks'
  E2E_K8S_NAMESPACE='declarest-tests'
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  : >"${E2E_KUBECONFIG}"

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:demo-a" "secret-provider:demo-b")

  E2E_COMPONENT_PATH['managed-service:demo-a']="${component_dir_one}"
  E2E_COMPONENT_PATH['secret-provider:demo-b']="${component_dir_two}"
  E2E_COMPONENT_DEPENDS_ON['managed-service:demo-a']=''
  E2E_COMPONENT_DEPENDS_ON['secret-provider:demo-b']=''
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo-a']='compose'
  E2E_COMPONENT_RUNTIME_KIND['secret-provider:demo-b']='compose'

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_PODMAN_LOG="${podman_log}"
  export FAKE_KIND_LOG="${kind_log}"
  export FAKE_KUBECTL_LOG="${kubectl_log}"

  e2e_components_start_local

  local pull_count save_count load_count
  pull_count=$(grep -c '^pull docker.io/gitea/gitea:1.25.4$' "${podman_log}" || true)
  save_count=$(grep -c '^save docker.io/gitea/gitea:1.25.4 ' "${podman_log}" || true)
  load_count=$(grep -c '^load image-archive .*/k8s-image-cache/docker.io_gitea_gitea_1.25.4.tar --name declarest-e2e-hooks$' "${kind_log}" || true)

  assert_eq "${pull_count}" "1" "expected one image pull for duplicated image references"
  assert_eq "${save_count}" "1" "expected one image export for duplicated image references"
  assert_eq "${load_count}" "1" "expected one kind image load for duplicated image references"
  assert_file_contains "${kubectl_log}" "apply -f ${E2E_STATE_DIR}/k8s-rendered/managed-service-demo-a/deployment.yaml"
  assert_file_contains "${kubectl_log}" "apply -f ${E2E_STATE_DIR}/k8s-rendered/secret-provider-demo-b/deployment.yaml"
}

test_k8s_component_start_reuses_cached_exported_archives_across_runs() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local podman_log="${tmp}/podman.log"
  local kind_log="${tmp}/kind.log"
  local kubectl_log="${tmp}/kubectl.log"

  cat >"${fake_bin}/podman" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == image\ exists* ]]; then
  exit 0
fi

if [[ "$*" == image\ inspect* ]]; then
  printf '%s\n' 'sha256:rundeck-test-id'
  exit 0
fi

if [[ "${1:-}" == 'save' ]]; then
  out=''
  image=''
  shift
  while (($# > 0)); do
    case "$1" in
      -o)
        out=$2
        shift 2
        ;;
      *)
        image=$1
        shift
        ;;
    esac
  done
  [[ -n "${out}" ]] || exit 1
  : >"${out}"
  printf 'save %s %s\n' "${image}" "${out}" >>"${FAKE_PODMAN_LOG}"
  exit 0
fi

printf 'unexpected podman invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/podman"

  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${FAKE_KIND_LOG}"
exit 0
EOF
  chmod +x "${fake_bin}/kind"

  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${FAKE_KUBECTL_LOG}"

if [[ "$*" == *" get svc "* ]]; then
  cat <<'JSON'
{"items":[]}
JSON
  exit 0
fi

if [[ "$*" == *" apply "* || "$*" == *" wait "* ]]; then
  exit 0
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  local component_dir="${tmp}/components/managed-service/demo"
  mkdir -p "${component_dir}/k8s"
  cat >"${component_dir}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: managed-service-demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: docker.io/rundeck/rundeck:5.19.0
EOF

  E2E_PLATFORM='kubernetes'
  E2E_CONTAINER_ENGINE='podman'
  E2E_KIND_CLUSTER_NAME='declarest-e2e-hooks-a'
  E2E_K8S_NAMESPACE='declarest-tests-a'
  E2E_KUBECONFIG="${tmp}/kubeconfig-a"
  : >"${E2E_KUBECONFIG}"

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-service:demo")
  E2E_COMPONENT_PATH['managed-service:demo']="${component_dir}"
  E2E_COMPONENT_DEPENDS_ON['managed-service:demo']=''
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo']='compose'

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_PODMAN_LOG="${podman_log}"
  export FAKE_KIND_LOG="${kind_log}"
  export FAKE_KUBECTL_LOG="${kubectl_log}"

  e2e_components_start_local

  E2E_RUN_ID='test-hooks-second'
  E2E_RUN_DIR="${tmp}/run-second"
  E2E_STATE_DIR="${tmp}/state-second"
  E2E_LOG_DIR="${tmp}/logs-second"
  E2E_CONTEXT_DIR="${tmp}/context-second"
  E2E_CONTEXT_FILE="${tmp}/contexts-second.yaml"
  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}"

  E2E_KIND_CLUSTER_NAME='declarest-e2e-hooks-b'
  E2E_K8S_NAMESPACE='declarest-tests-b'
  E2E_KUBECONFIG="${tmp}/kubeconfig-b"
  : >"${E2E_KUBECONFIG}"

  e2e_components_start_local

  local save_count load_count
  save_count=$(grep -c '^save docker.io/rundeck/rundeck:5.19.0 ' "${podman_log}" || true)
  load_count=$(grep -c '^load image-archive .*/k8s-image-cache/docker.io_rundeck_rundeck_5.19.0.tar --name declarest-e2e-hooks-' "${kind_log}" || true)

  assert_eq "${save_count}" "1" "expected exported image archive to be reused across runs"
  assert_eq "${load_count}" "2" "expected cached archive to still be loaded into each kind cluster"
}

test_kubernetes_runtime_retries_retryable_kind_bootstrap_failure() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local kind_log="${tmp}/kind.log"
  local kind_counter="${tmp}/kind.create.count"
  printf '0\n' >"${kind_counter}"

  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'provider=%s cmd=%s\n' "${KIND_EXPERIMENTAL_PROVIDER:-}" "$*" >>"${FAKE_KIND_LOG}"

if [[ "${1:-}" == 'create' && "${2:-}" == 'cluster' ]]; then
  count=$(cat "${FAKE_KIND_COUNTER}")
  count=$((count + 1))
  printf '%s\n' "${count}" >"${FAKE_KIND_COUNTER}"
  if ((count == 1)); then
    printf '%s\n' 'ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"' >&2
    exit 1
  fi
  exit 0
fi

if [[ "${1:-}" == 'delete' && "${2:-}" == 'cluster' ]]; then
  exit 0
fi

printf 'unexpected kind invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kind"

  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == *" get namespace "* ]]; then
  exit 1
fi

if [[ "$*" == *" create namespace "* ]]; then
  exit 0
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  E2E_PLATFORM='kubernetes'
  E2E_CONTAINER_ENGINE='podman'
  E2E_RUN_ID='kind-retry'
  E2E_SELECTED_COMPONENT_KEYS=('managed-service:demo')
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo']='compose'

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KIND_LOG="${kind_log}"
  export FAKE_KIND_COUNTER="${kind_counter}"

  e2e_kubernetes_runtime_ensure

  local create_count delete_count
  create_count=$(grep -c 'cmd=create cluster ' "${kind_log}" || true)
  delete_count=$(grep -c 'cmd=delete cluster ' "${kind_log}" || true)

  assert_eq "${create_count}" "2" "expected kind create cluster to retry once"
  assert_eq "${delete_count}" "1" "expected failed cluster attempt cleanup before retry"
  assert_file_contains "${kind_log}" "provider=podman cmd=create cluster"
}

test_kubernetes_runtime_fails_after_retryable_failures_without_reuse() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local kind_log="${tmp}/kind.log"

  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'provider=%s cmd=%s\n' "${KIND_EXPERIMENTAL_PROVIDER:-}" "$*" >>"${FAKE_KIND_LOG}"

if [[ "${1:-}" == 'create' && "${2:-}" == 'cluster' ]]; then
  printf '%s\n' 'ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"' >&2
  exit 1
fi

if [[ "${1:-}" == 'delete' && "${2:-}" == 'cluster' ]]; then
  exit 0
fi

if [[ "${1:-}" == 'get' && "${2:-}" == 'clusters' ]]; then
  printf '%s\n' 'declarest-e2e-existing'
  exit 0
fi

if [[ "${1:-}" == 'export' && "${2:-}" == 'kubeconfig' ]]; then
  kubeconfig=''
  shift 2
  while (($# > 0)); do
    case "$1" in
      --kubeconfig)
        kubeconfig=$2
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done
  [[ -n "${kubeconfig}" ]] || exit 1
  : >"${kubeconfig}"
  exit 0
fi

printf 'unexpected kind invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kind"

  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == *" get namespace "* ]]; then
  exit 1
fi

if [[ "$*" == *" create namespace "* ]]; then
  exit 0
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  E2E_PLATFORM='kubernetes'
  E2E_CONTAINER_ENGINE='podman'
  E2E_RUN_ID='kind-reuse'
  E2E_SELECTED_COMPONENT_KEYS=('managed-service:demo')
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo']='compose'

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KIND_LOG="${kind_log}"

  local output status
  set +e
  output=$(e2e_kubernetes_runtime_ensure 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "failed to create cluster"
  assert_not_contains "$(cat "${kind_log}")" "cmd=get clusters"
  assert_not_contains "$(cat "${kind_log}")" "cmd=export kubeconfig"
  assert_eq "${E2E_KIND_CLUSTER_REUSED}" "0" "expected reused cluster marker to remain unset"
}

test_kubernetes_runtime_serializes_podman_kind_create() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local kind_log="${tmp}/kind.log"
  local worker="${tmp}/worker.sh"

  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == 'create' && "${2:-}" == 'cluster' ]]; then
  name=''
  shift 2
  while (($# > 0)); do
    case "$1" in
      --name)
        name=$2
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done

  if ! mkdir "${FAKE_KIND_ACTIVE_DIR}" 2>/dev/null; then
    printf 'overlap %s\n' "${name}" >>"${FAKE_KIND_LOG}"
    exit 1
  fi

  printf 'start %s\n' "${name}" >>"${FAKE_KIND_LOG}"
  sleep 0.2
  rmdir "${FAKE_KIND_ACTIVE_DIR}"
  printf 'done %s\n' "${name}" >>"${FAKE_KIND_LOG}"
  exit 0
fi

if [[ "${1:-}" == 'delete' && "${2:-}" == 'cluster' ]]; then
  exit 0
fi

printf 'unexpected kind invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kind"

  cat >"${worker}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cluster_name=$1

export E2E_RUNS_DIR="${WORKER_RUNS_DIR}"
export E2E_LOCKS_DIR="${WORKER_LOCKS_DIR}"
export E2E_CONTAINER_ENGINE='podman'

source "${WORKER_REPO}/test/e2e/lib/common.sh"
source "${WORKER_REPO}/test/e2e/lib/components_runtime.sh"

mkdir -p "${WORKER_TMP}/${cluster_name}"
: >"${WORKER_TMP}/${cluster_name}/kind-config.yaml"

e2e_kind_create_cluster_with_retry \
  "${cluster_name}" \
  "${WORKER_TMP}/${cluster_name}/kubeconfig" \
  "${WORKER_TMP}/${cluster_name}/kind-config.yaml" \
  '5s'
EOF
  chmod +x "${worker}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KIND_LOG="${kind_log}"
  export FAKE_KIND_ACTIVE_DIR="${tmp}/kind-active"
  export WORKER_REPO="${E2E_ROOT_DIR}"
  export WORKER_RUNS_DIR="${tmp}/runs"
  export WORKER_LOCKS_DIR="${tmp}/runs/.locks"
  export WORKER_TMP="${tmp}/workers"

  local rc_a rc_b pid_a pid_b
  set +e
  bash "${worker}" 'declarest-e2e-parallel-a' &
  pid_a=$!
  bash "${worker}" 'declarest-e2e-parallel-b' &
  pid_b=$!
  wait "${pid_a}"
  rc_a=$?
  wait "${pid_b}"
  rc_b=$?
  set -e

  assert_status "${rc_a}" "0"
  assert_status "${rc_b}" "0"
  assert_not_contains "$(cat "${kind_log}")" "overlap "
  assert_file_contains "${kind_log}" "start declarest-e2e-parallel-a"
  assert_file_contains "${kind_log}" "start declarest-e2e-parallel-b"
}

test_kubernetes_runtime_serializes_podman_kind_load_against_create() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local kind_log="${tmp}/kind.log"
  local create_worker="${tmp}/create-worker.sh"
  local load_worker="${tmp}/load-worker.sh"

  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

action="${1:-} ${2:-}"
name=''
shift $(( $# >= 2 ? 2 : $# ))
while (($# > 0)); do
  case "$1" in
    --name)
      name=$2
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

case "${action}" in
  'create cluster'|'load image-archive')
    if ! mkdir "${FAKE_KIND_ACTIVE_DIR}" 2>/dev/null; then
      printf 'overlap %s %s\n' "${action}" "${name}" >>"${FAKE_KIND_LOG}"
      exit 1
    fi

    printf 'start %s %s\n' "${action}" "${name}" >>"${FAKE_KIND_LOG}"
    sleep 0.2
    rmdir "${FAKE_KIND_ACTIVE_DIR}"
    printf 'done %s %s\n' "${action}" "${name}" >>"${FAKE_KIND_LOG}"
    exit 0
    ;;
  'delete cluster')
    exit 0
    ;;
esac

printf 'unexpected kind invocation: %s %s\n' "${action}" "${name}" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kind"

  cat >"${create_worker}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cluster_name=$1

export E2E_RUNS_DIR="${WORKER_RUNS_DIR}"
export E2E_LOCKS_DIR="${WORKER_LOCKS_DIR}"
export E2E_CONTAINER_ENGINE='podman'

source "${WORKER_REPO}/test/e2e/lib/common.sh"
source "${WORKER_REPO}/test/e2e/lib/components_runtime.sh"

mkdir -p "${WORKER_TMP}/${cluster_name}"
: >"${WORKER_TMP}/${cluster_name}/kind-config.yaml"

e2e_kind_create_cluster_with_retry \
  "${cluster_name}" \
  "${WORKER_TMP}/${cluster_name}/kubeconfig" \
  "${WORKER_TMP}/${cluster_name}/kind-config.yaml" \
  '5s'
EOF
  chmod +x "${create_worker}"

  cat >"${load_worker}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cluster_name=$1

export E2E_RUNS_DIR="${WORKER_RUNS_DIR}"
export E2E_LOCKS_DIR="${WORKER_LOCKS_DIR}"
export E2E_CONTAINER_ENGINE='podman'

source "${WORKER_REPO}/test/e2e/lib/common.sh"
source "${WORKER_REPO}/test/e2e/lib/components_runtime.sh"

mkdir -p "${WORKER_TMP}/${cluster_name}"
: >"${WORKER_TMP}/image.tar"

e2e_kind_cmd_locked load image-archive "${WORKER_TMP}/image.tar" --name "${cluster_name}"
EOF
  chmod +x "${load_worker}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KIND_LOG="${kind_log}"
  export FAKE_KIND_ACTIVE_DIR="${tmp}/kind-active"
  export WORKER_REPO="${E2E_ROOT_DIR}"
  export WORKER_RUNS_DIR="${tmp}/runs"
  export WORKER_LOCKS_DIR="${tmp}/runs/.locks"
  export WORKER_TMP="${tmp}/workers"

  local rc_create rc_load pid_create pid_load
  set +e
  bash "${create_worker}" 'declarest-e2e-create' &
  pid_create=$!
  bash "${load_worker}" 'declarest-e2e-load' &
  pid_load=$!
  wait "${pid_create}"
  rc_create=$?
  wait "${pid_load}"
  rc_load=$?
  set -e

  assert_status "${rc_create}" "0"
  assert_status "${rc_load}" "0"
  assert_not_contains "$(cat "${kind_log}")" "overlap "
  assert_file_contains "${kind_log}" "start create cluster declarest-e2e-create"
  assert_file_contains "${kind_log}" "start load image-archive declarest-e2e-load"
}

test_kubernetes_runtime_limits_active_podman_clusters() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local worker="${tmp}/worker.sh"
  local worker_a_log="${tmp}/worker-a.log"
  local worker_b_log="${tmp}/worker-b.log"
  local worker_c_log="${tmp}/worker-c.log"

  cat >"${worker}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

export E2E_RUNS_DIR="${WORKER_RUNS_DIR}"
export E2E_LOCKS_DIR="${WORKER_LOCKS_DIR}"
export E2E_CONTAINER_ENGINE='podman'
export DECLAREST_E2E_KIND_ACTIVE_CLUSTER_SLOTS='2'
export DECLAREST_E2E_KIND_ACTIVE_CLUSTER_LOCK_WAIT_SECONDS='1'

source "${WORKER_REPO}/test/e2e/lib/common.sh"
source "${WORKER_REPO}/test/e2e/lib/components_runtime.sh"

e2e_kind_active_cluster_slot_acquire
sleep 2
e2e_kind_active_cluster_slot_release
EOF
  chmod +x "${worker}"

  export WORKER_REPO="${E2E_ROOT_DIR}"
  export WORKER_RUNS_DIR="${tmp}/runs"
  export WORKER_LOCKS_DIR="${tmp}/runs/.locks"

  local rc_a rc_b rc_c pid_a pid_b
  bash "${worker}" >"${worker_a_log}" 2>&1 &
  pid_a=$!
  bash "${worker}" >"${worker_b_log}" 2>&1 &
  pid_b=$!
  sleep 0.1
  set +e
  bash "${worker}" >"${worker_c_log}" 2>&1
  rc_c=$?
  wait "${pid_a}"
  rc_a=$?
  wait "${pid_b}"
  rc_b=$?
  set -e

  assert_status "${rc_a}" "0"
  assert_status "${rc_b}" "0"
  assert_status "${rc_c}" "1"
  assert_file_contains "${worker_c_log}" "timed out waiting for active kind cluster slot"
}

test_kubernetes_runtime_kind_lock_timeout_is_configurable() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local worker="${tmp}/worker.sh"
  local worker_a_log="${tmp}/worker-a.log"
  local worker_b_log="${tmp}/worker-b.log"

  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == 'create' && "${2:-}" == 'cluster' ]]; then
  sleep 2
  exit 0
fi

if [[ "${1:-}" == 'delete' && "${2:-}" == 'cluster' ]]; then
  exit 0
fi

printf 'unexpected kind invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kind"

  cat >"${worker}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cluster_name=$1

export E2E_RUNS_DIR="${WORKER_RUNS_DIR}"
export E2E_LOCKS_DIR="${WORKER_LOCKS_DIR}"
export E2E_CONTAINER_ENGINE='podman'
export DECLAREST_E2E_KIND_CREATE_LOCK_WAIT_SECONDS='1'

source "${WORKER_REPO}/test/e2e/lib/common.sh"
source "${WORKER_REPO}/test/e2e/lib/components_runtime.sh"

mkdir -p "${WORKER_TMP}/${cluster_name}"
: >"${WORKER_TMP}/${cluster_name}/kind-config.yaml"

e2e_kind_create_cluster_with_retry \
  "${cluster_name}" \
  "${WORKER_TMP}/${cluster_name}/kubeconfig" \
  "${WORKER_TMP}/${cluster_name}/kind-config.yaml" \
  '5s'
EOF
  chmod +x "${worker}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export WORKER_REPO="${E2E_ROOT_DIR}"
  export WORKER_RUNS_DIR="${tmp}/runs"
  export WORKER_LOCKS_DIR="${tmp}/runs/.locks"
  export WORKER_TMP="${tmp}/workers"

  local rc_a rc_b pid_a
  bash "${worker}" 'declarest-e2e-timeout-a' >"${worker_a_log}" 2>&1 &
  pid_a=$!
  sleep 0.1
  set +e
  bash "${worker}" 'declarest-e2e-timeout-b' >"${worker_b_log}" 2>&1
  rc_b=$?
  wait "${pid_a}"
  rc_a=$?
  set -e

  assert_status "${rc_a}" "0"
  assert_status "${rc_b}" "1"
  assert_file_contains "${worker_b_log}" "timed out waiting for lock: kind-podman"
}

test_k8s_component_start_uses_configurable_ready_timeout() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"
  local fake_log="${tmp}/kubectl.log"

  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${FAKE_KUBECTL_LOG:-}" ]]; then
  printf '%s\n' "$*" >>"${FAKE_KUBECTL_LOG}"
fi

if [[ "$*" == *" apply "* ]]; then
  exit 0
fi

if [[ "$*" == *" wait "* ]]; then
  exit 0
fi

if [[ "$*" == *" get svc "* ]]; then
  cat <<'JSON'
{"items":[]}
JSON
  exit 0
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  local component_dir="${tmp}/components/managed-service/demo"
  mkdir -p "${component_dir}/k8s"
  cat >"${component_dir}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: managed-service-demo
EOF

  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-service:demo']="${component_dir}"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS='601'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-service:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KUBECTL_LOG="${fake_log}"

  e2e_component_builtin_start_kubernetes 'managed-service:demo'

  assert_file_contains "${fake_log}" "--timeout=601s"
}

test_k8s_component_start_rejects_invalid_ready_timeout() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local fake_bin="${tmp}/bin"
  mkdir -p "${fake_bin}"

  cat >"${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$*" == *" apply "* ]]; then
  exit 0
fi

if [[ "$*" == *" get svc "* ]]; then
  cat <<'JSON'
{"items":[]}
JSON
  exit 0
fi

if [[ "$*" == *" wait "* ]]; then
  exit 0
fi

printf 'unexpected kubectl invocation: %s\n' "$*" >&2
exit 1
EOF
  chmod +x "${fake_bin}/kubectl"

  local component_dir="${tmp}/components/managed-service/demo"
  mkdir -p "${component_dir}/k8s"
  cat >"${component_dir}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: managed-service-demo
EOF

  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-service:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-service:demo']="${component_dir}"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS='invalid'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-service:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH

  local output status
  set +e
  output=$(e2e_component_builtin_start_kubernetes 'managed-service:demo' 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "invalid kubernetes component readiness timeout"
}

test_dependency_ordering_respects_dependencies
test_cycle_detection_fails_with_actionable_message
test_parallel_hook_failures_retain_component_logs_in_run_artifacts
test_prepare_metadata_workspace_uses_component_metadata_for_dir_mode
test_prepare_metadata_workspace_uses_keycloak_bundle_for_bundle_mode
test_prepare_metadata_workspace_falls_back_to_component_metadata_when_bundle_mapping_is_missing
test_prepare_metadata_workspace_allows_bundle_mode_without_mapping
test_forward_proxy_prompt_auth_prints_credentials_without_helper_file
test_repo_context_scripts_emit_metadata_bundle_when_set
test_prepare_component_openapi_specs_skips_local_openapi_for_bundle_mode
test_prepare_component_openapi_specs_keeps_local_openapi_for_dir_mode
test_keycloak_configure_auth_script_uses_current_client_id_fields
test_prepare_component_openapi_specs_defaults_to_bundle_mode
test_component_compose_file_resolves_compose_subdir
test_k8s_port_forward_pid_tracking_and_stop
test_k8s_port_forward_retries_after_runtime_disconnect
test_k8s_component_start_preloads_unique_images_before_apply
test_k8s_component_start_reuses_cached_exported_archives_across_runs
test_kubernetes_runtime_retries_retryable_kind_bootstrap_failure
test_kubernetes_runtime_fails_after_retryable_failures_without_reuse
test_kubernetes_runtime_serializes_podman_kind_create
test_kubernetes_runtime_serializes_podman_kind_load_against_create
test_kubernetes_runtime_limits_active_podman_clusters
test_kubernetes_runtime_kind_lock_timeout_is_configurable
test_k8s_component_start_uses_configurable_ready_timeout
test_k8s_component_start_rejects_invalid_ready_timeout
