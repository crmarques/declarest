#!/usr/bin/env bash

e2e_component_hook_script() {
  local component_key=$1
  local hook_name=$2
  printf '%s/scripts/%s.sh\n' "${E2E_COMPONENT_PATH[${component_key}]}" "${hook_name}"
}

e2e_component_export_env() {
  local component_key=$1
  local hook_name=$2
  local component_type
  local component_name

  component_type=$(e2e_component_type "${component_key}")
  component_name=$(e2e_component_name "${component_key}")

  export E2E_COMPONENT_KEY="${component_key}"
  export E2E_COMPONENT_TYPE="${component_type}"
  export E2E_COMPONENT_NAME="${component_name}"
  export E2E_COMPONENT_DIR="${E2E_COMPONENT_PATH[${component_key}]}"
  export E2E_COMPONENT_HOOK="${hook_name}"
  export E2E_COMPONENT_CONNECTION="$(e2e_component_connection_for_key "${component_key}")"
  export E2E_COMPONENT_RUNTIME_KIND="${E2E_COMPONENT_RUNTIME_KIND[${component_key}]:-native}"
  export E2E_COMPONENT_DEPENDS_ON="${E2E_COMPONENT_DEPENDS_ON[${component_key}]:-}"
  export E2E_COMPONENT_STATE_FILE="$(e2e_component_state_file "${component_key}")"
  export E2E_COMPONENT_PROJECT_NAME="${E2E_COMPONENT_PROJECT[${component_key}]:-}"
  export E2E_COMPONENT_CONTEXT_FRAGMENT="$(e2e_component_context_fragment_path "${component_key}")"
  export E2E_COMPONENT_OPENAPI_SPEC="${E2E_COMPONENT_OPENAPI_SPEC[${component_key}]:-}"
  export E2E_METADATA_DIR
  export E2E_ROOT_DIR
  export E2E_DIR
  export E2E_RUN_DIR
  export E2E_STATE_DIR
  export E2E_LOG_DIR
  export E2E_CONTEXT_DIR
  export E2E_CONTEXT_FILE

  export E2E_RESOURCE_SERVER
  export E2E_RESOURCE_SERVER_CONNECTION
  export E2E_RESOURCE_SERVER_BASIC_AUTH
  export E2E_RESOURCE_SERVER_OAUTH2
  export E2E_RESOURCE_SERVER_MTLS
  export E2E_REPO_TYPE
  export E2E_GIT_PROVIDER
  export E2E_GIT_PROVIDER_CONNECTION
  export E2E_SECRET_PROVIDER
  export E2E_SECRET_PROVIDER_CONNECTION
}

