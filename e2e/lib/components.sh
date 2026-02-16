#!/usr/bin/env bash

declare -Ag E2E_COMPONENT_PATH=()
declare -Ag E2E_COMPONENT_CONNECTIONS=()
declare -Ag E2E_COMPONENT_DEFAULT_CONNECTION=()
declare -Ag E2E_COMPONENT_REQUIRES_DOCKER=()
declare -Ag E2E_COMPONENT_DESCRIPTION=()
declare -Ag E2E_COMPONENT_PROJECT=()
declare -Ag E2E_CAPABILITY_SET=()

declare -ag E2E_COMPONENT_KEYS=()
declare -ag E2E_SELECTED_COMPONENT_KEYS=()
declare -ag E2E_STARTED_COMPONENT_KEYS=()

e2e_component_key() {
  local component_type=$1
  local component_name=$2
  printf '%s:%s\n' "${component_type}" "${component_name}"
}

e2e_component_type() {
  printf '%s\n' "${1%%:*}"
}

e2e_component_name() {
  printf '%s\n' "${1#*:}"
}

e2e_component_connection_for_key() {
  local component_key=$1
  local component_type

  component_type=$(e2e_component_type "${component_key}")
  case "${component_type}" in
    resource-server)
      printf '%s\n' "${E2E_RESOURCE_SERVER_CONNECTION}"
      ;;
    git-provider)
      printf '%s\n' "${E2E_GIT_PROVIDER_CONNECTION}"
      ;;
    secret-provider)
      printf '%s\n' "${E2E_SECRET_PROVIDER_CONNECTION}"
      ;;
    repo-type)
      printf 'local\n'
      ;;
    *)
      printf 'local\n'
      ;;
  esac
}

e2e_component_state_file() {
  local component_key=$1
  local component_type
  local component_name

  component_type=$(e2e_component_type "${component_key}")
  component_name=$(e2e_component_name "${component_key}")

  printf '%s/%s-%s.env\n' "${E2E_STATE_DIR}" "${component_type}" "${component_name}"
}

e2e_discover_components() {
  E2E_COMPONENT_KEYS=()

  local component_file
  while IFS= read -r component_file; do
    [[ -n "${component_file}" ]] || continue

    local metadata
    metadata=$(
      (
        # shellcheck disable=SC1090
        source "${component_file}"
        printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
          "${COMPONENT_TYPE}" \
          "${COMPONENT_NAME}" \
          "${SUPPORTED_CONNECTIONS}" \
          "${DEFAULT_CONNECTION}" \
          "${REQUIRES_DOCKER}" \
          "${DESCRIPTION:-}"
      )
    )

    local component_type
    local component_name
    local supported_connections
    local default_connection
    local requires_docker
    local description

    IFS=$'\t' read -r component_type component_name supported_connections default_connection requires_docker description <<<"${metadata}"

    local component_key
    component_key=$(e2e_component_key "${component_type}" "${component_name}")

    E2E_COMPONENT_PATH["${component_key}"]=$(dirname "${component_file}")
    E2E_COMPONENT_CONNECTIONS["${component_key}"]="${supported_connections}"
    E2E_COMPONENT_DEFAULT_CONNECTION["${component_key}"]="${default_connection}"
    E2E_COMPONENT_REQUIRES_DOCKER["${component_key}"]="${requires_docker}"
    E2E_COMPONENT_DESCRIPTION["${component_key}"]="${description}"
    E2E_COMPONENT_KEYS+=("${component_key}")
  done < <(find "${E2E_DIR}/components" -type f -name 'component.env' | sort)
}

