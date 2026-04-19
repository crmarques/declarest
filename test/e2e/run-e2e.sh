#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/args.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/profile.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/operator.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/context.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/cases.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/ui.sh"

E2E_CLI_ARGS=()
E2E_SHORT_CIRCUIT=0
E2E_OVERALL_FAILED=0
E2E_FINALIZED=0
E2E_BOOTSTRAP_LOG_DIR=''
E2E_PID_FILE=''
E2E_RUNNER_PID=$$
E2E_SIGNAL_HANDLED=0
E2E_MATCHED_RUNNER_PIDS=()
E2E_MANUAL_COMPONENT_ACCESS_OUTPUT=''

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/runner_cleanup.sh"

step_initialize() {
  e2e_parse_args "${E2E_CLI_ARGS[@]}" || return 1
  e2e_apply_profile_defaults || return 1
  e2e_validate_platform || return 1
  e2e_validate_container_engine || return 1

  e2e_discover_components || return 1

  if ((E2E_VALIDATE_COMPONENTS == 1)); then
    e2e_validate_all_discovered_component_contracts || return 1
    if ((E2E_LIST_COMPONENTS == 1)); then
      e2e_list_components || return 1
    fi
    E2E_SHORT_CIRCUIT=1
    return 0
  fi

  if ((E2E_LIST_COMPONENTS == 1)); then
    e2e_list_components || return 1
    E2E_SHORT_CIRCUIT=1
    return 0
  fi

  e2e_validate_selection || return 1
  e2e_validate_profile_rules || return 1

  e2e_build_selected_components || return 1
  e2e_validate_selected_component_dependencies || return 1
  e2e_build_capabilities || return 1
  e2e_preflight_requirements || return 1

  return 0
}

e2e_print_startup_execution_parameters() {
  local preview
  local line

  preview=$(
    (
      e2e_parse_args "${E2E_CLI_ARGS[@]}" || exit 1
      e2e_apply_profile_defaults || exit 1
      e2e_validate_platform || exit 1
      e2e_validate_container_engine || exit 1
      e2e_discover_components || exit 1

      if ((E2E_VALIDATE_COMPONENTS == 0 && E2E_LIST_COMPONENTS == 0)); then
        e2e_validate_selection || exit 1
        e2e_validate_profile_rules || exit 1
        e2e_build_selected_components || exit 1
        e2e_validate_selected_component_dependencies || exit 1
        e2e_build_capabilities || exit 1
      fi

      ui_execution_parameter_lines
    ) 2>/dev/null
  ) || return 1

  printf '\n'
  printf 'Execution Parameters\n'
  printf '%s\n' '--------------------'
  while IFS= read -r line; do
    printf '  %s\n' "${line}"
  done <<<"${preview}"
  printf '\n'
}

e2e_operator_runtime_dockerfile_path() {
  local go_arch=$1
  printf '%s/Dockerfile.operator-runtime-linux-%s\n' "${E2E_BUILD_CACHE_DIR}" "${go_arch}"
}

e2e_write_operator_runtime_dockerfile() {
  local dockerfile_path=$1
  local operator_binary_rel=$2

  cat >"${dockerfile_path}" <<EOF
FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /
COPY ${operator_binary_rel} /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
EOF
}

