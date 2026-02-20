#!/usr/bin/env bash

declare -Ag E2E_COMPONENT_PATH=()
declare -Ag E2E_COMPONENT_CONNECTIONS=()
declare -Ag E2E_COMPONENT_DEFAULT_CONNECTION=()
declare -Ag E2E_COMPONENT_REQUIRES_DOCKER=()
declare -Ag E2E_COMPONENT_RUNTIME_KIND=()
declare -Ag E2E_COMPONENT_DEPENDS_ON=()
declare -Ag E2E_COMPONENT_DESCRIPTION=()
declare -Ag E2E_COMPONENT_PROJECT=()
declare -Ag E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES=()
declare -Ag E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES=()
declare -Ag E2E_CAPABILITY_SET=()

declare -ag E2E_COMPONENT_KEYS=()
declare -ag E2E_SELECTED_COMPONENT_KEYS=()
declare -ag E2E_STARTED_COMPONENT_KEYS=()

e2e_resource_server_feature_enabled() {
  local feature=$1

  case "${feature}" in
    basic-auth)
      [[ "${E2E_RESOURCE_SERVER_BASIC_AUTH}" == 'true' ]]
      ;;
    oauth2)
      [[ "${E2E_RESOURCE_SERVER_OAUTH2}" == 'true' ]]
      ;;
    mtls)
      [[ "${E2E_RESOURCE_SERVER_MTLS}" == 'true' ]]
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_resource_server_feature_spec_supports() {
  local feature_spec=$1
  local feature=$2
  local supported=" ${feature_spec} "
  [[ "${supported}" == *" ${feature} "* ]]
}

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

e2e_component_context_fragment_path() {
  local component_key=$1

  if [[ -z "${E2E_CONTEXT_DIR:-}" ]]; then
    printf '\n'
    return 0
  fi

  printf '%s/%s-%s.yaml\n' \
    "${E2E_CONTEXT_DIR}" \
    "$(e2e_component_type "${component_key}")" \
    "$(e2e_component_name "${component_key}")"
}

e2e_component_validate_connections() {
  local component_key=$1
  local supported_connections=$2
  local default_connection=$3
  local connection
  local default_supported=0

  [[ -n "${supported_connections}" ]] || {
    e2e_die "component ${component_key} must declare SUPPORTED_CONNECTIONS"
    return 1
  }

  for connection in ${supported_connections}; do
    case "${connection}" in
      local|remote) ;;
      *)
        e2e_die "component ${component_key} has invalid connection value: ${connection}"
        return 1
        ;;
    esac

    if [[ "${connection}" == "${default_connection}" ]]; then
      default_supported=1
    fi
  done

  if ((default_supported == 0)); then
    e2e_die "component ${component_key} default connection ${default_connection} is not in SUPPORTED_CONNECTIONS"
    return 1
  fi

  return 0
}

e2e_component_validate_dependency_spec() {
  local component_key=$1
  local dependency_spec=$2
  local token
  local dependency_type
  local dependency_name

  for token in ${dependency_spec}; do
    [[ -n "${token}" ]] || continue

    if [[ "${token}" != *:* ]]; then
      e2e_die "component ${component_key} has invalid dependency token: ${token} (expected type:name or type:*)"
      return 1
    fi

    dependency_type=${token%%:*}
    dependency_name=${token#*:}

    [[ -n "${dependency_type}" && -n "${dependency_name}" ]] || {
      e2e_die "component ${component_key} has invalid dependency token: ${token}"
      return 1
    }
  done

  return 0
}

e2e_component_validate_security_feature_spec() {
  local component_key=$1
  local field_name=$2
  local feature_spec=$3
  local feature

  for feature in ${feature_spec}; do
    case "${feature}" in
      basic-auth|oauth2|mtls) ;;
      *)
        e2e_die "component ${component_key} has invalid ${field_name} value: ${feature} (allowed: basic-auth, oauth2, mtls)"
        return 1
        ;;
    esac
  done

  return 0
}