e2e_component_source_state_env() {
  local state_file=$1

  if [[ -f "${state_file}" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "${state_file}"
    set +a
  fi
}

e2e_component_runtime_is_compose() {
  local component_key=$1
  [[ "${E2E_COMPONENT_RUNTIME_KIND[${component_key}]:-native}" == 'compose' ]]
}

e2e_sanitize_project_name() {
  local value=$1
  value=${value//[^a-zA-Z0-9]/-}
  printf '%s\n' "${value,,}"
}

e2e_component_default_project_name() {
  local component_key=$1
  e2e_sanitize_project_name "declarest-${E2E_RUN_ID}-$(e2e_component_type "${component_key}")-$(e2e_component_name "${component_key}")"
}

e2e_component_builtin_start_compose() {
  local component_key=$1
  local connection
  local compose_file
  local state_file
  local project_name

  connection=$(e2e_component_connection_for_key "${component_key}")
  if [[ "${connection}" != 'local' ]]; then
    e2e_info "component start skipped key=${component_key} reason=connection:${connection}"
    return 0
  fi

  if ! e2e_component_runtime_is_compose "${component_key}"; then
    e2e_info "component start skipped key=${component_key} reason=runtime:native"
    return 0
  fi

  compose_file="${E2E_COMPONENT_PATH[${component_key}]}/compose.yaml"
  if [[ ! -f "${compose_file}" ]]; then
    e2e_die "missing compose file for ${component_key}: ${compose_file}"
    return 1
  fi

  state_file=$(e2e_component_state_file "${component_key}")
  project_name="${E2E_COMPONENT_PROJECT[${component_key}]:-}"
  if [[ -z "${project_name}" ]]; then
    project_name=$(e2e_component_default_project_name "${component_key}")
    E2E_COMPONENT_PROJECT["${component_key}"]="${project_name}"
  fi

  e2e_info "component start key=${component_key} project=${project_name} compose=${compose_file}"

  (
    e2e_component_source_state_env "${state_file}"
    e2e_compose_cmd -f "${compose_file}" -p "${project_name}" up -d
    e2e_compose_cmd -f "${compose_file}" -p "${project_name}" ps || true
  ) || {
    e2e_error "component start failed key=${component_key} project=${project_name}; collecting compose diagnostics"
    (
      e2e_component_source_state_env "${state_file}"
      e2e_compose_cmd -f "${compose_file}" -p "${project_name}" ps || true
      e2e_compose_cmd -f "${compose_file}" -p "${project_name}" logs || true
    )
    return 1
  }

  return 0
}

e2e_component_builtin_stop_compose() {
  local component_key=$1
  local compose_file
  local state_file
  local project_name

  if ! e2e_component_runtime_is_compose "${component_key}"; then
    return 0
  fi

  compose_file="${E2E_COMPONENT_PATH[${component_key}]}/compose.yaml"
  [[ -f "${compose_file}" ]] || return 0

  project_name="${E2E_COMPONENT_PROJECT[${component_key}]:-}"
  if [[ -z "${project_name}" ]]; then
    project_name=$(e2e_component_default_project_name "${component_key}")
  fi

  state_file=$(e2e_component_state_file "${component_key}")

  e2e_info "component stop key=${component_key} project=${project_name}"
  (
    e2e_component_source_state_env "${state_file}"
    e2e_compose_cmd -f "${compose_file}" -p "${project_name}" down -v --remove-orphans
  ) || true

  return 0
}

e2e_component_run_hook() {
  local component_key=$1
  local hook_name=$2
  shift 2

  local script_path
  script_path=$(e2e_component_hook_script "${component_key}" "${hook_name}")

  local state_file
  state_file=$(e2e_component_state_file "${component_key}")
  mkdir -p -- "$(dirname -- "${state_file}")"
  [[ -f "${state_file}" ]] || : >"${state_file}"

  local connection
  connection=$(e2e_component_connection_for_key "${component_key}")

  e2e_component_export_env "${component_key}" "${hook_name}"

  if [[ -f "${script_path}" ]]; then
    e2e_info "component-hook start key=${component_key} hook=${hook_name} connection=${connection} script=${script_path}"

    if ! bash "${script_path}" "$@"; then
      e2e_error "component-hook failed key=${component_key} hook=${hook_name} script=${script_path}"
      return 1
    fi

    e2e_info "component-hook done key=${component_key} hook=${hook_name}"
    return 0
  fi

  case "${hook_name}" in
    start)
      e2e_component_builtin_start_compose "${component_key}" || return 1
      ;;
    stop)
      e2e_component_builtin_stop_compose "${component_key}" || return 1
      ;;
    *)
      return 0
      ;;
  esac

  return 0
}