step_prepare_runtime() {
  local cached_cli_bin
  local cached_cli_tmp
  local cached_operator_bin=''
  local cached_operator_tmp=''

  if [[ -z "${E2E_RUN_ID}" ]]; then
    E2E_RUN_ID=$(date +%Y%m%d-%H%M%S)-$$
  fi
  E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  E2E_LOG_DIR="${E2E_RUN_DIR}/logs"
  E2E_CONTEXT_DIR="${E2E_RUN_DIR}/context"
  E2E_CONTEXT_FILE="${E2E_RUN_DIR}/contexts.yaml"
  E2E_BIN="${E2E_RUN_DIR}/bin/declarest"
  cached_cli_bin="${E2E_BUILD_CACHE_DIR}/declarest"
  cached_cli_tmp="${E2E_BUILD_CACHE_DIR}/declarest.${E2E_RUN_ID}.tmp"

  if [[ -z "${E2E_EXECUTION_LOG}" ]]; then
    E2E_EXECUTION_LOG="${E2E_RUN_DIR}/execution.log"
  fi

  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}" "${E2E_BUILD_CACHE_DIR}" "$(dirname -- "${E2E_BIN}")" || return 1
  e2e_info "runtime paths run-dir=${E2E_RUN_DIR} state-dir=${E2E_STATE_DIR} log-dir=${E2E_LOG_DIR} context-file=${E2E_CONTEXT_FILE}"
  e2e_info "runtime binary path=${E2E_BIN}"
  e2e_runtime_state_record_platform || return 1

  e2e_prepare_metadata_workspace || return 1
  e2e_operator_prepare_repository_webhook || return 1

  printf '%s\n' "${E2E_SELECTED_COMPONENT_KEYS[@]}" >"${E2E_STATE_DIR}/selected-components.txt"

  if [[ -n "${E2E_BOOTSTRAP_LOG_DIR}" && -d "${E2E_BOOTSTRAP_LOG_DIR}" ]]; then
    cp -a "${E2E_BOOTSTRAP_LOG_DIR}/." "${E2E_LOG_DIR}/" 2>/dev/null || true
  fi

  if e2e_go_build_target_is_stale \
    "${cached_cli_bin}" \
    "${E2E_ROOT_DIR}/go.mod" \
    "${E2E_ROOT_DIR}/go.sum" \
    "${E2E_ROOT_DIR}/api" \
    "${E2E_ROOT_DIR}/cmd" \
    "${E2E_ROOT_DIR}/config" \
    "${E2E_ROOT_DIR}/debugctx" \
    "${E2E_ROOT_DIR}/faults" \
    "${E2E_ROOT_DIR}/internal" \
    "${E2E_ROOT_DIR}/managedservice" \
    "${E2E_ROOT_DIR}/metadata" \
    "${E2E_ROOT_DIR}/orchestrator" \
    "${E2E_ROOT_DIR}/repository" \
    "${E2E_ROOT_DIR}/resource" \
    "${E2E_ROOT_DIR}/secrets"; then
    rm -f "${cached_cli_tmp}" || return 1
    e2e_run_cmd go build -o "${cached_cli_tmp}" ./cmd/declarest || return 1
    mv -f "${cached_cli_tmp}" "${cached_cli_bin}" || return 1
  else
    e2e_info "using cached e2e cli binary path=${cached_cli_bin}"
  fi
  e2e_stage_cached_binary "${cached_cli_bin}" "${E2E_BIN}" || return 1
  if e2e_profile_is_operator; then
    local go_arch
    local operator_runtime_dockerfile
    go_arch=$(e2e_resolve_go_arch) || return 1
    cached_operator_bin="${E2E_BUILD_CACHE_DIR}/declarest-operator-manager-linux-${go_arch}"
    cached_operator_tmp="${E2E_BUILD_CACHE_DIR}/declarest-operator-manager-linux-${go_arch}.${E2E_RUN_ID}.tmp"
    E2E_OPERATOR_BIN="${cached_operator_bin}"

    if e2e_go_build_target_is_stale \
      "${cached_operator_bin}" \
      "${E2E_ROOT_DIR}/go.mod" \
      "${E2E_ROOT_DIR}/go.sum" \
      "${E2E_ROOT_DIR}/api" \
      "${E2E_ROOT_DIR}/cmd" \
      "${E2E_ROOT_DIR}/config" \
      "${E2E_ROOT_DIR}/debugctx" \
      "${E2E_ROOT_DIR}/faults" \
      "${E2E_ROOT_DIR}/internal" \
      "${E2E_ROOT_DIR}/managedservice" \
      "${E2E_ROOT_DIR}/metadata" \
      "${E2E_ROOT_DIR}/orchestrator" \
      "${E2E_ROOT_DIR}/repository" \
      "${E2E_ROOT_DIR}/resource" \
      "${E2E_ROOT_DIR}/secrets"; then
      rm -f "${cached_operator_tmp}" || return 1
      e2e_run_cmd env CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" go build -o "${cached_operator_tmp}" ./cmd/declarest-operator-manager || return 1
      mv -f "${cached_operator_tmp}" "${cached_operator_bin}" || return 1
    else
      e2e_info "using cached e2e operator manager binary path=${cached_operator_bin}"
    fi

    E2E_OPERATOR_IMAGE="localhost/declarest/e2e-operator-manager:${E2E_RUN_ID}"
    export E2E_OPERATOR_IMAGE
    local operator_binary_rel
    operator_binary_rel=$(basename -- "${cached_operator_bin}") || return 1
    operator_runtime_dockerfile=$(e2e_operator_runtime_dockerfile_path "${go_arch}") || return 1
    e2e_write_operator_runtime_dockerfile "${operator_runtime_dockerfile}" "${operator_binary_rel}" || return 1
    e2e_run_cmd "${E2E_CONTAINER_ENGINE}" build -f "${operator_runtime_dockerfile}" -t "${E2E_OPERATOR_IMAGE}" "${E2E_BUILD_CACHE_DIR}" || return 1
    e2e_info "runtime operator image=${E2E_OPERATOR_IMAGE}"
  fi

  e2e_collect_case_files || return 1
  e2e_info "runtime case files collected count=${#E2E_CASE_FILES[@]}"
  return 0
}