e2e_component_validate_resource_server_security_contract() {
  local component_key=$1
  local supported_features=$2
  local required_features=$3
  local has_supported_features=$4
  local has_required_features=$5

  if [[ "${has_supported_features}" != '1' ]]; then
    e2e_die "resource-server component ${component_key} must declare SUPPORTED_SECURITY_FEATURES in component.env"
    return 1
  fi

  e2e_component_validate_security_feature_spec "${component_key}" 'SUPPORTED_SECURITY_FEATURES' "${supported_features}" || return 1

  if [[ "${has_required_features}" == '1' ]]; then
    e2e_component_validate_security_feature_spec "${component_key}" 'REQUIRED_SECURITY_FEATURES' "${required_features}" || return 1

    local feature
    for feature in ${required_features}; do
      if ! e2e_resource_server_feature_spec_supports "${supported_features}" "${feature}"; then
        e2e_die "component ${component_key} has REQUIRED_SECURITY_FEATURES entry not listed in SUPPORTED_SECURITY_FEATURES: ${feature}"
        return 1
      fi
    done
  fi

  return 0
}

e2e_component_validate_contract() {
  local component_key=$1
  local component_path=$2
  local requires_docker=$3
  local runtime_kind=$4
  local has_requires_docker=$5
  local has_runtime_kind=$6
  local has_depends_on=$7
  local supported_security_features=$8
  local required_security_features=$9
  local has_supported_security_features=${10}
  local has_required_security_features=${11}

  [[ -n "${COMPONENT_TYPE:-}" ]] || {
    e2e_die "component metadata missing COMPONENT_TYPE in ${component_path}/component.env"
    return 1
  }
  [[ -n "${COMPONENT_NAME:-}" ]] || {
    e2e_die "component metadata missing COMPONENT_NAME in ${component_path}/component.env"
    return 1
  }

  if [[ "${has_requires_docker}" != '1' ]]; then
    e2e_die "component ${component_key} must declare REQUIRES_DOCKER in ${component_path}/component.env"
    return 1
  fi

  if [[ "${has_runtime_kind}" != '1' ]]; then
    e2e_die "component ${component_key} must declare COMPONENT_RUNTIME_KIND in ${component_path}/component.env"
    return 1
  fi

  if [[ "${has_depends_on}" != '1' ]]; then
    e2e_die "component ${component_key} must declare COMPONENT_DEPENDS_ON in ${component_path}/component.env"
    return 1
  fi

  if [[ "${requires_docker}" != 'true' && "${requires_docker}" != 'false' ]]; then
    e2e_die "component ${component_key} has invalid REQUIRES_DOCKER=${requires_docker} (allowed: true, false)"
    return 1
  fi

  case "${runtime_kind}" in
    native|compose) ;;
    *)
      e2e_die "component ${component_key} has invalid COMPONENT_RUNTIME_KIND=${runtime_kind} (allowed: native, compose)"
      return 1
      ;;
  esac

  if [[ "${runtime_kind}" == 'compose' && "${requires_docker}" != 'true' ]]; then
    e2e_die "component ${component_key} uses COMPONENT_RUNTIME_KIND=compose but REQUIRES_DOCKER is not true"
    return 1
  fi

  if [[ "${runtime_kind}" == 'native' && "${requires_docker}" == 'true' ]]; then
    e2e_die "component ${component_key} uses COMPONENT_RUNTIME_KIND=native but REQUIRES_DOCKER=true"
    return 1
  fi

  e2e_component_validate_connections "${component_key}" "${SUPPORTED_CONNECTIONS:-}" "${DEFAULT_CONNECTION:-}" || return 1
  e2e_component_validate_dependency_spec "${component_key}" "${COMPONENT_DEPENDS_ON:-}" || return 1

  if [[ "${COMPONENT_TYPE}" == 'resource-server' ]]; then
    e2e_component_validate_resource_server_security_contract \
      "${component_key}" \
      "${supported_security_features}" \
      "${required_security_features}" \
      "${has_supported_security_features}" \
      "${has_required_security_features}" || return 1
  fi

  local required_hook
  for required_hook in init configure-auth context; do
    if [[ ! -f "${component_path}/scripts/${required_hook}.sh" ]]; then
      e2e_die "component ${component_key} missing required hook script: scripts/${required_hook}.sh"
      return 1
    fi
  done

  if [[ "${runtime_kind}" == 'compose' ]]; then
    if [[ ! -f "${component_path}/compose.yaml" ]]; then
      e2e_die "component ${component_key} missing compose.yaml for compose runtime"
      return 1
    fi
    if [[ ! -f "${component_path}/scripts/health.sh" ]]; then
      e2e_die "component ${component_key} missing required health hook for compose runtime"
      return 1
    fi
  fi

  return 0
}

