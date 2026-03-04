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
  E2E_METADATA_BUNDLE=''
  E2E_METADATA='bundle'
  E2E_MANAGED_SERVER='demo'
  E2E_MANAGED_SERVER_CONNECTION='local'
  E2E_MANAGED_SERVER_AUTH_TYPE='oauth2'
  E2E_MANAGED_SERVER_MTLS='false'
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
  path_a=$(create_hook_component "${tmp}/components" "managed-server:alpha" "printf '%s\\n' \"\${E2E_COMPONENT_KEY}\" >> ${order_file@Q}")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "printf '%s\\n' \"\${E2E_COMPONENT_KEY}\" >> ${order_file@Q}")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-server:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["managed-server:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["managed-server:alpha"]=""
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]="managed-server:alpha"
  E2E_COMPONENT_RUNTIME_KIND["managed-server:alpha"]='native'
  E2E_COMPONENT_RUNTIME_KIND["repo-type:filesystem"]='native'

  e2e_components_run_hook_for_keys init false "${E2E_SELECTED_COMPONENT_KEYS[@]}"

  local actual
  actual=$(tr '\n' ' ' <"${order_file}" | sed 's/[[:space:]]\+$//')
  assert_eq "${actual}" "managed-server:alpha repo-type:filesystem" "expected dependency-ordered hook execution"
}

test_cycle_detection_fails_with_actionable_message() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local path_a path_b
  path_a=$(create_hook_component "${tmp}/components" "managed-server:alpha" "exit 0")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "exit 0")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-server:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["managed-server:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["managed-server:alpha"]="repo-type:filesystem"
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]="managed-server:alpha"
  E2E_COMPONENT_RUNTIME_KIND["managed-server:alpha"]='native'
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
  path_a=$(create_hook_component "${tmp}/components" "managed-server:alpha" "printf 'alpha hook ran\\n'; exit 0")
  path_b=$(create_hook_component "${tmp}/components" "repo-type:filesystem" "printf 'beta hook failed\\n' >&2; exit 23")

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-server:alpha" "repo-type:filesystem")

  E2E_COMPONENT_PATH["managed-server:alpha"]="${path_a}"
  E2E_COMPONENT_PATH["repo-type:filesystem"]="${path_b}"
  E2E_COMPONENT_DEPENDS_ON["managed-server:alpha"]=""
  E2E_COMPONENT_DEPENDS_ON["repo-type:filesystem"]=""
  E2E_COMPONENT_RUNTIME_KIND["managed-server:alpha"]='native'
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

test_prepare_metadata_workspace_uses_component_metadata_for_base_dir_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_MANAGED_SERVER='keycloak'
  E2E_METADATA='base-dir'
  E2E_COMPONENT_PATH=()
  local component_dir="${tmp}/components/managed-server/keycloak"
  local component_metadata="${component_dir}/metadata"
  mkdir -p "${component_metadata}"
  E2E_COMPONENT_PATH['managed-server:keycloak']="${component_dir}"

  e2e_prepare_metadata_workspace

  assert_eq "${E2E_METADATA_BUNDLE:-}" "" "expected metadata bundle to stay unset for base-dir mode"
  assert_eq "${E2E_METADATA_DIR}" "${component_metadata}" "expected metadata dir to reference component metadata path"
}

test_prepare_metadata_workspace_uses_keycloak_bundle_for_bundle_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_MANAGED_SERVER='keycloak'
  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:keycloak']="${tmp}/components/managed-server/keycloak"

  e2e_prepare_metadata_workspace

  assert_eq "${E2E_METADATA_BUNDLE}" "keycloak-bundle:0.0.1" "expected keycloak shorthand metadata bundle"
  assert_eq "${E2E_METADATA_DIR:-}" "" "expected metadata workspace dir to stay unset when bundle is selected"
}