step_prepare_components() {
  e2e_components_run_hook_all 'init' 'true' || return 1
}

step_start_components() {
  e2e_components_start_local || return 1
  e2e_components_healthcheck_local || return 1
}

step_configure_access() {
  local deferred_webhook_provider=''
  local deferred_webhook_url=''
  local deferred_webhook_secret=''

  if e2e_operator_should_defer_repository_webhook_registration; then
    deferred_webhook_provider=${E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER:-}
    deferred_webhook_url=${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL:-}
    deferred_webhook_secret=${E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET:-}
    unset E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER
    unset E2E_OPERATOR_REPOSITORY_WEBHOOK_URL
    unset E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET
  fi

  if ! e2e_components_run_hook_all 'configure-auth' 'true'; then
    if [[ -n "${deferred_webhook_provider}" ]]; then
      E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER=${deferred_webhook_provider}
      export E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER
    fi
    if [[ -n "${deferred_webhook_url}" ]]; then
      E2E_OPERATOR_REPOSITORY_WEBHOOK_URL=${deferred_webhook_url}
      export E2E_OPERATOR_REPOSITORY_WEBHOOK_URL
    fi
    if [[ -n "${deferred_webhook_secret}" ]]; then
      E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET=${deferred_webhook_secret}
      export E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET
    fi
    return 1
  fi

  if [[ -n "${deferred_webhook_provider}" ]]; then
    E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER=${deferred_webhook_provider}
    export E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER
  fi
  if [[ -n "${deferred_webhook_url}" ]]; then
    E2E_OPERATOR_REPOSITORY_WEBHOOK_URL=${deferred_webhook_url}
    export E2E_OPERATOR_REPOSITORY_WEBHOOK_URL
  fi
  if [[ -n "${deferred_webhook_secret}" ]]; then
    E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET=${deferred_webhook_secret}
    export E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET
  fi

  mkdir -p "${E2E_CONTEXT_DIR}" || return 1
  e2e_prepare_component_openapi_specs || return 1
  e2e_components_run_hook_all 'context' 'true' || return 1

  e2e_context_build || return 1

  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" context show >/dev/null || return 1

  if e2e_profile_is_manual_handoff; then
    e2e_manual_collect_component_access_info || return 1
  fi
}

step_run_workload() {
  if e2e_profile_is_manual_handoff; then
    e2e_profile_manual_handoff "${E2E_CONTEXT_NAME}" || return 1
    return 0
  fi

  e2e_run_cases || return 1
}

step_operator_install() {
  e2e_profile_seed_repo_from_template || return 1
  e2e_profile_init_repo_if_needed || return 1
  e2e_operator_seed_remote_repo_if_git || return 1
  e2e_operator_install_stack || return 1
  e2e_operator_configure_repository_webhook_if_needed || return 1
}

e2e_manual_collect_component_access_info() {
  local component_key
  local managed_service_key=''
  local details
  local output=''

  if [[ -n "${E2E_MANAGED_SERVICE:-}" && "${E2E_MANAGED_SERVICE}" != 'none' ]]; then
    managed_service_key=$(e2e_component_key 'managed-service' "${E2E_MANAGED_SERVICE}")
  fi

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    details=$(e2e_component_collect_manual_info "${component_key}" || true)
    if [[ -z "${details//[$'\t\r\n ']}" && -n "${managed_service_key}" && "${component_key}" == "${managed_service_key}" ]]; then
      details=$(e2e_profile_managed_service_access_details || true)
    fi
    if [[ -z "${details//[$'\t\r\n ']}" ]]; then
      continue
    fi

    if [[ -n "${output}" ]]; then
      output+=$'\n'
    fi

    output+="${component_key}"$'\n'
    output+="$(printf '%s\n' "${details}" | sed 's/^/  /')"
  done

  E2E_MANUAL_COMPONENT_ACCESS_OUTPUT="${output}"
}