e2e_validate_component_dependency_catalog() {
  local component_key
  local token
  local dependency_type
  local dependency_name
  local dependency_key
  local candidate
  local found
  local -A discovered=()

  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    discovered["${component_key}"]=1
  done

  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    for token in ${E2E_COMPONENT_DEPENDS_ON[${component_key}]:-}; do
      [[ -n "${token}" ]] || continue

      dependency_type=${token%%:*}
      dependency_name=${token#*:}

      if [[ "${dependency_name}" == '*' ]]; then
        found=0
        for candidate in "${E2E_COMPONENT_KEYS[@]}"; do
          if [[ "$(e2e_component_type "${candidate}")" == "${dependency_type}" ]]; then
            found=1
            break
          fi
        done

        if ((found == 0)); then
          e2e_die "component ${component_key} dependency selector ${token} did not match any discovered component type"
          return 1
        fi
        continue
      fi

      dependency_key=$(e2e_component_key "${dependency_type}" "${dependency_name}")
      if [[ "${discovered[${dependency_key}]:-0}" != '1' ]]; then
        e2e_die "component ${component_key} dependency ${dependency_key} is not a discovered component"
        return 1
      fi
    done
  done

  return 0
}

e2e_discover_components() {
  E2E_COMPONENT_KEYS=()
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_CONNECTIONS=()
  E2E_COMPONENT_DEFAULT_CONNECTION=()
  E2E_COMPONENT_REQUIRES_DOCKER=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_DESCRIPTION=()
  E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES=()
  E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES=()

  local component_file
  while IFS= read -r component_file; do
    [[ -n "${component_file}" ]] || continue

    local metadata
    metadata=$(
      (
        local sep=$'\x1f'

        # shellcheck disable=SC1090
        source "${component_file}"

        local requires_docker=${REQUIRES_DOCKER:-}
        local runtime_kind=${COMPONENT_RUNTIME_KIND:-}
        local supported_security_features=${SUPPORTED_SECURITY_FEATURES:-}
        local required_security_features=${REQUIRED_SECURITY_FEATURES:-}
        local has_requires_docker=0
        local has_runtime_kind=0
        local has_depends_on=0
        local has_supported_security_features=0
        local has_required_security_features=0

        if [[ -n "${REQUIRES_DOCKER+x}" ]]; then
          has_requires_docker=1
        fi
        if [[ -n "${COMPONENT_RUNTIME_KIND+x}" ]]; then
          has_runtime_kind=1
        fi
        if [[ -n "${COMPONENT_DEPENDS_ON+x}" ]]; then
          has_depends_on=1
        fi
        if [[ -n "${SUPPORTED_SECURITY_FEATURES+x}" ]]; then
          has_supported_security_features=1
        fi
        if [[ -n "${REQUIRED_SECURITY_FEATURES+x}" ]]; then
          has_required_security_features=1
        fi

        printf '%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s\n' \
          "${COMPONENT_TYPE}" \
          "${sep}" \
          "${COMPONENT_NAME}" \
          "${sep}" \
          "${SUPPORTED_CONNECTIONS}" \
          "${sep}" \
          "${DEFAULT_CONNECTION}" \
          "${sep}" \
          "${requires_docker}" \
          "${sep}" \
          "${runtime_kind}" \
          "${sep}" \
          "${COMPONENT_DEPENDS_ON:-}" \
          "${sep}" \
          "${has_requires_docker}" \
          "${sep}" \
          "${has_runtime_kind}" \
          "${sep}" \
          "${has_depends_on}" \
          "${sep}" \
          "${supported_security_features}" \
          "${sep}" \
          "${required_security_features}" \
          "${sep}" \
          "${has_supported_security_features}" \
          "${sep}" \
          "${has_required_security_features}" \
          "${sep}" \
          "${DESCRIPTION:-}"
      )
    )

    local component_type
    local component_name
    local supported_connections
    local default_connection
    local requires_docker
    local runtime_kind
    local depends_on
    local has_requires_docker
    local has_runtime_kind
    local has_depends_on
    local supported_security_features
    local required_security_features
    local has_supported_security_features
    local has_required_security_features
    local description

    IFS=$'\x1f' read -r component_type component_name supported_connections default_connection requires_docker runtime_kind depends_on has_requires_docker has_runtime_kind has_depends_on supported_security_features required_security_features has_supported_security_features has_required_security_features description <<<"${metadata}"

    local component_key
    local component_path

    component_key=$(e2e_component_key "${component_type}" "${component_name}")
    component_path=$(dirname "${component_file}")

    COMPONENT_TYPE="${component_type}" \
    COMPONENT_NAME="${component_name}" \
    SUPPORTED_CONNECTIONS="${supported_connections}" \
    DEFAULT_CONNECTION="${default_connection}" \
    REQUIRES_DOCKER="${requires_docker}" \
    COMPONENT_RUNTIME_KIND="${runtime_kind}" \
    COMPONENT_DEPENDS_ON="${depends_on}" \
      e2e_component_validate_contract \
        "${component_key}" \
        "${component_path}" \
        "${requires_docker}" \
        "${runtime_kind}" \
        "${has_requires_docker}" \
        "${has_runtime_kind}" \
        "${has_depends_on}" \
        "${supported_security_features}" \
        "${required_security_features}" \
        "${has_supported_security_features}" \
        "${has_required_security_features}" || return 1

    E2E_COMPONENT_PATH["${component_key}"]="${component_path}"
    E2E_COMPONENT_CONNECTIONS["${component_key}"]="${supported_connections}"
    E2E_COMPONENT_DEFAULT_CONNECTION["${component_key}"]="${default_connection}"
    E2E_COMPONENT_REQUIRES_DOCKER["${component_key}"]="${requires_docker}"
    E2E_COMPONENT_RUNTIME_KIND["${component_key}"]="${runtime_kind}"
    E2E_COMPONENT_DEPENDS_ON["${component_key}"]="${depends_on}"
    E2E_COMPONENT_DESCRIPTION["${component_key}"]="${description}"
    E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES["${component_key}"]="${supported_security_features}"
    E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES["${component_key}"]="${required_security_features}"
    E2E_COMPONENT_KEYS+=("${component_key}")
  done < <(find "${E2E_DIR}/components" -type f -name 'component.env' | sort)

  e2e_validate_component_dependency_catalog || return 1
}

e2e_list_components() {
  printf 'Discovered e2e components\n'
  printf '%s\n' '------------------------'
  printf '%-18s %-14s %-18s %-10s %-20s %-30s %s\n' 'TYPE' 'NAME' 'CONNECTIONS' 'RUNTIME' 'DEPENDS-ON' 'SECURITY' 'DESCRIPTION'

  local component_key
  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    local component_type
    local component_name
    local security='-'

    component_type=$(e2e_component_type "${component_key}")
    component_name=$(e2e_component_name "${component_key}")

    if [[ "${component_type}" == 'resource-server' ]]; then
      local supported_features=${E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES[${component_key}]:-}
      local required_features=${E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES[${component_key}]:-}

      security=${supported_features:-none}
      if [[ -n "${required_features}" ]]; then
        security="${security} (required: ${required_features})"
      fi
    fi

    printf '%-18s %-14s %-18s %-10s %-20s %-30s %s\n' \
      "${component_type}" \
      "${component_name}" \
      "${E2E_COMPONENT_CONNECTIONS[${component_key}]}" \
      "${E2E_COMPONENT_RUNTIME_KIND[${component_key}]}" \
      "${E2E_COMPONENT_DEPENDS_ON[${component_key}]:-none}" \
      "${security}" \
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

e2e_validate_resource_server_security_selection() {
  if [[ "${E2E_RESOURCE_SERVER}" == 'none' ]]; then
    if e2e_is_explicit 'resource-server-basic-auth' && [[ "${E2E_RESOURCE_SERVER_BASIC_AUTH}" == 'true' ]]; then
      e2e_die '--resource-server-basic-auth requires a selected resource-server component'
      return 1
    fi
    if e2e_is_explicit 'resource-server-oauth2' && [[ "${E2E_RESOURCE_SERVER_OAUTH2}" == 'true' ]]; then
      e2e_die '--resource-server-oauth2 requires a selected resource-server component'
      return 1
    fi
    if e2e_is_explicit 'resource-server-mtls' && [[ "${E2E_RESOURCE_SERVER_MTLS}" == 'true' ]]; then
      e2e_die '--resource-server-mtls requires a selected resource-server component'
      return 1
    fi
    return 0
  fi

  if [[ "${E2E_RESOURCE_SERVER_BASIC_AUTH}" == 'true' && "${E2E_RESOURCE_SERVER_OAUTH2}" == 'true' ]]; then
    e2e_die '--resource-server-basic-auth and --resource-server-oauth2 cannot both be true (resource-server auth is one-of)'
    return 1
  fi

  local component_key
  component_key=$(e2e_component_key 'resource-server' "${E2E_RESOURCE_SERVER}")

  local supported_features=${E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES[${component_key}]:-}
  local required_features=${E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES[${component_key}]:-}
  local feature
  local selected

  for feature in basic-auth oauth2 mtls; do
    if e2e_resource_server_feature_enabled "${feature}"; then
      if ! e2e_resource_server_feature_spec_supports "${supported_features}" "${feature}"; then
        e2e_die "resource-server ${E2E_RESOURCE_SERVER} does not support selected security feature: ${feature}"
        return 1
      fi
    fi
  done

  for feature in ${required_features}; do
    selected='false'
    if e2e_resource_server_feature_enabled "${feature}"; then
      selected='true'
    fi
    if [[ "${selected}" != 'true' ]]; then
      e2e_die "resource-server ${E2E_RESOURCE_SERVER} requires security feature ${feature}=true"
      return 1
    fi
  done

  return 0
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

  e2e_validate_resource_server_security_selection || return 1

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

  return 0
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

e2e_validate_selected_component_dependencies() {
  local -A selected_set=()
  local component_key

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    selected_set["${component_key}"]=1
  done

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    e2e_component_dependency_keys "${component_key}" selected_set >/dev/null || return 1
  done

  return 0
}

e2e_build_capabilities() {
  E2E_CAPABILITY_SET=()

  E2E_CAPABILITY_SET["profile=${E2E_PROFILE}"]=1
  E2E_CAPABILITY_SET["repo-type=${E2E_REPO_TYPE}"]=1
  E2E_CAPABILITY_SET["resource-server=${E2E_RESOURCE_SERVER}"]=1
  E2E_CAPABILITY_SET["resource-server-connection=${E2E_RESOURCE_SERVER_CONNECTION}"]=1
  E2E_CAPABILITY_SET["resource-server-basic-auth=${E2E_RESOURCE_SERVER_BASIC_AUTH}"]=1
  E2E_CAPABILITY_SET["resource-server-oauth2=${E2E_RESOURCE_SERVER_OAUTH2}"]=1
  E2E_CAPABILITY_SET["resource-server-mtls=${E2E_RESOURCE_SERVER_MTLS}"]=1
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

  if [[ "${E2E_RESOURCE_SERVER}" != 'none' && "${E2E_RESOURCE_SERVER_BASIC_AUTH}" == 'true' ]]; then
    E2E_CAPABILITY_SET['has-resource-server-basic-auth']=1
  fi
  if [[ "${E2E_RESOURCE_SERVER}" != 'none' && "${E2E_RESOURCE_SERVER_OAUTH2}" == 'true' ]]; then
    E2E_CAPABILITY_SET['has-resource-server-oauth2']=1
  fi
  if [[ "${E2E_RESOURCE_SERVER}" != 'none' && "${E2E_RESOURCE_SERVER_MTLS}" == 'true' ]]; then
    E2E_CAPABILITY_SET['has-resource-server-mtls']=1
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
  export E2E_COMPONENT_RUNTIME_KIND="${E2E_COMPONENT_RUNTIME_KIND[${component_key}]:-native}"
  export E2E_COMPONENT_DEPENDS_ON="${E2E_COMPONENT_DEPENDS_ON[${component_key}]:-}"
  export E2E_COMPONENT_STATE_FILE="$(e2e_component_state_file "${component_key}")"
  export E2E_COMPONENT_PROJECT_NAME="${E2E_COMPONENT_PROJECT[${component_key}]:-}"
  export E2E_COMPONENT_CONTEXT_FRAGMENT="$(e2e_component_context_fragment_path "${component_key}")"
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

e2e_components_run_hook_batch_parallel() {
  local hook_name=$1
  shift
  local -a batch=("$@")

  if ((${#batch[@]} == 0)); then
    return 0
  fi

  local tmp_dir
  tmp_dir=$(mktemp -d /tmp/declarest-e2e-hook-${hook_name}.XXXXXX)

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

  rm -rf "${tmp_dir}" || true

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
    if [[ "${connection}" == 'local' ]] && e2e_component_runtime_is_compose "${component_key}"; then
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
