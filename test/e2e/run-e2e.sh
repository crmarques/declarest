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

  e2e_info 'execution parameters:'
  while IFS= read -r line; do
    e2e_info "  ${line}"
  done < <(ui_execution_parameter_lines)

  return 0
}

step_prepare_runtime() {
  if [[ -z "${E2E_RUN_ID}" ]]; then
    E2E_RUN_ID=$(date +%Y%m%d-%H%M%S)-$$
  fi
  E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  E2E_LOG_DIR="${E2E_RUN_DIR}/logs"
  E2E_CONTEXT_DIR="${E2E_RUN_DIR}/context"
  E2E_CONTEXT_FILE="${E2E_RUN_DIR}/contexts.yaml"
  E2E_BIN="${E2E_RUN_DIR}/bin/declarest"
  E2E_OPERATOR_BIN="${E2E_ROOT_DIR}/.e2e-build/declarest-operator-manager-${E2E_RUN_ID}"

  if [[ -z "${E2E_EXECUTION_LOG}" ]]; then
    E2E_EXECUTION_LOG="${E2E_RUN_DIR}/execution.log"
  fi

  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}" "$(dirname -- "${E2E_BIN}")" "$(dirname -- "${E2E_OPERATOR_BIN}")" || return 1
  e2e_info "runtime paths run-dir=${E2E_RUN_DIR} state-dir=${E2E_STATE_DIR} log-dir=${E2E_LOG_DIR} context-file=${E2E_CONTEXT_FILE}"
  e2e_info "runtime binary path=${E2E_BIN}"
  e2e_runtime_state_record_platform || return 1

  e2e_prepare_metadata_workspace || return 1
  e2e_operator_prepare_repository_webhook || return 1

  printf '%s\n' "${E2E_SELECTED_COMPONENT_KEYS[@]}" >"${E2E_STATE_DIR}/selected-components.txt"

  if [[ -n "${E2E_BOOTSTRAP_LOG_DIR}" && -d "${E2E_BOOTSTRAP_LOG_DIR}" ]]; then
    cp -a "${E2E_BOOTSTRAP_LOG_DIR}/." "${E2E_LOG_DIR}/" 2>/dev/null || true
  fi

  e2e_run_cmd go build -o "${E2E_BIN}" ./cmd/declarest || return 1
  if e2e_profile_is_operator; then
    local go_version
    go_version=$(e2e_resolve_go_version) || return 1
    e2e_run_cmd env CGO_ENABLED=0 GOOS=linux go build -o "${E2E_OPERATOR_BIN}" ./cmd/declarest-operator-manager || return 1
    e2e_register_temp_file "${E2E_OPERATOR_BIN}"
    E2E_OPERATOR_IMAGE="localhost/declarest/e2e-operator-manager:${E2E_RUN_ID}"
    export E2E_OPERATOR_IMAGE
    local operator_binary_rel="${E2E_OPERATOR_BIN#${E2E_ROOT_DIR}/}"
    if [[ "${operator_binary_rel}" == "${E2E_OPERATOR_BIN}" ]]; then
      e2e_die "operator binary path is outside repository root: ${E2E_OPERATOR_BIN}"
      return 1
    fi
    e2e_run_cmd "${E2E_CONTAINER_ENGINE}" build -f "${E2E_ROOT_DIR}/Dockerfile.operator" --build-arg "GO_VERSION=${go_version}" --build-arg "MANAGER_BINARY=${operator_binary_rel}" -t "${E2E_OPERATOR_IMAGE}" "${E2E_ROOT_DIR}" || return 1
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

  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" config show >/dev/null || return 1

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
  local managed_server_key=''
  local details
  local output=''

  if [[ -n "${E2E_MANAGED_SERVER:-}" && "${E2E_MANAGED_SERVER}" != 'none' ]]; then
    managed_server_key=$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")
  fi

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    details=$(e2e_component_collect_manual_info "${component_key}" || true)
    if [[ -z "${details//[$'\t\r\n ']}" && -n "${managed_server_key}" && "${component_key}" == "${managed_server_key}" ]]; then
      details=$(e2e_profile_managed_server_access_details || true)
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

e2e_profile_adjust_seeded_repo() {
  local repo_base_dir=$1

  if ! e2e_profile_is_operator || [[ "${E2E_MANAGED_SERVER}" != 'keycloak' ]]; then
    return 0
  fi

  local realm_root="${repo_base_dir}/admin/realms/acme"
  if [[ ! -d "${realm_root}" ]]; then
    return 0
  fi

  local pruned=0
  local child
  for child in authentication clients organizations user-registry; do
    if [[ -e "${realm_root}/${child}" ]]; then
      rm -rf "${realm_root:?}/${child}"
      ((pruned += 1))
    fi
  done

  if ((pruned > 0)); then
    e2e_info "operator profile keycloak seed adjusted removed-non-idempotent-paths=${pruned} root=${realm_root}"
  fi
}

e2e_profile_seed_repo_from_template() {
  if ! e2e_profile_is_manual_handoff && ! e2e_profile_is_operator; then
    return 0
  fi

  if [[ "${E2E_MANAGED_SERVER}" == 'none' ]]; then
    e2e_info "${E2E_PROFILE} profile repo-template sync skipped: managed-server=none"
    return 0
  fi

  local repo_component_key
  local resource_component_key
  local repo_state_file
  local repo_base_dir
  local template_dir
  local file_count

  repo_component_key=$(e2e_component_key 'repo-type' "${E2E_REPO_TYPE}")
  resource_component_key=$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")
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

  e2e_profile_adjust_seeded_repo "${repo_base_dir}" || return 1

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

main "$@"