e2e_list_components() {
  printf 'Discovered e2e components\n'
  printf '%s\n' '------------------------'
  printf '%-18s %-14s %-18s %-10s %s\n' 'TYPE' 'NAME' 'CONNECTIONS' 'RUNTIME' 'DESCRIPTION'

  local component_key
  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    local component_type
    local component_name

    component_type=$(e2e_component_type "${component_key}")
    component_name=$(e2e_component_name "${component_key}")

    printf '%-18s %-14s %-18s %-10s %s\n' \
      "${component_type}" \
      "${component_name}" \
      "${E2E_COMPONENT_CONNECTIONS[${component_key}]}" \
      "${E2E_COMPONENT_REQUIRES_DOCKER[${component_key}]}" \
      "${E2E_COMPONENT_DESCRIPTION[${component_key}]}"
  done
}

e2e_component_exists() {
  local component_type=$1
  local component_name=$2
  local component_key

  component_key=$(e2e_component_key "${component_type}" "${component_name}")
  [[ -n "${E2E_COMPONENT_PATH[${component_key}]:-}" ]]
}

e2e_component_supports_connection() {
  local component_type=$1
  local component_name=$2
  local connection=$3
  local component_key

  component_key=$(e2e_component_key "${component_type}" "${component_name}")

  local supported
  supported=" ${E2E_COMPONENT_CONNECTIONS[${component_key}]:-} "
  [[ "${supported}" == *" ${connection} "* ]]
}