e2e_profile_prepare_seeded_repo() {
  local component_key=$1
  local template_dir=$2
  local repo_base_dir=$3
  local hook_script

  hook_script="${E2E_COMPONENT_PATH[${component_key}]:-}/scripts/prepare-repo-template.sh"
  if [[ ! -x "${hook_script}" ]]; then
    return 0
  fi

  e2e_info "${E2E_PROFILE} profile repo-template prepare hook component=${component_key} script=${hook_script}"
  E2E_COMPONENT_KEY="${component_key}" \
  E2E_COMPONENT_TYPE="$(e2e_component_type "${component_key}")" \
    E2E_COMPONENT_NAME="$(e2e_component_name "${component_key}")" \
    E2E_COMPONENT_DIR="${E2E_COMPONENT_PATH[${component_key}]}" \
    E2E_REPO_TEMPLATE_DIR="${template_dir}" \
    E2E_REPO_BASE_DIR="${repo_base_dir}" \
    bash "${hook_script}" "${repo_base_dir}" || {
      e2e_die "${E2E_PROFILE} profile repo-template prepare hook failed for ${component_key}: ${hook_script}"
      return 1
    }
}

e2e_profile_seed_repo_from_template() {
  if ! e2e_profile_is_manual_handoff && ! e2e_profile_is_operator; then
    return 0
  fi

  if [[ "${E2E_MANAGED_SERVICE}" == 'none' ]]; then
    e2e_info "${E2E_PROFILE} profile repo-template sync skipped: managed-service=none"
    return 0
  fi

  local repo_component_key
  local resource_component_key
  local repo_state_file
  local repo_base_dir
  local template_dir
  local file_count

  repo_component_key=$(e2e_component_key 'repo-type' "${E2E_REPO_TYPE}")
  resource_component_key=$(e2e_component_key 'managed-service' "${E2E_MANAGED_SERVICE}")
  repo_state_file=$(e2e_component_state_file "${repo_component_key}")

  repo_base_dir=$(e2e_state_get "${repo_state_file}" 'REPO_BASE_DIR' || true)
  if [[ -z "${repo_base_dir}" ]]; then
    e2e_die "${E2E_PROFILE} profile repo-template sync failed: missing REPO_BASE_DIR in ${repo_state_file}"
    return 1
  fi

  template_dir="${E2E_COMPONENT_PATH[${resource_component_key}]:-}/repo-template"
  if [[ ! -d "${template_dir}" ]]; then
    e2e_die "${E2E_PROFILE} profile repo-template sync failed: template dir not found: ${template_dir}"
    return 1
  fi

  mkdir -p "${repo_base_dir}" || {
    e2e_die "${E2E_PROFILE} profile repo-template sync failed: cannot create repo dir: ${repo_base_dir}"
    return 1
  }

  e2e_info "${E2E_PROFILE} profile repo-template sync source=${template_dir} target=${repo_base_dir}"
  cp -a "${template_dir}/." "${repo_base_dir}/" || {
    e2e_die "${E2E_PROFILE} profile repo-template sync failed while copying from ${template_dir} to ${repo_base_dir}"
    return 1
  }

  e2e_profile_prepare_seeded_repo "${resource_component_key}" "${template_dir}" "${repo_base_dir}" || return 1

  file_count=$(find "${repo_base_dir}" -type f | wc -l | tr -d ' ')
  e2e_info "${E2E_PROFILE} profile repo-template sync copied-files=${file_count}"
  return 0
}

e2e_profile_init_repo_if_needed() {
  if ! e2e_profile_is_manual_handoff && ! e2e_profile_is_operator; then
    return 0
  fi

  if [[ "${E2E_REPO_TYPE}" != 'git' ]]; then
    return 0
  fi

  e2e_info "${E2E_PROFILE} profile initializing git repository"
  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" repository init >/dev/null || {
    e2e_die "${E2E_PROFILE} profile git repository initialization failed"
    return 1
  }

  e2e_info "${E2E_PROFILE} profile git repository initialized"
  return 0
}

