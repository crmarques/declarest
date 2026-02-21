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

e2e_validate_cleanup_run_id() {
  local run_id=$1

  if [[ -z "${run_id}" ]]; then
    e2e_die 'cleanup run-id must not be empty'
    return 1
  fi

  if [[ "${run_id}" == *'/'* || "${run_id}" == *'..'* ]]; then
    e2e_die "invalid cleanup run-id: ${run_id}"
    return 1
  fi

  if [[ ! "${run_id}" =~ ^[A-Za-z0-9._-]+$ ]]; then
    e2e_die "invalid cleanup run-id: ${run_id}"
    return 1
  fi

  return 0
}

e2e_runner_pid_file_for_run_id() {
  local run_id=$1
  printf '%s/%s/runner.pid\n' "${E2E_RUNS_DIR}" "${run_id}"
}

e2e_is_live_pid() {
  local pid=$1
  [[ "${pid}" =~ ^[0-9]+$ ]] || return 1
  kill -0 "${pid}" >/dev/null 2>&1
}

e2e_runner_cmdline_matches() {
  local pid=$1
  local -a argv=()
  local arg

  [[ -r "/proc/${pid}/cmdline" ]] || return 1

  while IFS= read -r -d '' arg; do
    argv+=("${arg}")
  done </proc/"${pid}"/cmdline 2>/dev/null || true

  ((${#argv[@]} > 0)) || return 1

  case "${argv[0]}" in
    */run-e2e.sh|run-e2e.sh|./test/e2e/run-e2e.sh|test/e2e/run-e2e.sh)
      return 0
      ;;
    */bash|bash|*/sh|sh)
      if ((${#argv[@]} >= 2)); then
        case "${argv[1]}" in
          */run-e2e.sh|run-e2e.sh|./test/e2e/run-e2e.sh|test/e2e/run-e2e.sh)
            return 0
            ;;
        esac
      fi
      ;;
    */env|env)
      if ((${#argv[@]} >= 3)); then
        case "${argv[1]}" in
          */bash|bash|*/sh|sh)
            case "${argv[2]}" in
              */run-e2e.sh|run-e2e.sh|./test/e2e/run-e2e.sh|test/e2e/run-e2e.sh)
                return 0
                ;;
            esac
            ;;
        esac
      fi
      ;;
  esac

  return 1
}

e2e_runner_pid_marker_matches() {
  local pid=$1

  cat /proc/"${pid}"/environ 2>/dev/null | tr '\0' '\n' | grep -Fxq "E2E_RUNNER_PID=${pid}"
}

e2e_runner_pid_matches_run_id() {
  local pid=$1
  local run_id=$2

  cat /proc/"${pid}"/environ 2>/dev/null | tr '\0' '\n' | grep -Fxq "E2E_RUN_ID=${run_id}"
}

e2e_runner_pids_for_run_id_from_ps() {
  local run_id=$1
  local pid
  local ppid
  local args
  local cursor
  local guard
  local proc

  local -a candidate_pids=()
  declare -A ppid_by_pid=()
  declare -A runner_by_pid=()
  declare -A matched_runner=()
  E2E_MATCHED_RUNNER_PIDS=()

  while read -r pid ppid args; do
    [[ "${pid}" =~ ^[0-9]+$ ]] || continue
    [[ "${ppid}" =~ ^[0-9]+$ ]] || continue

    ppid_by_pid["${pid}"]="${ppid}"
    if [[ "${args}" == *"${run_id}"* ]]; then
      candidate_pids+=("${pid}")
    fi
  done < <(ps -eo pid=,ppid=,args= 2>/dev/null)

  for proc in /proc/[0-9]*; do
    pid=${proc#/proc/}
    [[ "${pid}" =~ ^[0-9]+$ ]] || continue
    [[ "${pid}" != "${E2E_RUNNER_PID}" ]] || continue
    e2e_is_live_pid "${pid}" || continue
    e2e_runner_cmdline_matches "${pid}" || continue
    runner_by_pid["${pid}"]=1
  done

  for pid in "${candidate_pids[@]}"; do
    cursor=${pid}
    guard=0

    while [[ -n "${cursor}" && "${cursor}" != '0' && ${guard} -lt 256 ]]; do
      if [[ "${runner_by_pid[${cursor}]:-0}" == '1' ]]; then
        matched_runner["${cursor}"]=1
        break
      fi

      cursor=${ppid_by_pid[${cursor}]:-0}
      ((guard += 1))
    done
  done

  if ((${#matched_runner[@]} == 0)); then
    return 1
  fi

  for pid in "${!matched_runner[@]}"; do
    [[ "${pid}" != "${E2E_RUNNER_PID}" ]] || continue
    E2E_MATCHED_RUNNER_PIDS+=("${pid}")
  done

  if ((${#E2E_MATCHED_RUNNER_PIDS[@]} == 0)); then
    return 1
  fi

  mapfile -t E2E_MATCHED_RUNNER_PIDS < <(printf '%s\n' "${E2E_MATCHED_RUNNER_PIDS[@]}" | sort -n)
  return 0
}

e2e_wait_pid_gone() {
  local pid=$1
  local loops=${2:-50}
  local i

  for ((i = 0; i < loops; i++)); do
    if ! e2e_is_live_pid "${pid}"; then
      return 0
    fi
    sleep 0.1
  done

  return 1
}

e2e_terminate_runner_pid() {
  local pid=$1

  [[ "${pid}" != "${E2E_RUNNER_PID}" ]] || return 0
  e2e_is_live_pid "${pid}" || return 0
  if ! e2e_runner_pid_marker_matches "${pid}"; then
    e2e_runner_cmdline_matches "${pid}" || return 0
  fi

  e2e_info "stopping e2e runner pid=${pid}"

  kill -INT "${pid}" >/dev/null 2>&1 || true
  if e2e_wait_pid_gone "${pid}" 50; then
    return 0
  fi

  kill -TERM "${pid}" >/dev/null 2>&1 || true
  if e2e_wait_pid_gone "${pid}" 50; then
    return 0
  fi

  kill -KILL "${pid}" >/dev/null 2>&1 || true
  if ! e2e_wait_pid_gone "${pid}" 20; then
    e2e_warn "failed to stop e2e runner pid=${pid}"
    return 1
  fi

  return 0
}

e2e_kill_runner_for_run_id() {
  local run_id=$1
  local pid_file
  local pid
  local -A attempted=()
  local remaining=0

  pid_file=$(e2e_runner_pid_file_for_run_id "${run_id}")
  if [[ -f "${pid_file}" ]]; then
    pid=$(head -n 1 "${pid_file}" 2>/dev/null || true)
    if [[ -n "${pid}" ]]; then
      attempted["${pid}"]=1
      e2e_terminate_runner_pid "${pid}" || return 1
    fi
  fi

  for pid in /proc/[0-9]*; do
    pid=${pid#/proc/}
    [[ -n "${pid}" ]] || continue
    [[ "${pid}" =~ ^[0-9]+$ ]] || continue
    [[ "${pid}" != "${E2E_RUNNER_PID}" ]] || continue
    e2e_is_live_pid "${pid}" || continue
    e2e_runner_cmdline_matches "${pid}" || continue
    e2e_runner_pid_matches_run_id "${pid}" "${run_id}" || continue
    [[ "${attempted[${pid}]:-0}" == '1' ]] && continue
    attempted["${pid}"]=1
    e2e_terminate_runner_pid "${pid}" || return 1
  done

  if e2e_runner_pids_for_run_id_from_ps "${run_id}"; then
    for pid in "${E2E_MATCHED_RUNNER_PIDS[@]}"; do
      [[ "${attempted[${pid}]:-0}" == '1' ]] && continue
      attempted["${pid}"]=1
      e2e_terminate_runner_pid "${pid}" || return 1
    done
  fi

  if e2e_runner_pids_for_run_id_from_ps "${run_id}"; then
    for pid in "${E2E_MATCHED_RUNNER_PIDS[@]}"; do
      e2e_warn "failed to stop active runner for run-id=${run_id} pid=${pid}"
      remaining=1
    done
  fi

  if ((remaining == 1)); then
    return 1
  fi

  return 0
}

e2e_kill_all_runner_processes() {
  local pid
  local failed=0

  for pid in /proc/[0-9]*; do
    pid=${pid#/proc/}
    [[ -n "${pid}" ]] || continue
    [[ "${pid}" =~ ^[0-9]+$ ]] || continue
    [[ "${pid}" != "${E2E_RUNNER_PID}" ]] || continue
    e2e_is_live_pid "${pid}" || continue
    e2e_runner_cmdline_matches "${pid}" || continue
    e2e_terminate_runner_pid "${pid}" || failed=1
  done

  if ((failed == 1)); then
    return 1
  fi

  return 0
}

e2e_cleanup_run_containers() {
  local run_id=$1
  local run_dir="${E2E_RUNS_DIR}/${run_id}"
  local selected_file="${run_dir}/state/selected-components.txt"
  local started_file="${run_dir}/state/started-components.tsv"
  local component_key

  e2e_discover_components || return 1
  e2e_validate_container_engine || return 1

  if ! command -v "${E2E_CONTAINER_ENGINE}" >/dev/null 2>&1; then
    e2e_warn "container engine not found; skipping container cleanup: ${E2E_CONTAINER_ENGINE}"
    return 0
  fi

  if [[ -f "${started_file}" ]]; then
    local line
    local project_name
    local compose_file
    while IFS=$'\t' read -r component_key project_name; do
      [[ -n "${component_key}" ]] || continue

      if [[ -z "${E2E_COMPONENT_PATH[${component_key}]:-}" ]]; then
        continue
      fi

      compose_file="${E2E_COMPONENT_PATH[${component_key}]}/compose.yaml"
      [[ -f "${compose_file}" ]] || continue

      if [[ -z "${project_name}" ]]; then
        project_name=$(e2e_sanitize_project_name "declarest-${run_id}-$(e2e_component_type "${component_key}")-$(e2e_component_name "${component_key}")")
      fi

      e2e_info "cleanup container project=${project_name}"
      set +e
      "${E2E_CONTAINER_ENGINE}" compose -f "${compose_file}" -p "${project_name}" down -v --remove-orphans >/dev/null 2>&1
      local rc=$?
      set -e

      if ((rc != 0 && E2E_VERBOSE == 1)); then
        e2e_warn "container cleanup returned rc=${rc} for project=${project_name}"
      fi
    done <"${started_file}"
    return 0
  fi

  if [[ -f "${selected_file}" ]]; then
    while IFS= read -r component_key; do
      [[ -n "${component_key}" ]] || continue
      e2e_component_runtime_is_compose "${component_key}" || continue

      local compose_file="${E2E_COMPONENT_PATH[${component_key}]}/compose.yaml"
      [[ -f "${compose_file}" ]] || continue

      local project_name
      project_name=$(e2e_sanitize_project_name "declarest-${run_id}-$(e2e_component_type "${component_key}")-$(e2e_component_name "${component_key}")")
      e2e_info "cleanup container project=${project_name}"

      set +e
      "${E2E_CONTAINER_ENGINE}" compose -f "${compose_file}" -p "${project_name}" down -v --remove-orphans >/dev/null 2>&1
      local rc=$?
      set -e

      if ((rc != 0 && E2E_VERBOSE == 1)); then
        e2e_warn "container cleanup returned rc=${rc} for project=${project_name}"
      fi
    done <"${selected_file}"
    return 0
  fi

  e2e_warn "run state missing selected/started component records for ${run_id}; falling back to all container components"
  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    if ! e2e_component_runtime_is_compose "${component_key}"; then
      continue
    fi

    local compose_file="${E2E_COMPONENT_PATH[${component_key}]}/compose.yaml"
    [[ -f "${compose_file}" ]] || continue

    local project_name
    project_name=$(e2e_sanitize_project_name "declarest-${run_id}-$(e2e_component_type "${component_key}")-$(e2e_component_name "${component_key}")")
    e2e_info "cleanup container project=${project_name}"

    set +e
    "${E2E_CONTAINER_ENGINE}" compose -f "${compose_file}" -p "${project_name}" down -v --remove-orphans >/dev/null 2>&1
    local rc=$?
    set -e

    if ((rc != 0 && E2E_VERBOSE == 1)); then
      e2e_warn "container cleanup returned rc=${rc} for project=${project_name}"
    fi
  done

  return 0
}

e2e_cleanup_run_id() {
  local run_id=$1
  local run_dir="${E2E_RUNS_DIR}/${run_id}"

  e2e_validate_cleanup_run_id "${run_id}" || return 1
  e2e_kill_runner_for_run_id "${run_id}" || return 1
  e2e_cleanup_run_containers "${run_id}" || return 1

  if [[ -d "${run_dir}" ]]; then
    rm -rf "${run_dir}" || {
      e2e_die "failed to remove run directory: ${run_dir}"
      return 1
    }
    e2e_info "removed run directory: ${run_dir}"
  else
    e2e_warn "run directory not found: ${run_dir}"
  fi

  rm -f "$(e2e_runner_pid_file_for_run_id "${run_id}")" || true
  return 0
}

e2e_cleanup_all_runs() {
  if [[ ! -d "${E2E_RUNS_DIR}" ]]; then
    e2e_info "no runs directory found: ${E2E_RUNS_DIR}"
    return 0
  fi

  local -a run_ids=()
  local run_id
  local run_path

  while IFS= read -r run_path; do
    [[ -n "${run_path}" ]] || continue
    run_id=$(basename -- "${run_path}")
    run_ids+=("${run_id}")
  done < <(find "${E2E_RUNS_DIR}" -mindepth 1 -maxdepth 1 -type d | sort)

  if ((${#run_ids[@]} == 0)); then
    e2e_info "no run directories found under ${E2E_RUNS_DIR}"
    return 0
  fi

  e2e_kill_all_runner_processes || true

  local failed=0
  for run_id in "${run_ids[@]}"; do
    e2e_cleanup_run_id "${run_id}" || failed=1
  done

  if ((failed == 1)); then
    return 1
  fi

  return 0
}

e2e_handle_cleanup_mode() {
  if [[ -n "${E2E_CLEAN_RUN_ID}" ]]; then
    e2e_cleanup_run_id "${E2E_CLEAN_RUN_ID}"
    return $?
  fi

  if ((E2E_CLEAN_ALL == 1)); then
    e2e_cleanup_all_runs
    return $?
  fi

  e2e_die 'cleanup mode requested but no target was provided'
  return 1
}

e2e_handle_termination_signal() {
  local signal_name=$1

  if ((E2E_SIGNAL_HANDLED == 1)); then
    return
  fi
  E2E_SIGNAL_HANDLED=1

  printf '\n' >&2
  ui_spinner_stop || true
  e2e_warn "received ${signal_name}; stopping e2e run"
  step_finalize || true
  [[ -n "${E2E_PID_FILE}" && -f "${E2E_PID_FILE}" ]] && rm -f "${E2E_PID_FILE}" || true
  exit 130
}

step_initialize() {
  e2e_parse_args "${E2E_CLI_ARGS[@]}" || return 1
  e2e_apply_profile_defaults || return 1
  e2e_validate_container_engine || return 1

  e2e_discover_components || return 1

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