test_prepare_metadata_workspace_allows_bundle_mode_without_mapping() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_MANAGED_SERVER='simple-api-server'
  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:simple-api-server']="${tmp}/components/managed-server/simple-api-server"

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
REPO_RESOURCE_FORMAT=json
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
    assert_eq "$(context_metadata_line "${fragment_file}")" "  base-dir: ${metadata_dir}" "expected ${script_path} to emit metadata.base-dir fallback"
  done
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
  component_dir=$(create_openapi_component "${tmp}/components" "managed-server:demo")

  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-server:demo")
  E2E_COMPONENT_PATH['managed-server:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC['managed-server:demo']:-}" ]]; then
    fail "expected bundle mode to skip local openapi spec wiring"
  fi
}

test_prepare_component_openapi_specs_keeps_local_openapi_for_base_dir_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local component_dir
  component_dir=$(create_openapi_component "${tmp}/components" "managed-server:demo")

  E2E_METADATA='base-dir'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-server:demo")
  E2E_COMPONENT_PATH['managed-server:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  local copied_spec="${E2E_COMPONENT_OPENAPI_SPEC['managed-server:demo']:-}"
  [[ -n "${copied_spec}" ]] || fail "expected base-dir mode to wire local openapi spec"
  assert_path_exists "${copied_spec}"
}

test_prepare_component_openapi_specs_defaults_to_bundle_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local component_dir
  component_dir=$(create_openapi_component "${tmp}/components" "managed-server:demo")

  unset E2E_METADATA
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("managed-server:demo")
  E2E_COMPONENT_PATH['managed-server:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC['managed-server:demo']:-}" ]]; then
    fail "expected default metadata type to skip local openapi spec wiring"
  fi
}

test_component_compose_file_resolves_compose_subdir() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:demo']="${tmp}/components/managed-server/demo"

  local compose_file
  compose_file=$(e2e_component_compose_file 'managed-server:demo')
  assert_eq "${compose_file}" "${tmp}/components/managed-server/demo/compose/compose.yaml" "expected compose artifact path to use compose/compose.yaml"
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
  E2E_COMPONENT_RUNTIME_KIND['managed-server:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:demo']="${tmp}/components/managed-server/demo"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-server:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KUBECTL_LOG="${fake_log}"

  e2e_component_start_k8s_port_forwards 'managed-server:demo' "${state_file}"

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

  e2e_component_builtin_stop_kubernetes 'managed-server:demo'

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
  E2E_COMPONENT_RUNTIME_KIND['managed-server:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:demo']="${tmp}/components/managed-server/demo"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-server:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KUBECTL_LOG="${fake_log}"
  export FAKE_KUBECTL_COUNTER="${counter_file}"

  e2e_component_start_k8s_port_forwards 'managed-server:demo' "${state_file}"

  local pids
  pids=$(e2e_state_get "${state_file}" 'K8S_PORT_FORWARD_PIDS')
  [[ -n "${pids}" ]] || fail "expected k8s port-forward pid to be persisted"

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

  e2e_component_builtin_stop_kubernetes 'managed-server:demo'
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

  local component_dir_one="${tmp}/components/managed-server/demo-a"
  local component_dir_two="${tmp}/components/secret-provider/demo-b"
  mkdir -p "${component_dir_one}/k8s" "${component_dir_two}/k8s"

  cat >"${component_dir_one}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: managed-server-demo-a
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
  E2E_SELECTED_COMPONENT_KEYS=("managed-server:demo-a" "secret-provider:demo-b")

  E2E_COMPONENT_PATH['managed-server:demo-a']="${component_dir_one}"
  E2E_COMPONENT_PATH['secret-provider:demo-b']="${component_dir_two}"
  E2E_COMPONENT_DEPENDS_ON['managed-server:demo-a']=''
  E2E_COMPONENT_DEPENDS_ON['secret-provider:demo-b']=''
  E2E_COMPONENT_RUNTIME_KIND['managed-server:demo-a']='compose'
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
  load_count=$(grep -c '^load image-archive .*/docker.io_gitea_gitea_1.25.4.tar --name declarest-e2e-hooks$' "${kind_log}" || true)

  assert_eq "${pull_count}" "1" "expected one image pull for duplicated image references"
  assert_eq "${save_count}" "1" "expected one image export for duplicated image references"
  assert_eq "${load_count}" "1" "expected one kind image load for duplicated image references"
  assert_file_contains "${kubectl_log}" "apply -f ${E2E_STATE_DIR}/k8s-rendered/managed-server-demo-a/deployment.yaml"
  assert_file_contains "${kubectl_log}" "apply -f ${E2E_STATE_DIR}/k8s-rendered/secret-provider-demo-b/deployment.yaml"
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
  E2E_SELECTED_COMPONENT_KEYS=('managed-server:demo')
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-server:demo']='compose'

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

