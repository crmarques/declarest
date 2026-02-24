#!/usr/bin/env bash

e2e_proc_root() {
  printf '%s\n' "${E2E_PROC_ROOT:-/proc}"
}

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
  local proc_root
  proc_root=$(e2e_proc_root)

  [[ -r "${proc_root}/${pid}/cmdline" ]] || return 1

  while IFS= read -r -d '' arg; do
    argv+=("${arg}")
  done <"${proc_root}/${pid}/cmdline" 2>/dev/null || true

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
  local proc_root
  proc_root=$(e2e_proc_root)

  cat "${proc_root}/${pid}/environ" 2>/dev/null | tr '\0' '\n' | grep -Fxq "E2E_RUNNER_PID=${pid}"
}

e2e_runner_pid_matches_run_id() {
  local pid=$1
  local run_id=$2
  local proc_root
  proc_root=$(e2e_proc_root)

  cat "${proc_root}/${pid}/environ" 2>/dev/null | tr '\0' '\n' | grep -Fxq "E2E_RUN_ID=${run_id}"
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

e2e_remove_path_entry() {
  local target=$1
  if [[ -z "${target}" ]]; then
    return 0
  fi

  local path_value="${PATH:-}"
  local -a path_entries=()
  local -a kept=()
  local entry
  local removed=0
  local IFS=':'

  read -ra path_entries <<< "${path_value}"
  for entry in "${path_entries[@]}"; do
    if [[ "${entry}" == "${target}" ]]; then
      removed=1
      continue
    fi
    kept+=("${entry}")
  done

  if ((removed == 0)); then
    return 0
  fi

  if ((${#kept[@]} == 0)); then
    export PATH=''
    return 0
  fi

  local last_index=$(( ${#kept[@]} - 1 ))
  local last_entry=${kept[${last_index}]}
  local new_path

  printf -v new_path '%s:' "${kept[@]}"
  if [[ -z "${last_entry}" ]]; then
    export PATH="${new_path}"
  else
    export PATH="${new_path%:}"
  fi
}

e2e_remove_run_bin_from_path() {
  local run_id=$1
  [[ -n "${run_id}" ]] || return 0
  local run_bin="${E2E_RUNS_DIR}/${run_id}/bin"
  e2e_remove_path_entry "${run_bin}"
}

e2e_cleanup_run_id() {
  local run_id=$1
  local run_dir="${E2E_RUNS_DIR}/${run_id}"

  e2e_validate_cleanup_run_id "${run_id}" || return 1
  e2e_remove_run_bin_from_path "${run_id}"
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
