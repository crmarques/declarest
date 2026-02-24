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

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/runner_cleanup.sh"

step_initialize() {
  e2e_parse_args "${E2E_CLI_ARGS[@]}" || return 1
  e2e_apply_profile_defaults || return 1
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

  e2e_info "profile=${E2E_PROFILE} repo-type=${E2E_REPO_TYPE} resource-server=${E2E_RESOURCE_SERVER} secret-provider=${E2E_SECRET_PROVIDER} container-engine=${E2E_CONTAINER_ENGINE}"
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

  if [[ -z "${E2E_EXECUTION_LOG}" ]]; then
    E2E_EXECUTION_LOG="${E2E_RUN_DIR}/execution.log"
  fi

  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}" "$(dirname -- "${E2E_BIN}")" || return 1
  e2e_info "runtime paths run-dir=${E2E_RUN_DIR} state-dir=${E2E_STATE_DIR} log-dir=${E2E_LOG_DIR} context-file=${E2E_CONTEXT_FILE}"
  e2e_info "runtime binary path=${E2E_BIN}"

  e2e_prepare_metadata_workspace || return 1

  printf '%s\n' "${E2E_SELECTED_COMPONENT_KEYS[@]}" >"${E2E_STATE_DIR}/selected-components.txt"

  if [[ -n "${E2E_BOOTSTRAP_LOG_DIR}" && -d "${E2E_BOOTSTRAP_LOG_DIR}" ]]; then
    cp -a "${E2E_BOOTSTRAP_LOG_DIR}/." "${E2E_LOG_DIR}/" 2>/dev/null || true
  fi

  e2e_run_cmd go build -o "${E2E_BIN}" ./cmd/declarest || return 1

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
  e2e_components_run_hook_all 'configure-auth' 'true' || return 1

  mkdir -p "${E2E_CONTEXT_DIR}" || return 1
  e2e_prepare_component_openapi_specs || return 1
  e2e_components_run_hook_all 'context' 'true' || return 1

  e2e_context_build || return 1

  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" config show >/dev/null || return 1
}

step_run_workload() {
  if [[ "${E2E_PROFILE}" == 'manual' ]]; then
    e2e_profile_manual_handoff "${E2E_CONTEXT_NAME}" || return 1
    return 0
  fi

  e2e_run_cases || return 1
}

e2e_manual_print_component_access_info() {
  local component_key
  local section_printed=0
  local details

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    details=$(e2e_component_collect_manual_info "${component_key}" || true)
    if [[ -z "${details//[$'\t\r\n ']}" ]]; then
      continue
    fi

    if ((section_printed == 0)); then
      printf '\nManual Component Access\n'
      printf '%s\n' '-----------------------'
      section_printed=1
    fi

    printf '%s\n' "${component_key}"
    printf '%s\n' "${details}" | sed 's/^/  /'
  done
}

e2e_manual_seed_repo_from_template() {
  if [[ "${E2E_PROFILE}" != 'manual' ]]; then
    return 0
  fi

  if [[ "${E2E_RESOURCE_SERVER}" == 'none' ]]; then
    e2e_info 'manual profile repo-template sync skipped: resource-server=none'
    return 0
  fi

  local repo_component_key
  local resource_component_key
  local repo_state_file
  local repo_base_dir
  local template_dir
  local file_count

  repo_component_key=$(e2e_component_key 'repo-type' "${E2E_REPO_TYPE}")
  resource_component_key=$(e2e_component_key 'resource-server' "${E2E_RESOURCE_SERVER}")
  repo_state_file=$(e2e_component_state_file "${repo_component_key}")

  repo_base_dir=$(e2e_state_get "${repo_state_file}" 'REPO_BASE_DIR' || true)
  if [[ -z "${repo_base_dir}" ]]; then
    e2e_die "manual profile repo-template sync failed: missing REPO_BASE_DIR in ${repo_state_file}"
    return 1
  fi

  template_dir="${E2E_COMPONENT_PATH[${resource_component_key}]:-}/repo-template"
  if [[ ! -d "${template_dir}" ]]; then
    e2e_die "manual profile repo-template sync failed: template dir not found: ${template_dir}"
    return 1
  fi

  mkdir -p "${repo_base_dir}" || {
    e2e_die "manual profile repo-template sync failed: cannot create repo dir: ${repo_base_dir}"
    return 1
  }

  e2e_info "manual profile repo-template sync source=${template_dir} target=${repo_base_dir}"
  cp -a "${template_dir}/." "${repo_base_dir}/" || {
    e2e_die "manual profile repo-template sync failed while copying from ${template_dir} to ${repo_base_dir}"
    return 1
  }

  file_count=$(find "${template_dir}" -type f | wc -l | tr -d ' ')
  e2e_info "manual profile repo-template sync copied-files=${file_count}"
  return 0
}

e2e_manual_init_repo_if_needed() {
  if [[ "${E2E_PROFILE}" != 'manual' ]]; then
    return 0
  fi

  if [[ "${E2E_REPO_TYPE}" != 'git' ]]; then
    return 0
  fi

  e2e_info 'manual profile initializing git repository'
  DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" repo init >/dev/null || {
    e2e_die 'manual profile git repository initialization failed'
    return 1
  }

  e2e_info 'manual profile git repository initialized'
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
    if [[ "${E2E_PROFILE:-}" == 'manual' ]]; then
      e2e_info 'keeping runtime resources for manual profile'
    else
      e2e_info 'keeping runtime resources because --keep-runtime was set'
    fi
  else
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
  local manual_handoff_needed=0

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
  local requested_profile='basic'
  requested_profile=$(e2e_profile_from_cli_args "${E2E_CLI_ARGS[@]}")
  if [[ "${requested_profile}" == 'manual' ]]; then
    E2E_STEPS_TOTAL=5
  else
    E2E_STEPS_TOTAL=7
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
      if [[ "${E2E_PROFILE}" != 'manual' ]]; then
        ui_run_step 6 "${E2E_STEPS_TOTAL}" 'Running Test Cases' step_skip_not_requested || true
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
      if ((E2E_OVERALL_FAILED == 0)) && [[ "${E2E_PROFILE}" != 'manual' ]]; then
        ui_run_step 6 "${E2E_STEPS_TOTAL}" 'Running Test Cases' step_run_workload || E2E_OVERALL_FAILED=1
      fi
    fi
  fi

  if [[ "${E2E_PROFILE}" == 'manual' ]]; then
    if ((E2E_OVERALL_FAILED == 0 && E2E_SHORT_CIRCUIT == 0)); then
      e2e_manual_print_component_access_info || true
      e2e_manual_seed_repo_from_template || E2E_OVERALL_FAILED=1
      if ((E2E_OVERALL_FAILED == 0)); then
        e2e_manual_init_repo_if_needed || E2E_OVERALL_FAILED=1
      fi
      if ((E2E_OVERALL_FAILED == 0)); then
        E2E_KEEP_RUNTIME=1
        manual_handoff_needed=1
      fi
    fi
    step_finalize || true
  else
    ui_run_step 7 "${E2E_STEPS_TOTAL}" 'Finalizing' step_finalize || true
  fi

  ui_print_summary

  if ((manual_handoff_needed == 1)); then
    printf '\n'
    e2e_profile_manual_handoff "${E2E_CONTEXT_NAME}" || E2E_OVERALL_FAILED=1
  fi

  if ((E2E_OVERALL_FAILED == 1 || E2E_CASE_FAILED > 0)); then
    exit 1
  fi

  trap - INT TERM
  exit 0
}

main "$@"