test_kubernetes_runtime_reuses_existing_cluster_after_retryable_failures() {
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
  E2E_SELECTED_COMPONENT_KEYS=('managed-server:demo')
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-server:demo']='compose'

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KIND_LOG="${kind_log}"
  export DECLAREST_E2E_KIND_REUSE_EXISTING_ON_CREATE_FAILURE='true'

  e2e_kubernetes_runtime_ensure

  assert_eq "${E2E_KIND_CLUSTER_NAME}" "declarest-e2e-existing" "expected fallback to existing kind cluster"
  assert_eq "${E2E_KIND_CLUSTER_REUSED}" "1" "expected reused cluster marker to be set"
  assert_file_contains "${kind_log}" "provider=podman cmd=export kubeconfig --name declarest-e2e-existing --kubeconfig ${E2E_KUBECONFIG}"
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

  local component_dir="${tmp}/components/managed-server/demo"
  mkdir -p "${component_dir}/k8s"
  cat >"${component_dir}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: managed-server-demo
EOF

  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-server:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:demo']="${component_dir}"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS='601'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-server:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH
  export FAKE_KUBECTL_LOG="${fake_log}"

  e2e_component_builtin_start_kubernetes 'managed-server:demo'

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

  local component_dir="${tmp}/components/managed-server/demo"
  mkdir -p "${component_dir}/k8s"
  cat >"${component_dir}/k8s/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: managed-server-demo
EOF

  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_RUNTIME_KIND['managed-server:demo']='compose'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:demo']="${component_dir}"
  E2E_KUBECONFIG="${tmp}/kubeconfig"
  E2E_K8S_NAMESPACE='declarest-tests'
  E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS='invalid'
  : >"${E2E_KUBECONFIG}"

  local state_file
  state_file=$(e2e_component_state_file 'managed-server:demo')
  : >"${state_file}"

  local old_path="${PATH}"
  PATH="${fake_bin}:${old_path}"
  export PATH

  local output status
  set +e
  output=$(e2e_component_builtin_start_kubernetes 'managed-server:demo' 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "invalid kubernetes component readiness timeout"
}

test_dependency_ordering_respects_dependencies
test_cycle_detection_fails_with_actionable_message
test_parallel_hook_failures_retain_component_logs_in_run_artifacts
test_prepare_metadata_workspace_uses_component_metadata_for_base_dir_mode
test_prepare_metadata_workspace_uses_keycloak_bundle_for_bundle_mode
test_prepare_metadata_workspace_allows_bundle_mode_without_mapping
test_repo_context_scripts_emit_metadata_bundle_when_set
test_prepare_component_openapi_specs_skips_local_openapi_for_bundle_mode
test_prepare_component_openapi_specs_keeps_local_openapi_for_base_dir_mode
test_prepare_component_openapi_specs_defaults_to_bundle_mode
test_component_compose_file_resolves_compose_subdir
test_k8s_port_forward_pid_tracking_and_stop
test_k8s_port_forward_retries_after_runtime_disconnect
test_k8s_component_start_preloads_unique_images_before_apply
test_kubernetes_runtime_retries_retryable_kind_bootstrap_failure
test_kubernetes_runtime_reuses_existing_cluster_after_retryable_failures
test_k8s_component_start_uses_configurable_ready_timeout
test_k8s_component_start_rejects_invalid_ready_timeout
