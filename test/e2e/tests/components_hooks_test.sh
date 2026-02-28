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
  E2E_RESOURCE_SERVER='demo'
  E2E_RESOURCE_SERVER_CONNECTION='local'
  E2E_RESOURCE_SERVER_AUTH_TYPE='oauth2'
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

test_prepare_metadata_workspace_uses_component_metadata_for_local_dir_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_RESOURCE_SERVER='keycloak'
  E2E_METADATA='local-dir'
  E2E_COMPONENT_PATH=()
  local component_dir="${tmp}/components/resource-server/keycloak"
  local component_metadata="${component_dir}/metadata"
  mkdir -p "${component_metadata}"
  E2E_COMPONENT_PATH['resource-server:keycloak']="${component_dir}"

  e2e_prepare_metadata_workspace

  assert_eq "${E2E_METADATA_BUNDLE:-}" "" "expected metadata bundle to stay unset for local-dir mode"
  assert_eq "${E2E_METADATA_DIR}" "${component_metadata}" "expected metadata dir to reference component metadata path"
}

test_prepare_metadata_workspace_uses_keycloak_bundle_for_bundle_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  E2E_RESOURCE_SERVER='keycloak'
  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['resource-server:keycloak']="${tmp}/components/resource-server/keycloak"

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

  E2E_RESOURCE_SERVER='simple-api-server'
  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['resource-server:simple-api-server']="${tmp}/components/resource-server/simple-api-server"

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
  component_dir=$(create_openapi_component "${tmp}/components" "resource-server:demo")

  E2E_METADATA='bundle'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("resource-server:demo")
  E2E_COMPONENT_PATH['resource-server:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC['resource-server:demo']:-}" ]]; then
    fail "expected bundle mode to skip local openapi spec wiring"
  fi
}

test_prepare_component_openapi_specs_keeps_local_openapi_for_local_dir_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local component_dir
  component_dir=$(create_openapi_component "${tmp}/components" "resource-server:demo")

  E2E_METADATA='local-dir'
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("resource-server:demo")
  E2E_COMPONENT_PATH['resource-server:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  local copied_spec="${E2E_COMPONENT_OPENAPI_SPEC['resource-server:demo']:-}"
  [[ -n "${copied_spec}" ]] || fail "expected local-dir mode to wire local openapi spec"
  assert_path_exists "${copied_spec}"
}

test_prepare_component_openapi_specs_defaults_to_bundle_mode() {
  load_hook_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_runtime_globals "${tmp}"

  local component_dir
  component_dir=$(create_openapi_component "${tmp}/components" "resource-server:demo")

  unset E2E_METADATA
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_OPENAPI_SPEC=()
  E2E_SELECTED_COMPONENT_KEYS=("resource-server:demo")
  E2E_COMPONENT_PATH['resource-server:demo']="${component_dir}"

  e2e_prepare_component_openapi_specs

  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC['resource-server:demo']:-}" ]]; then
    fail "expected default metadata mode to skip local openapi spec wiring"
  fi
}

test_dependency_ordering_respects_dependencies
test_cycle_detection_fails_with_actionable_message
test_parallel_hook_failures_retain_component_logs_in_run_artifacts
test_prepare_metadata_workspace_uses_component_metadata_for_local_dir_mode
test_prepare_metadata_workspace_uses_keycloak_bundle_for_bundle_mode
test_prepare_metadata_workspace_allows_bundle_mode_without_mapping
test_repo_context_scripts_emit_metadata_bundle_when_set
test_prepare_component_openapi_specs_skips_local_openapi_for_bundle_mode
test_prepare_component_openapi_specs_keeps_local_openapi_for_local_dir_mode
test_prepare_component_openapi_specs_defaults_to_bundle_mode