e2e_operator_seed_remote_repo_if_git() {
  if ! e2e_profile_is_operator || [[ "${E2E_REPO_TYPE}" != 'git' ]]; then
    return 0
  fi

  e2e_info 'operator profile committing seeded repository content'
  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" repository commit -m 'operator e2e seed content' >/dev/null || {
    e2e_die 'operator profile failed to commit seeded repository content'
    return 1
  }

  e2e_info 'operator profile pushing seeded repository content to remote'
  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" repository push --force-push >/dev/null || {
    e2e_die 'operator profile failed to push seeded repository content'
    return 1
  }

  return 0
}

step_skip_not_requested() {
  return "${E2E_STEP_SKIP}"
}

step_finalize() {
  if ((E2E_FINALIZED == 1)); then
    return 0
  fi

  E2E_FINALIZED=1

  if ((E2E_KEEP_RUNTIME == 1)); then
    if e2e_profile_is_cli_manual "${E2E_PROFILE:-}"; then
      e2e_info 'keeping runtime resources for cli-manual profile'
    elif e2e_profile_is_operator_manual "${E2E_PROFILE:-}"; then
      e2e_info 'keeping runtime resources for operator-manual profile'
    else
      e2e_info 'keeping runtime resources because --keep-runtime was set'
    fi
  else
    e2e_operator_stop_manager || true
    e2e_components_stop_started || true
  fi

  if [[ -n "${E2E_BOOTSTRAP_LOG_DIR}" && -d "${E2E_BOOTSTRAP_LOG_DIR}" && "${E2E_LOG_DIR}" != "${E2E_BOOTSTRAP_LOG_DIR}" ]]; then
    rm -rf "${E2E_BOOTSTRAP_LOG_DIR}" || true
  fi

  [[ -n "${E2E_PID_FILE}" && -f "${E2E_PID_FILE}" ]] && rm -f "${E2E_PID_FILE}" || true
  e2e_release_reserved_ports_for_run "${E2E_RUN_ID:-}" || true
  e2e_cleanup_temp_files
  return 0
}