e2e_validate_resource_server_fixture_tree() {
  local component_name=$1
  local component_key
  component_key=$(e2e_component_key 'resource-server' "${component_name}")

  local component_dir="${E2E_COMPONENT_PATH[${component_key}]:-}"
  if [[ -z "${component_dir}" ]]; then
    e2e_die "resource-server component path not found: ${component_name}"
    return 1
  fi

  local template_dir="${component_dir}/repo-template"
  if [[ ! -d "${template_dir}" ]]; then
    e2e_die "resource-server ${component_name} missing repo-template fixture tree: ${template_dir}"
    return 1
  fi

  local -a metadata_files=()
  local metadata_file
  while IFS= read -r metadata_file; do
    [[ -n "${metadata_file}" ]] || continue
    metadata_files+=("${metadata_file}")
  done < <(find "${template_dir}" -type f -path '*/_/metadata.json' | sort)

  if ((${#metadata_files[@]} == 0)); then
    e2e_die "resource-server ${component_name} has no collection metadata fixtures under ${template_dir}"
    return 1
  fi

  local -a payload_files=()
  local payload_file
  while IFS= read -r payload_file; do
    [[ -n "${payload_file}" ]] || continue
    payload_files+=("${payload_file}")
  done < <(find "${template_dir}" -type f -name '*.json' ! -path '*/_/metadata.json' | sort)

  if ((${#payload_files[@]} == 0)); then
    e2e_die "resource-server ${component_name} repo-template has no resource payload files under ${template_dir}"
    return 1
  fi

  for payload_file in "${payload_files[@]}"; do
    local payload_rel
    payload_rel=${payload_file#${template_dir}/}
    if [[ "$(basename -- "${payload_rel}")" != 'resource.json' ]]; then
      e2e_die "resource-server ${component_name} has invalid resource payload fixture path: ${payload_rel} (expected */resource.json)"
      return 1
    fi
  done

  for metadata_file in "${metadata_files[@]}"; do
    local rel
    rel=${metadata_file#${template_dir}/}
    if [[ "${rel}" != *_/metadata.json ]]; then
      e2e_die "resource-server ${component_name} has invalid metadata fixture path: ${rel} (expected */_/metadata.json)"
      return 1
    fi
  done
}

e2e_validate_selection() {
  if [[ "${E2E_RESOURCE_SERVER}" != 'none' ]] && ! e2e_component_exists 'resource-server' "${E2E_RESOURCE_SERVER}"; then
    e2e_die "unknown resource-server component: ${E2E_RESOURCE_SERVER}"
    return 1
  fi

  if ! e2e_component_exists 'repo-type' "${E2E_REPO_TYPE}"; then
    e2e_die "unknown repo-type component: ${E2E_REPO_TYPE}"
    return 1
  fi

  if [[ -n "${E2E_GIT_PROVIDER}" ]] && ! e2e_component_exists 'git-provider' "${E2E_GIT_PROVIDER}"; then
    e2e_die "unknown git-provider component: ${E2E_GIT_PROVIDER}"
    return 1
  fi

  if [[ "${E2E_SECRET_PROVIDER}" != 'none' ]] && ! e2e_component_exists 'secret-provider' "${E2E_SECRET_PROVIDER}"; then
    e2e_die "unknown secret-provider component: ${E2E_SECRET_PROVIDER}"
    return 1
  fi

  if [[ "${E2E_RESOURCE_SERVER}" != 'none' ]] && ! e2e_component_supports_connection 'resource-server' "${E2E_RESOURCE_SERVER}" "${E2E_RESOURCE_SERVER_CONNECTION}"; then
    e2e_die "resource-server ${E2E_RESOURCE_SERVER} does not support connection ${E2E_RESOURCE_SERVER_CONNECTION}"
    return 1
  fi

  if [[ "${E2E_RESOURCE_SERVER}" != 'none' ]]; then
    e2e_validate_resource_server_fixture_tree "${E2E_RESOURCE_SERVER}" || return 1
  fi

  if [[ -n "${E2E_GIT_PROVIDER}" ]] && ! e2e_component_supports_connection 'git-provider' "${E2E_GIT_PROVIDER}" "${E2E_GIT_PROVIDER_CONNECTION}"; then
    e2e_die "git-provider ${E2E_GIT_PROVIDER} does not support connection ${E2E_GIT_PROVIDER_CONNECTION}"
    return 1
  fi

  if [[ "${E2E_SECRET_PROVIDER}" != 'none' ]] && ! e2e_component_supports_connection 'secret-provider' "${E2E_SECRET_PROVIDER}" "${E2E_SECRET_PROVIDER_CONNECTION}"; then
    e2e_die "secret-provider ${E2E_SECRET_PROVIDER} does not support connection ${E2E_SECRET_PROVIDER_CONNECTION}"
    return 1
  fi

  if [[ "${E2E_REPO_TYPE}" == 'git' && -z "${E2E_GIT_PROVIDER}" ]]; then
    e2e_die '--repo-type git requires --git-provider'
    return 1
  fi

  if [[ "${E2E_REPO_TYPE}" != 'git' && -n "${E2E_GIT_PROVIDER}" ]]; then
    e2e_die '--git-provider requires --repo-type git'
    return 1
  fi
}

e2e_build_selected_components() {
  E2E_SELECTED_COMPONENT_KEYS=()

  if [[ -n "${E2E_GIT_PROVIDER}" ]]; then
    E2E_SELECTED_COMPONENT_KEYS+=("$(e2e_component_key 'git-provider' "${E2E_GIT_PROVIDER}")")
  fi

  if [[ "${E2E_RESOURCE_SERVER}" != 'none' ]]; then
    E2E_SELECTED_COMPONENT_KEYS+=("$(e2e_component_key 'resource-server' "${E2E_RESOURCE_SERVER}")")
  fi

  if [[ "${E2E_SECRET_PROVIDER}" != 'none' ]]; then
    E2E_SELECTED_COMPONENT_KEYS+=("$(e2e_component_key 'secret-provider' "${E2E_SECRET_PROVIDER}")")
  fi

  E2E_SELECTED_COMPONENT_KEYS+=("$(e2e_component_key 'repo-type' "${E2E_REPO_TYPE}")")
}

e2e_build_capabilities() {
  E2E_CAPABILITY_SET=()

  E2E_CAPABILITY_SET["profile=${E2E_PROFILE}"]=1
  E2E_CAPABILITY_SET["repo-type=${E2E_REPO_TYPE}"]=1
  E2E_CAPABILITY_SET["resource-server=${E2E_RESOURCE_SERVER}"]=1
  E2E_CAPABILITY_SET["resource-server-connection=${E2E_RESOURCE_SERVER_CONNECTION}"]=1
  E2E_CAPABILITY_SET["secret-provider=${E2E_SECRET_PROVIDER}"]=1
  E2E_CAPABILITY_SET["secret-provider-connection=${E2E_SECRET_PROVIDER_CONNECTION}"]=1

  if [[ -n "${E2E_GIT_PROVIDER}" ]]; then
    E2E_CAPABILITY_SET["git-provider=${E2E_GIT_PROVIDER}"]=1
    E2E_CAPABILITY_SET["git-provider-connection=${E2E_GIT_PROVIDER_CONNECTION}"]=1
  fi

  if [[ "${E2E_SECRET_PROVIDER}" != 'none' ]]; then
    E2E_CAPABILITY_SET['has-secret-provider']=1
  fi

  if [[ "${E2E_RESOURCE_SERVER}" != 'none' ]]; then
    E2E_CAPABILITY_SET['has-resource-server']=1
  fi

  if [[ "${E2E_GIT_PROVIDER_CONNECTION}" == 'remote' || "${E2E_RESOURCE_SERVER_CONNECTION}" == 'remote' || "${E2E_SECRET_PROVIDER_CONNECTION}" == 'remote' ]]; then
    E2E_CAPABILITY_SET['remote-selection']=1
  fi
}

e2e_has_capability() {
  local capability=$1
  [[ "${E2E_CAPABILITY_SET[${capability}]:-0}" == '1' ]]
}

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
  export E2E_COMPONENT_STATE_FILE="$(e2e_component_state_file "${component_key}")"
  export E2E_COMPONENT_PROJECT_NAME="${E2E_COMPONENT_PROJECT[${component_key}]:-}"
  export E2E_ROOT_DIR
  export E2E_DIR
  export E2E_RUN_DIR
  export E2E_STATE_DIR
  export E2E_LOG_DIR
  export E2E_CONTEXT_DIR
  export E2E_CONTEXT_FILE

  export E2E_RESOURCE_SERVER
  export E2E_RESOURCE_SERVER_CONNECTION
  export E2E_REPO_TYPE
  export E2E_GIT_PROVIDER
  export E2E_GIT_PROVIDER_CONNECTION
  export E2E_SECRET_PROVIDER
  export E2E_SECRET_PROVIDER_CONNECTION
}

e2e_component_run_hook() {
  local component_key=$1
  local hook_name=$2
  shift 2

  local script_path
  script_path=$(e2e_component_hook_script "${component_key}" "${hook_name}")

  if [[ ! -f "${script_path}" ]]; then
    return 0
  fi

  local state_file
  state_file=$(e2e_component_state_file "${component_key}")
  mkdir -p -- "$(dirname -- "${state_file}")"
  [[ -f "${state_file}" ]] || : >"${state_file}"

  local connection
  connection=$(e2e_component_connection_for_key "${component_key}")

  e2e_component_export_env "${component_key}" "${hook_name}"
  e2e_info "component-hook start key=${component_key} hook=${hook_name} connection=${connection} script=${script_path}"

  if ! bash "${script_path}" "$@"; then
    e2e_error "component-hook failed key=${component_key} hook=${hook_name} script=${script_path}"
    return 1
  fi

  e2e_info "component-hook done key=${component_key} hook=${hook_name}"
}

e2e_components_run_hook_all() {
  local hook_name=$1
  local component_key

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    e2e_component_run_hook "${component_key}" "${hook_name}" || return 1
  done
}

e2e_sanitize_project_name() {
  local value=$1
  value=${value//[^a-zA-Z0-9]/-}
  printf '%s\n' "${value,,}"
}

e2e_components_start_local() {
  E2E_STARTED_COMPONENT_KEYS=()
  e2e_info "starting local container components with engine=${E2E_CONTAINER_ENGINE}"
  local started_components_file="${E2E_STATE_DIR}/started-components.tsv"
  : >"${started_components_file}"

  local component_key
  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    local connection
    connection=$(e2e_component_connection_for_key "${component_key}")
    if [[ "${connection}" != 'local' ]]; then
      e2e_info "component start skipped key=${component_key} reason=connection:${connection}"
      continue
    fi

    if [[ "${E2E_COMPONENT_REQUIRES_DOCKER[${component_key}]:-false}" != 'true' ]]; then
      e2e_info "component start skipped key=${component_key} reason=non-container"
      continue
    fi

    local compose_file="${E2E_COMPONENT_PATH[${component_key}]}/compose.yaml"
    if [[ ! -f "${compose_file}" ]]; then
      e2e_die "missing compose file for ${component_key}: ${compose_file}"
      return 1
    fi

    local state_file
    state_file=$(e2e_component_state_file "${component_key}")
    if [[ -f "${state_file}" ]]; then
      # shellcheck disable=SC1090
      set -a; source "${state_file}"; set +a
    fi

    local project_name
    project_name=$(e2e_sanitize_project_name "declarest-${E2E_RUN_ID}-$(e2e_component_type "${component_key}")-$(e2e_component_name "${component_key}")")
    E2E_COMPONENT_PROJECT["${component_key}"]="${project_name}"
    e2e_info "component start key=${component_key} project=${project_name} compose=${compose_file}"

    if ! e2e_compose_cmd -f "${compose_file}" -p "${project_name}" up -d; then
      e2e_error "component start failed key=${component_key} project=${project_name}; collecting compose diagnostics"
      e2e_compose_cmd -f "${compose_file}" -p "${project_name}" ps || true
      e2e_compose_cmd -f "${compose_file}" -p "${project_name}" logs || true
      return 1
    fi
    e2e_compose_cmd -f "${compose_file}" -p "${project_name}" ps || true
    E2E_STARTED_COMPONENT_KEYS+=("${component_key}")
    printf '%s\t%s\n' "${component_key}" "${project_name}" >>"${started_components_file}"
  done
}

e2e_components_healthcheck_local() {
  local component_key

  for component_key in "${E2E_STARTED_COMPONENT_KEYS[@]}"; do
    e2e_info "component healthcheck start key=${component_key}"
    e2e_component_run_hook "${component_key}" 'health' || return 1
    e2e_info "component healthcheck done key=${component_key}"
  done
}

e2e_components_stop_started() {
  local index
  for ((index = ${#E2E_STARTED_COMPONENT_KEYS[@]} - 1; index >= 0; index--)); do
    local component_key=${E2E_STARTED_COMPONENT_KEYS[index]}
    local compose_file="${E2E_COMPONENT_PATH[${component_key}]}/compose.yaml"
    local project_name="${E2E_COMPONENT_PROJECT[${component_key}]:-}"

    if [[ -z "${project_name}" || ! -f "${compose_file}" ]]; then
      continue
    fi

    e2e_info "component stop key=${component_key} project=${project_name}"
    e2e_compose_cmd -f "${compose_file}" -p "${project_name}" down -v --remove-orphans || true
  done
}

e2e_preflight_requirements() {
  e2e_info 'preflight checking required commands: bash go git jq curl'
  e2e_require_command bash || return 1
  e2e_require_command go || return 1
  e2e_require_command git || return 1
  e2e_require_command jq || return 1
  e2e_require_command curl || return 1

  local needs_container_runtime=0
  local component_key
  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    local connection
    connection=$(e2e_component_connection_for_key "${component_key}")
    if [[ "${connection}" == 'local' && "${E2E_COMPONENT_REQUIRES_DOCKER[${component_key}]:-false}" == 'true' ]]; then
      needs_container_runtime=1
      break
    fi
  done

  if ((needs_container_runtime == 1)); then
    e2e_info "preflight checking container runtime: ${E2E_CONTAINER_ENGINE}"
    e2e_validate_container_engine || return 1
    e2e_require_command "${E2E_CONTAINER_ENGINE}" || return 1
    e2e_compose_cmd version >/dev/null || return 1
  fi
}