e2e_component_dependency_keys() {
  local component_key=$1
  local -n selected_ref=$2
  local dependency_spec
  local token
  local dependency_type
  local dependency_name
  local dependency_key
  local candidate
  local found
  local -A resolved=()

  dependency_spec="${E2E_COMPONENT_DEPENDS_ON[${component_key}]:-}"
  for token in ${dependency_spec}; do
    [[ -n "${token}" ]] || continue

    dependency_type=${token%%:*}
    dependency_name=${token#*:}

    if [[ "${dependency_name}" == '*' ]]; then
      found=0
      for candidate in "${!selected_ref[@]}"; do
        if [[ "$(e2e_component_type "${candidate}")" != "${dependency_type}" ]]; then
          continue
        fi
        if [[ "${selected_ref[${candidate}]:-0}" != '1' ]]; then
          continue
        fi
        resolved["${candidate}"]=1
        found=1
      done

      if ((found == 0)); then
        e2e_die "component ${component_key} dependency selector ${token} did not match any selected component"
        return 1
      fi
      continue
    fi

    dependency_key=$(e2e_component_key "${dependency_type}" "${dependency_name}")
    if [[ "${selected_ref[${dependency_key}]:-0}" != '1' ]]; then
      e2e_die "component ${component_key} dependency ${dependency_key} is not selected"
      return 1
    fi
    resolved["${dependency_key}"]=1
  done

  for dependency_key in "${!resolved[@]}"; do
    printf '%s\n' "${dependency_key}"
  done | sort
}

e2e_components_hook_batch_log_dir() {
  local hook_name=$1
  local safe_hook_name
  safe_hook_name=${hook_name//[^a-zA-Z0-9._-]/-}

  if [[ -n "${E2E_LOG_DIR:-}" ]]; then
    local artifact_root="${E2E_LOG_DIR}/component-hooks"
    mkdir -p "${artifact_root}" || {
      e2e_warn "failed to create component hook log artifact directory: ${artifact_root}"
      mktemp -d "/tmp/declarest-e2e-hook-${safe_hook_name}.XXXXXX"
      return
    }
    mktemp -d "${artifact_root}/${safe_hook_name}.XXXXXX"
    return
  fi

  mktemp -d "/tmp/declarest-e2e-hook-${safe_hook_name}.XXXXXX"
}

e2e_components_run_hook_batch_parallel() {
  local hook_name=$1
  shift
  local -a batch=("$@")

  if ((${#batch[@]} == 0)); then
    return 0
  fi

  local tmp_dir
  tmp_dir=$(e2e_components_hook_batch_log_dir "${hook_name}") || return 1
  local keep_artifacts=0
  if [[ -n "${E2E_LOG_DIR:-}" && "${tmp_dir}" == "${E2E_LOG_DIR}/component-hooks/"* ]]; then
    keep_artifacts=1
  fi

  local -a pids=()
  local -a keys=()
  local -a logs=()
  local -a rcs=()
  local component_key

  for component_key in "${batch[@]}"; do
    local safe_key
    local log_file

    safe_key=${component_key//[:\/]/-}
    log_file="${tmp_dir}/${safe_key}.log"

    (
      e2e_component_run_hook "${component_key}" "${hook_name}"
    ) >"${log_file}" 2>&1 &

    pids+=("$!")
    keys+=("${component_key}")
    logs+=("${log_file}")
  done

  local failed=0
  local idx
  local pid
  local rc

  for idx in "${!pids[@]}"; do
    pid=${pids[${idx}]}
    set +e
    wait "${pid}"
    rc=$?
    set -e

    rcs[${idx}]="${rc}"
    if ((rc != 0)); then
      failed=1
    fi
  done

  for idx in "${!keys[@]}"; do
    component_key=${keys[${idx}]}
    rc=${rcs[${idx}]}

    if ((E2E_VERBOSE == 1 || rc != 0)); then
      while IFS= read -r line; do
        printf '[%s] %s\n' "${component_key}" "${line}"
      done <"${logs[${idx}]}"
    fi

    if ((rc != 0)); then
      e2e_error "component hook failed key=${component_key} hook=${hook_name}"
    fi
  done

  if ((failed == 1 && keep_artifacts == 1)); then
    e2e_error "parallel hook logs retained dir=${tmp_dir}"
  elif ((E2E_VERBOSE == 1 && keep_artifacts == 1)); then
    e2e_info "parallel hook logs retained dir=${tmp_dir}"
  fi

  if ((keep_artifacts == 0)); then
    rm -rf "${tmp_dir}" || true
  fi

  if ((failed == 1)); then
    return 1
  fi

  return 0
}

e2e_components_run_hook_for_keys() {
  local hook_name=$1
  local parallel_mode=${2:-false}
  shift 2
  local -a target_keys=("$@")

  if ((${#target_keys[@]} == 0)); then
    return 0
  fi

  local -A selected_set=()
  local -A target_set=()
  local -A done_set=()
  local component_key

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    selected_set["${component_key}"]=1
  done

  for component_key in "${target_keys[@]}"; do
    target_set["${component_key}"]=1
  done

  local -a pending=("${target_keys[@]}")

  while ((${#pending[@]} > 0)); do
    local -a batch=()

    for component_key in "${pending[@]}"; do
      local -a dependencies=()
      local dep
      local ready=1

      mapfile -t dependencies < <(e2e_component_dependency_keys "${component_key}" selected_set) || return 1

      for dep in "${dependencies[@]}"; do
        if [[ "${target_set[${dep}]:-0}" != '1' ]]; then
          continue
        fi
        if [[ "${done_set[${dep}]:-0}" != '1' ]]; then
          ready=0
          break
        fi
      done

      if ((ready == 1)); then
        batch+=("${component_key}")
      fi
    done

    if ((${#batch[@]} == 0)); then
      e2e_die "dependency cycle detected while running hook ${hook_name} for components: ${pending[*]}"
      return 1
    fi

    if [[ "${parallel_mode}" == 'true' ]] && ((${#batch[@]} > 1)); then
      e2e_components_run_hook_batch_parallel "${hook_name}" "${batch[@]}" || return 1
    else
      for component_key in "${batch[@]}"; do
        e2e_component_run_hook "${component_key}" "${hook_name}" || return 1
      done
    fi

    for component_key in "${batch[@]}"; do
      done_set["${component_key}"]=1
    done

    local -a next_pending=()
    for component_key in "${pending[@]}"; do
      if [[ "${done_set[${component_key}]:-0}" != '1' ]]; then
        next_pending+=("${component_key}")
      fi
    done
    pending=("${next_pending[@]}")
  done

  return 0
}

e2e_components_run_hook_all() {
  local hook_name=$1
  local parallel_mode=${2:-false}
  e2e_components_run_hook_for_keys "${hook_name}" "${parallel_mode}" "${E2E_SELECTED_COMPONENT_KEYS[@]}"
}
