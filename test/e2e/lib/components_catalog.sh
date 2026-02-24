# Component catalog/discovery helpers split from components.sh.

e2e_discover_components() {
  E2E_COMPONENT_KEYS=()
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_CONNECTIONS=()
  E2E_COMPONENT_DEFAULT_CONNECTION=()
  E2E_COMPONENT_REQUIRES_DOCKER=()
  E2E_COMPONENT_CONTRACT_VERSION=()
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
        local contract_version=${COMPONENT_CONTRACT_VERSION:-}
        local runtime_kind=${COMPONENT_RUNTIME_KIND:-}
        local supported_security_features=${SUPPORTED_SECURITY_FEATURES:-}
        local required_security_features=${REQUIRED_SECURITY_FEATURES:-}
        local has_requires_docker=0
        local has_contract_version=0
        local has_runtime_kind=0
        local has_depends_on=0
        local has_supported_security_features=0
        local has_required_security_features=0

        if [[ -n "${REQUIRES_DOCKER+x}" ]]; then
          has_requires_docker=1
        fi
        if [[ -n "${COMPONENT_CONTRACT_VERSION+x}" ]]; then
          has_contract_version=1
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

        printf '%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s\n' \
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
          "${contract_version}" \
          "${sep}" \
          "${runtime_kind}" \
          "${sep}" \
          "${COMPONENT_DEPENDS_ON:-}" \
          "${sep}" \
          "${has_requires_docker}" \
          "${sep}" \
          "${has_contract_version}" \
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
    local contract_version
    local runtime_kind
    local depends_on
    local has_requires_docker
    local has_contract_version
    local has_runtime_kind
    local has_depends_on
    local supported_security_features
    local required_security_features
    local has_supported_security_features
    local has_required_security_features
    local description

    IFS=$'\x1f' read -r component_type component_name supported_connections default_connection requires_docker contract_version runtime_kind depends_on has_requires_docker has_contract_version has_runtime_kind has_depends_on supported_security_features required_security_features has_supported_security_features has_required_security_features description <<<"${metadata}"

    local component_key
    local component_path

    component_key=$(e2e_component_key "${component_type}" "${component_name}")
    component_path=$(dirname "${component_file}")

    COMPONENT_TYPE="${component_type}" \
    COMPONENT_NAME="${component_name}" \
    SUPPORTED_CONNECTIONS="${supported_connections}" \
    DEFAULT_CONNECTION="${default_connection}" \
    REQUIRES_DOCKER="${requires_docker}" \
    COMPONENT_CONTRACT_VERSION="${contract_version}" \
    COMPONENT_RUNTIME_KIND="${runtime_kind}" \
    COMPONENT_DEPENDS_ON="${depends_on}" \
      e2e_component_validate_contract \
        "${component_key}" \
        "${component_path}" \
        "${requires_docker}" \
        "${contract_version}" \
        "${runtime_kind}" \
        "${has_requires_docker}" \
        "${has_contract_version}" \
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
    E2E_COMPONENT_CONTRACT_VERSION["${component_key}"]="${contract_version}"
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
