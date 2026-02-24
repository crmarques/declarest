#!/usr/bin/env bash

e2e_component_collect_manual_info() {
  local component_key=$1
  local script_path

  script_path=$(e2e_component_hook_script "${component_key}" 'manual-info')
  if [[ ! -f "${script_path}" ]]; then
    return 0
  fi

  local state_file
  state_file=$(e2e_component_state_file "${component_key}")
  mkdir -p -- "$(dirname -- "${state_file}")"
  [[ -f "${state_file}" ]] || : >"${state_file}"

  e2e_component_export_env "${component_key}" 'manual-info'
  bash "${script_path}"
}

e2e_components_start_local() {
  E2E_STARTED_COMPONENT_KEYS=()
  e2e_info "starting local compose components with engine=${E2E_CONTAINER_ENGINE}"

  local started_components_file="${E2E_STATE_DIR}/started-components.tsv"
  : >"${started_components_file}"

  local -a start_candidates=()
  local component_key

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    local connection
    connection=$(e2e_component_connection_for_key "${component_key}")

    if [[ "${connection}" != 'local' ]]; then
      e2e_info "component start skipped key=${component_key} reason=connection:${connection}"
      continue
    fi

    if ! e2e_component_runtime_is_compose "${component_key}"; then
      e2e_info "component start skipped key=${component_key} reason=runtime:native"
      continue
    fi

    E2E_COMPONENT_PROJECT["${component_key}"]=$(e2e_component_default_project_name "${component_key}")
    start_candidates+=("${component_key}")
  done

  if ((${#start_candidates[@]} == 0)); then
    return 0
  fi

  e2e_components_run_hook_for_keys 'start' 'true' "${start_candidates[@]}" || return 1

  for component_key in "${start_candidates[@]}"; do
    E2E_STARTED_COMPONENT_KEYS+=("${component_key}")
    printf '%s\t%s\n' "${component_key}" "${E2E_COMPONENT_PROJECT[${component_key}]}" >>"${started_components_file}"
  done

  return 0
}

e2e_components_healthcheck_local() {
  if ((${#E2E_STARTED_COMPONENT_KEYS[@]} == 0)); then
    return 0
  fi

  e2e_components_run_hook_for_keys 'health' 'true' "${E2E_STARTED_COMPONENT_KEYS[@]}"
}

e2e_components_stop_started() {
  local index

  for ((index = ${#E2E_STARTED_COMPONENT_KEYS[@]} - 1; index >= 0; index--)); do
    local component_key=${E2E_STARTED_COMPONENT_KEYS[index]}
    e2e_component_run_hook "${component_key}" 'stop' || true
  done
}