main() {
  E2E_CLI_ARGS=("$@")
  local cleanup_parse_rc=0
  local profile_handoff_needed=0

  if e2e_has_help_flag "${E2E_CLI_ARGS[@]}"; then
    e2e_usage
    exit 0
  fi

  e2e_parse_cleanup_args "${E2E_CLI_ARGS[@]}" || cleanup_parse_rc=$?
  if ((cleanup_parse_rc == 0)); then
    e2e_handle_cleanup_mode || exit 1
    exit 0
  fi
  if ((cleanup_parse_rc > 1)); then
    exit 1
  fi

  trap 'e2e_handle_termination_signal INT' INT
  trap 'e2e_handle_termination_signal TERM' TERM

  E2E_START_EPOCH=$(e2e_epoch_now)

  mkdir -p "${E2E_RUNS_DIR}"
  if [[ -z "${E2E_RUN_ID}" ]]; then
    E2E_RUN_ID=$(date +%Y%m%d-%H%M%S)-$$
  fi
  E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  mkdir -p "${E2E_RUN_DIR}"
  export E2E_RUN_ID
  export E2E_RUNNER_PID
  E2E_PID_FILE=$(e2e_runner_pid_file_for_run_id "${E2E_RUN_ID}")
  printf '%s\n' "${E2E_RUNNER_PID}" >"${E2E_PID_FILE}"

  if [[ -z "${E2E_EXECUTION_LOG}" ]]; then
    E2E_EXECUTION_LOG="${E2E_RUN_DIR}/execution.log"
  fi
  mkdir -p "$(dirname -- "${E2E_EXECUTION_LOG}")"
  : >"${E2E_EXECUTION_LOG}"

  printf 'E2E execution log: %s\n' "${E2E_EXECUTION_LOG}"
  e2e_print_startup_execution_parameters || true

  E2E_BOOTSTRAP_LOG_DIR=$(mktemp -d /tmp/declarest-e2e-bootstrap.XXXXXX)
  E2E_LOG_DIR="${E2E_BOOTSTRAP_LOG_DIR}"

  ui_init
  local requested_profile='cli-basic'
  local workload_step=6
  requested_profile=$(e2e_profile_from_cli_args "${E2E_CLI_ARGS[@]}")
  E2E_STEPS_TOTAL=$(e2e_profile_total_steps "${requested_profile}")
  if e2e_profile_is_operator_workload "${requested_profile}"; then
    workload_step=7
  fi

  if ! ui_run_step 1 "${E2E_STEPS_TOTAL}" 'Initializing' step_initialize; then
    E2E_OVERALL_FAILED=1
  fi

  if ((E2E_OVERALL_FAILED == 0)); then
    if ((E2E_SHORT_CIRCUIT == 1)); then
      cat "${E2E_LOG_DIR}/step-1.log"
      ui_run_step 2 "${E2E_STEPS_TOTAL}" 'Preparing Runtime' step_skip_not_requested || true
      ui_run_step 3 "${E2E_STEPS_TOTAL}" 'Preparing Components' step_skip_not_requested || true
      ui_run_step 4 "${E2E_STEPS_TOTAL}" 'Starting Components' step_skip_not_requested || true
      ui_run_step 5 "${E2E_STEPS_TOTAL}" 'Configuring Access' step_skip_not_requested || true
      if e2e_profile_is_operator; then
        ui_run_step 6 "${E2E_STEPS_TOTAL}" 'Installing Operator' step_skip_not_requested || true
      fi
      if e2e_profile_is_workload; then
        ui_run_step "${workload_step}" "${E2E_STEPS_TOTAL}" 'Running Test Cases' step_skip_not_requested || true
      fi
    else
      ui_run_step 2 "${E2E_STEPS_TOTAL}" 'Preparing Runtime' step_prepare_runtime || E2E_OVERALL_FAILED=1
      if ((E2E_OVERALL_FAILED == 0)); then
        ui_run_step 3 "${E2E_STEPS_TOTAL}" 'Preparing Components' step_prepare_components || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)); then
        ui_run_step 4 "${E2E_STEPS_TOTAL}" 'Starting Components' step_start_components || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)); then
        ui_run_step 5 "${E2E_STEPS_TOTAL}" 'Configuring Access' step_configure_access || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)) && e2e_profile_is_operator; then
        ui_run_step 6 "${E2E_STEPS_TOTAL}" 'Installing Operator' step_operator_install || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)) && e2e_profile_is_workload; then
        ui_run_step "${workload_step}" "${E2E_STEPS_TOTAL}" 'Running Test Cases' step_run_workload || E2E_OVERALL_FAILED=1
      fi
    fi
  fi

  if e2e_profile_is_cli_manual; then
    if ((E2E_OVERALL_FAILED == 0 && E2E_SHORT_CIRCUIT == 0)); then
      e2e_profile_seed_repo_from_template || E2E_OVERALL_FAILED=1
      if ((E2E_OVERALL_FAILED == 0)); then
        e2e_profile_init_repo_if_needed || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)); then
        E2E_KEEP_RUNTIME=1
        profile_handoff_needed=1
      fi
    fi
    step_finalize || true
  else
    if e2e_profile_is_operator_manual && ((E2E_OVERALL_FAILED == 0 && E2E_SHORT_CIRCUIT == 0)); then
      E2E_KEEP_RUNTIME=1
      profile_handoff_needed=1
    fi
    ui_run_step "${E2E_STEPS_TOTAL}" "${E2E_STEPS_TOTAL}" 'Finalizing' step_finalize || true
  fi

  ui_print_summary

  if ((profile_handoff_needed == 1)); then
    printf '\n'
    if e2e_profile_is_operator_manual; then
      e2e_profile_operator_handoff "${E2E_CONTEXT_NAME}" || E2E_OVERALL_FAILED=1
    else
      e2e_profile_manual_handoff "${E2E_CONTEXT_NAME}" || E2E_OVERALL_FAILED=1
    fi
  fi

  if ((E2E_OVERALL_FAILED == 1 || E2E_CASE_FAILED > 0)); then
    exit 1
  fi

  trap - INT TERM
  exit 0
}

if [[ "${E2E_SOURCE_ONLY:-0}" != '1' ]]; then
  main "$@"
fi
