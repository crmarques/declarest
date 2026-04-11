# Component validation/selection/capability helpers split from components.sh.

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
      none|basic-auth|oauth2|custom-header|mtls) ;;
      *)
        e2e_die "component ${component_key} has invalid ${field_name} value: ${feature} (allowed: none, basic-auth, oauth2, custom-header, mtls)"
        return 1
        ;;
    esac
  done

  return 0
}

e2e_component_validate_service_port() {
  local component_key=$1
  local service_port=$2

  [[ -n "${service_port}" ]] || return 0

  if ! [[ "${service_port}" =~ ^[0-9]+$ ]] || ((service_port <= 0 || service_port > 65535)); then
    e2e_die "component ${component_key} has invalid COMPONENT_SERVICE_PORT=${service_port} (allowed: integer 1-65535)"
    return 1
  fi

  return 0
}

e2e_component_validate_default_selection_spec() {
  local component_key=$1
  local component_type=$2
  local selection_spec=$3
  local supported_connections=$4
  local repository_webhook_provider=$5
  local selection

  for selection in ${selection_spec}; do
    case "${selection}" in
      base|operator) ;;
      *)
        e2e_die "component ${component_key} has invalid DEFAULT_SELECTIONS value: ${selection} (allowed: base, operator)"
        return 1
        ;;
    esac

    if [[ "${selection}" == 'operator' && "${component_type}" == 'git-provider' ]]; then
      if [[ -z "${repository_webhook_provider}" ]]; then
        e2e_die "git-provider component ${component_key} must declare REPOSITORY_WEBHOOK_PROVIDER when DEFAULT_SELECTIONS includes operator"
        return 1
      fi
      if [[ " ${supported_connections} " != *' local '* ]]; then
        e2e_die "git-provider component ${component_key} must support local connection when DEFAULT_SELECTIONS includes operator"
        return 1
      fi
    fi
  done

  return 0
}

e2e_component_validate_type_specific_contract() {
  local component_key=$1
  local component_type=$2
  local metadata_bundle_ref=$3
  local operator_example_resource_path=$4
  local operator_example_resource_payload=$5
  local repository_webhook_provider=$6
  local repo_provider_login_path=$7

  case "${component_type}" in
    managed-server)
      if [[ -n "${operator_example_resource_path}" && -z "${operator_example_resource_payload}" ]]; then
        e2e_die "managed-server component ${component_key} must declare OPERATOR_EXAMPLE_RESOURCE_PAYLOAD when OPERATOR_EXAMPLE_RESOURCE_PATH is set"
        return 1
      fi
      if [[ -z "${operator_example_resource_path}" && -n "${operator_example_resource_payload}" ]]; then
        e2e_die "managed-server component ${component_key} must declare OPERATOR_EXAMPLE_RESOURCE_PATH when OPERATOR_EXAMPLE_RESOURCE_PAYLOAD is set"
        return 1
      fi
      if [[ -n "${repository_webhook_provider}" ]]; then
        e2e_die "managed-server component ${component_key} must not declare REPOSITORY_WEBHOOK_PROVIDER"
        return 1
      fi
      if [[ -n "${repo_provider_login_path}" ]]; then
        e2e_die "managed-server component ${component_key} must not declare REPO_PROVIDER_LOGIN_PATH"
        return 1
      fi
      ;;
    git-provider)
      if [[ -n "${metadata_bundle_ref}" ]]; then
        e2e_die "git-provider component ${component_key} must not declare METADATA_BUNDLE_REF"
        return 1
      fi
      if [[ -n "${operator_example_resource_path}" || -n "${operator_example_resource_payload}" ]]; then
        e2e_die "git-provider component ${component_key} must not declare OPERATOR_EXAMPLE_RESOURCE_PATH or OPERATOR_EXAMPLE_RESOURCE_PAYLOAD"
        return 1
      fi
      ;;
    *)
      if [[ -n "${metadata_bundle_ref}" ]]; then
        e2e_die "component ${component_key} must not declare METADATA_BUNDLE_REF outside managed-server components"
        return 1
      fi
      if [[ -n "${operator_example_resource_path}" || -n "${operator_example_resource_payload}" ]]; then
        e2e_die "component ${component_key} must not declare OPERATOR_EXAMPLE_RESOURCE_PATH or OPERATOR_EXAMPLE_RESOURCE_PAYLOAD outside managed-server components"
        return 1
      fi
      if [[ -n "${repository_webhook_provider}" ]]; then
        e2e_die "component ${component_key} must not declare REPOSITORY_WEBHOOK_PROVIDER outside git-provider components"
        return 1
      fi
      if [[ -n "${repo_provider_login_path}" ]]; then
        e2e_die "component ${component_key} must not declare REPO_PROVIDER_LOGIN_PATH outside git-provider components"
        return 1
      fi
      ;;
  esac

  return 0
}

e2e_managed_server_auth_capability_count() {
  local feature_spec=$1
  local feature
  local count=0

  for feature in ${feature_spec}; do
    if e2e_managed_server_security_feature_is_auth "${feature}"; then
      ((count += 1))
    fi
  done

  printf '%s\n' "${count}"
}

e2e_managed_server_first_required_auth_type() {
  local required_features=$1
  local feature

  for feature in ${required_features}; do
    if e2e_managed_server_security_feature_is_auth "${feature}"; then
      e2e_managed_server_auth_type_for_feature "${feature}" || return 1
      return 0
    fi
  done

  return 1
}

e2e_managed_server_default_auth_type() {
  local component_name=$1
  local supported_features=$2
  local required_features=$3
  local auth_type
  local feature

  if auth_type=$(e2e_managed_server_first_required_auth_type "${required_features}" 2>/dev/null); then
    printf '%s\n' "${auth_type}"
    return 0
  fi

  for auth_type in oauth2 custom-header basic none; do
    feature=$(e2e_managed_server_auth_feature_for_type "${auth_type}") || return 1
    if e2e_managed_server_feature_spec_supports "${supported_features}" "${feature}"; then
      printf '%s\n' "${auth_type}"
      return 0
    fi
  done

  e2e_die "managed-server ${component_name} does not declare any auth-type capability in SUPPORTED_SECURITY_FEATURES (expected one of none, basic-auth, oauth2, custom-header)"
  return 1
}

e2e_component_validate_managed_server_security_contract() {
  local component_key=$1
  local supported_features=$2
  local required_features=$3
  local has_supported_features=$4
  local has_required_features=$5

  if [[ "${has_supported_features}" != '1' ]]; then
    e2e_die "managed-server component ${component_key} must declare SUPPORTED_SECURITY_FEATURES in component.env"
    return 1
  fi

  e2e_component_validate_security_feature_spec "${component_key}" 'SUPPORTED_SECURITY_FEATURES' "${supported_features}" || return 1

  local supported_auth_count
  supported_auth_count=$(e2e_managed_server_auth_capability_count "${supported_features}") || return 1
  if ((supported_auth_count == 0)); then
    e2e_die "managed-server component ${component_key} must declare at least one auth-type capability in SUPPORTED_SECURITY_FEATURES (none, basic-auth, oauth2, custom-header)"
    return 1
  fi

  if [[ "${has_required_features}" == '1' ]]; then
    e2e_component_validate_security_feature_spec "${component_key}" 'REQUIRED_SECURITY_FEATURES' "${required_features}" || return 1

    local feature
    for feature in ${required_features}; do
      if ! e2e_managed_server_feature_spec_supports "${supported_features}" "${feature}"; then
        e2e_die "component ${component_key} has REQUIRED_SECURITY_FEATURES entry not listed in SUPPORTED_SECURITY_FEATURES: ${feature}"
        return 1
      fi
    done

    local required_auth_count
    required_auth_count=$(e2e_managed_server_auth_capability_count "${required_features}") || return 1
    if ((required_auth_count > 1)); then
      e2e_die "component ${component_key} has multiple auth-type entries in REQUIRED_SECURITY_FEATURES (managed-server auth is one-of)"
      return 1
    fi
  fi

  return 0
}

e2e_component_validate_contract() {
  local component_key=$1
  local component_path=$2
  local requires_docker=$3
  local contract_version=$4
  local runtime_kind=$5
  local default_selections=$6
  local has_requires_docker=$7
  local has_contract_version=$8
  local has_runtime_kind=$9
  local has_depends_on=${10}
  local supported_security_features=${11}
  local required_security_features=${12}
  local has_supported_security_features=${13}
  local has_required_security_features=${14}
  local component_service_port=${15}
  local metadata_bundle_ref=${16}
  local operator_example_resource_path=${17}
  local operator_example_resource_payload=${18}
  local repository_webhook_provider=${19}
  local repo_provider_login_path=${20}

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

  if [[ "${has_contract_version}" != '1' ]]; then
    e2e_die "component ${component_key} must declare COMPONENT_CONTRACT_VERSION in ${component_path}/component.env"
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

  if [[ "${contract_version}" != '1' ]]; then
    e2e_die "component ${component_key} has unsupported COMPONENT_CONTRACT_VERSION=${contract_version} (supported: 1)"
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
  e2e_component_validate_default_selection_spec \
    "${component_key}" \
    "${COMPONENT_TYPE}" \
    "${default_selections}" \
    "${SUPPORTED_CONNECTIONS:-}" \
    "${repository_webhook_provider}" || return 1
  e2e_component_validate_service_port "${component_key}" "${component_service_port}" || return 1
  e2e_component_validate_type_specific_contract \
    "${component_key}" \
    "${COMPONENT_TYPE}" \
    "${metadata_bundle_ref}" \
    "${operator_example_resource_path}" \
    "${operator_example_resource_payload}" \
    "${repository_webhook_provider}" \
    "${repo_provider_login_path}" || return 1

  if [[ "${COMPONENT_TYPE}" == 'managed-server' ]]; then
    e2e_component_validate_managed_server_security_contract \
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
    local compose_file
    compose_file="${component_path}/compose/compose.yaml"
    if [[ ! -f "${compose_file}" ]]; then
      e2e_die "component ${component_key} missing compose/compose.yaml for containerized runtime"
      return 1
    fi
    local k8s_dir
    k8s_dir="${component_path}/k8s"
    if [[ ! -d "${k8s_dir}" ]]; then
      e2e_die "component ${component_key} missing k8s artifact directory for containerized runtime"
      return 1
    fi
    if ! find "${k8s_dir}" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.yml' \) | grep -q .; then
      e2e_die "component ${component_key} has no k8s manifests under ${k8s_dir}"
      return 1
    fi
    if [[ ! -f "${component_path}/scripts/health.sh" ]]; then
      e2e_die "component ${component_key} missing required health hook for compose runtime"
      return 1
    fi
  fi

  return 0
}

e2e_validate_component_default_selection_catalog() {
  local component_key
  local component_type
  local selection
  local -A seen=()

  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    component_type=$(e2e_component_type "${component_key}")
    for selection in ${E2E_COMPONENT_DEFAULT_SELECTIONS[${component_key}]:-}; do
      local selector_key="${component_type}:${selection}"
      if [[ -n "${seen[${selector_key}]:-}" ]]; then
        e2e_die "components ${seen[${selector_key}]} and ${component_key} both declare DEFAULT_SELECTIONS=${selection} for type ${component_type}"
        return 1
      fi
      seen["${selector_key}"]="${component_key}"
    done
  done

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

e2e_metadata_fixture_has_identity_fields() {
  local metadata_file=$1

  case "${metadata_file}" in
    *.json)
      if ! command -v jq >/dev/null 2>&1; then
        e2e_die 'jq is required to validate managed-server fixture metadata'
        return 1
      fi

      jq -e '((.resource.id // "") | (type == "string" and length > 0) and (startswith("/") | not)) and ((.resource.alias // "") | (type == "string" and length > 0) and (startswith("/") | not))' \
        "${metadata_file}" >/dev/null 2>&1
      return $?
      ;;
    *.yaml)
      grep -Eq '^[[:space:]]*resource:[[:space:]]*$' "${metadata_file}" \
        && grep -Eq '^[[:space:]]*id:[[:space:]]*[^[:space:]#]' "${metadata_file}" \
        && grep -Eq '^[[:space:]]*alias:[[:space:]]*[^[:space:]#]' "${metadata_file}"
      return $?
      ;;
    *)
      e2e_die "unsupported managed-server metadata fixture file: ${metadata_file}"
      return 1
      ;;
  esac
}


e2e_validate_managed_server_fixture_tree() {
  local component_name=$1
  local component_key
  component_key=$(e2e_component_key 'managed-server' "${component_name}")

  local component_dir="${E2E_COMPONENT_PATH[${component_key}]:-}"
  if [[ -z "${component_dir}" ]]; then
    e2e_die "managed-server component path not found: ${component_name}"
    return 1
  fi

  local template_dir="${component_dir}/repo-template"
  if [[ ! -d "${template_dir}" ]]; then
    e2e_die "managed-server ${component_name} missing repo-template fixture tree: ${template_dir}"
    return 1
  fi

  local metadata_root="${component_dir}/metadata"
  local metadata_dir
  if [[ -d "${metadata_root}" ]]; then
    metadata_dir="${metadata_root}"
  else
    metadata_dir="${template_dir}"
  fi

  local -a metadata_files=()
  local metadata_file
  while IFS= read -r metadata_file; do
    [[ -n "${metadata_file}" ]] || continue
    metadata_files+=("${metadata_file}")
  done < <(e2e_find_collection_metadata_files "${metadata_dir}")

  local -a payload_files=()
  local payload_file
  while IFS= read -r payload_file; do
    [[ -n "${payload_file}" ]] || continue
    payload_files+=("${payload_file}")
  done < <(find "${template_dir}" -type f -name 'resource.*' | sort)

  if ((${#payload_files[@]} == 0)); then
    e2e_die "managed-server ${component_name} repo-template has no resource payload files under ${template_dir}"
    return 1
  fi

  for payload_file in "${payload_files[@]}"; do
    local payload_rel
    payload_rel=${payload_file#${template_dir}/}
    if [[ "$(basename -- "${payload_rel}")" != resource.* || "$(basename -- "${payload_rel}")" == 'resource.' ]]; then
      e2e_die "managed-server ${component_name} has invalid resource payload fixture path: ${payload_rel} (expected */resource.<ext>)"
      return 1
    fi
  done

  for metadata_file in "${metadata_files[@]}"; do
    local rel
    rel=${metadata_file#${metadata_dir}/}
    if [[ "${rel}" != *_/metadata.json && "${rel}" != *_/metadata.yaml ]]; then
      e2e_die "managed-server ${component_name} has invalid metadata fixture path: ${rel} (expected */_/metadata.json or */_/metadata.yaml)"
      return 1
    fi

    if ! e2e_metadata_fixture_has_identity_fields "${metadata_file}"; then
      e2e_die "managed-server ${component_name} metadata fixture missing resource.id or resource.alias: ${rel}"
      return 1
    fi
  done

  return 0
}

e2e_component_validate_script_syntax() {
  local component_key=$1
  local component_path=$2
  local runtime_kind=${3:-${E2E_COMPONENT_RUNTIME_KIND[${component_key}]:-native}}
  local script_path
  local hook

  for hook in init configure-auth context; do
    script_path="${component_path}/scripts/${hook}.sh"
    [[ -f "${script_path}" ]] || continue
    bash -n "${script_path}" || {
      e2e_die "component ${component_key} has invalid bash syntax in scripts/${hook}.sh"
      return 1
    }
  done

  if [[ "${runtime_kind}" == 'compose' ]]; then
    script_path="${component_path}/scripts/health.sh"
    [[ -f "${script_path}" ]] || {
      e2e_die "component ${component_key} missing required health hook for compose runtime"
      return 1
    }
    bash -n "${script_path}" || {
      e2e_die "component ${component_key} has invalid bash syntax in scripts/health.sh"
      return 1
    }
  fi

  for hook in manual-info start stop; do
    script_path="${component_path}/scripts/${hook}.sh"
    [[ -f "${script_path}" ]] || continue
    bash -n "${script_path}" || {
      e2e_die "component ${component_key} has invalid bash syntax in scripts/${hook}.sh"
      return 1
    }
  done

  return 0
}

e2e_validate_all_discovered_component_contracts() {
  local component_key
  local component_type
  local component_name
  local validated_components=0
  local validated_managed_servers=0

  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    component_type=$(e2e_component_type "${component_key}")
    component_name=$(e2e_component_name "${component_key}")

    e2e_component_validate_script_syntax \
      "${component_key}" \
      "${E2E_COMPONENT_PATH[${component_key}]}" \
      "${E2E_COMPONENT_RUNTIME_KIND[${component_key}]}" || return 1

    if [[ "${component_type}" == 'managed-server' ]]; then
      e2e_validate_managed_server_fixture_tree "${component_name}" || return 1
      ((validated_managed_servers += 1))
    fi

    ((validated_components += 1))
  done

  e2e_info "component validation OK components=${validated_components} managed-servers=${validated_managed_servers}"
  return 0
}

e2e_validate_managed_server_security_selection() {
  if [[ "${E2E_MANAGED_SERVER}" == 'none' ]]; then
    if e2e_is_explicit 'managed-server-auth-type'; then
      e2e_die '--managed-server-auth-type requires a selected managed-server component'
      return 1
    fi
    if e2e_is_explicit 'managed-server-mtls' && [[ "${E2E_MANAGED_SERVER_MTLS}" == 'true' ]]; then
      e2e_die '--managed-server-mtls requires a selected managed-server component'
      return 1
    fi
    return 0
  fi

  local component_key
  component_key=$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")

  local supported_features=${E2E_COMPONENT_MANAGED_SERVER_SECURITY_FEATURES[${component_key}]:-}
  local required_features=${E2E_COMPONENT_MANAGED_SERVER_REQUIRED_SECURITY_FEATURES[${component_key}]:-}
  local feature
  local selected
  local selected_auth_feature

  if [[ -z "${E2E_MANAGED_SERVER_AUTH_TYPE:-}" ]]; then
    E2E_MANAGED_SERVER_AUTH_TYPE=$(e2e_managed_server_default_auth_type "${E2E_MANAGED_SERVER}" "${supported_features}" "${required_features}") || return 1
    e2e_info "managed-server auth-type defaulted component=${E2E_MANAGED_SERVER} auth-type=${E2E_MANAGED_SERVER_AUTH_TYPE}"
  fi

  selected_auth_feature=$(e2e_managed_server_auth_feature_for_type "${E2E_MANAGED_SERVER_AUTH_TYPE}") || {
    e2e_die "invalid selected managed-server auth-type: ${E2E_MANAGED_SERVER_AUTH_TYPE}"
    return 1
  }

  for feature in "${selected_auth_feature}" mtls; do
    if e2e_managed_server_feature_enabled "${feature}"; then
      if ! e2e_managed_server_feature_spec_supports "${supported_features}" "${feature}"; then
        if e2e_managed_server_security_feature_is_auth "${feature}"; then
          e2e_die "managed-server ${E2E_MANAGED_SERVER} does not support selected auth-type: ${E2E_MANAGED_SERVER_AUTH_TYPE}"
        else
          e2e_die "managed-server ${E2E_MANAGED_SERVER} does not support selected security feature: ${feature}"
        fi
        return 1
      fi
    fi
  done

  for feature in ${required_features}; do
    selected='false'
    if e2e_managed_server_feature_enabled "${feature}"; then
      selected='true'
    fi
    if [[ "${selected}" != 'true' ]]; then
      if e2e_managed_server_security_feature_is_auth "${feature}"; then
        local required_auth_type
        required_auth_type=$(e2e_managed_server_auth_type_for_feature "${feature}") || return 1
        e2e_die "managed-server ${E2E_MANAGED_SERVER} requires auth-type ${required_auth_type}"
      else
        e2e_die "managed-server ${E2E_MANAGED_SERVER} requires security feature ${feature}=true"
      fi
      return 1
    fi
  done

  return 0
}

e2e_validate_selection() {
  if [[ "${E2E_MANAGED_SERVER}" != 'none' ]] && ! e2e_component_exists 'managed-server' "${E2E_MANAGED_SERVER}"; then
    e2e_die "unknown managed-server component: ${E2E_MANAGED_SERVER}"
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

  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]] && ! e2e_component_exists 'proxy' "$(e2e_proxy_component_name)"; then
    e2e_die "unknown proxy component: $(e2e_proxy_component_name)"
    return 1
  fi

  if [[ "${E2E_MANAGED_SERVER}" != 'none' ]] && ! e2e_component_supports_connection 'managed-server' "${E2E_MANAGED_SERVER}" "${E2E_MANAGED_SERVER_CONNECTION}"; then
    e2e_die "managed-server ${E2E_MANAGED_SERVER} does not support connection ${E2E_MANAGED_SERVER_CONNECTION}"
    return 1
  fi

  e2e_validate_managed_server_security_selection || return 1

  if [[ "${E2E_MANAGED_SERVER}" != 'none' ]]; then
    e2e_validate_managed_server_fixture_tree "${E2E_MANAGED_SERVER}" || return 1
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

  if [[ "${E2E_MANAGED_SERVER}" != 'none' ]]; then
    E2E_SELECTED_COMPONENT_KEYS+=("$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")")
  fi

  if [[ "${E2E_SECRET_PROVIDER}" != 'none' ]]; then
    E2E_SELECTED_COMPONENT_KEYS+=("$(e2e_component_key 'secret-provider' "${E2E_SECRET_PROVIDER}")")
  fi

  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    E2E_SELECTED_COMPONENT_KEYS+=("$(e2e_proxy_component_key)")
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
  E2E_CAPABILITY_SET["platform=${E2E_PLATFORM}"]=1
  E2E_CAPABILITY_SET["repo-type=${E2E_REPO_TYPE}"]=1
  E2E_CAPABILITY_SET["managed-server=${E2E_MANAGED_SERVER}"]=1
  E2E_CAPABILITY_SET["managed-server-connection=${E2E_MANAGED_SERVER_CONNECTION}"]=1
  E2E_CAPABILITY_SET["managed-server-auth-type=${E2E_MANAGED_SERVER_AUTH_TYPE}"]=1
  E2E_CAPABILITY_SET["managed-server-mtls=${E2E_MANAGED_SERVER_MTLS}"]=1
  E2E_CAPABILITY_SET["proxy-mode=${E2E_PROXY_MODE:-none}"]=1
  E2E_CAPABILITY_SET["proxy-auth-type=$(e2e_effective_proxy_auth_type)"]=1
  E2E_CAPABILITY_SET["secret-provider=${E2E_SECRET_PROVIDER}"]=1
  E2E_CAPABILITY_SET["secret-provider-connection=${E2E_SECRET_PROVIDER_CONNECTION}"]=1

  if [[ -n "${E2E_GIT_PROVIDER}" ]]; then
    E2E_CAPABILITY_SET["git-provider=${E2E_GIT_PROVIDER}"]=1
    E2E_CAPABILITY_SET["git-provider-connection=${E2E_GIT_PROVIDER_CONNECTION}"]=1
  fi

  if [[ "${E2E_SECRET_PROVIDER}" != 'none' ]]; then
    E2E_CAPABILITY_SET['has-secret-provider']=1
  fi

  if [[ "${E2E_MANAGED_SERVER}" != 'none' ]]; then
    E2E_CAPABILITY_SET['has-managed-server']=1
  fi

  if [[ "${E2E_MANAGED_SERVER}" != 'none' && "${E2E_MANAGED_SERVER_MTLS}" == 'true' ]]; then
    E2E_CAPABILITY_SET['has-managed-server-mtls']=1
  fi

  if [[ "${E2E_PROXY_MODE:-none}" != 'none' ]]; then
    E2E_CAPABILITY_SET['has-proxy']=1
  fi

  if [[ "${E2E_GIT_PROVIDER_CONNECTION}" == 'remote' || "${E2E_MANAGED_SERVER_CONNECTION}" == 'remote' || "${E2E_SECRET_PROVIDER_CONNECTION}" == 'remote' ]]; then
    E2E_CAPABILITY_SET['remote-selection']=1
  fi
}

e2e_has_capability() {
  local capability=$1
  [[ "${E2E_CAPABILITY_SET[${capability}]:-0}" == '1' ]]
}

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components_hooks.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components_runtime.sh"

e2e_preflight_requirements() {
  e2e_info 'preflight checking required commands: bash go git jq curl'
  e2e_require_command bash || return 1
  e2e_require_command go || return 1
  e2e_require_command git || return 1
  e2e_require_command jq || return 1
  e2e_require_command curl || return 1

  local needs_container_runtime=0
  if e2e_component_has_local_containerized_runtime; then
    needs_container_runtime=1
  fi

  if ((needs_container_runtime == 1)); then
    case "${E2E_PLATFORM}" in
      compose)
        e2e_info "preflight checking compose runtime: ${E2E_CONTAINER_ENGINE}"
        e2e_validate_container_engine || return 1
        e2e_require_command "${E2E_CONTAINER_ENGINE}" || return 1
        e2e_compose_cmd version >/dev/null || return 1
        ;;
      kubernetes)
        e2e_info "preflight checking kubernetes runtime with kind provider engine=${E2E_CONTAINER_ENGINE}"
        e2e_validate_container_engine || return 1
        e2e_require_command "${E2E_CONTAINER_ENGINE}" || return 1
        e2e_require_command kind || return 1
        e2e_require_command kubectl || return 1
        e2e_require_command envsubst || return 1

        local kind_provider_output=''
        local kind_provider_rc=0
        set +e
        kind_provider_output=$(e2e_kind_run_raw get clusters 2>&1)
        kind_provider_rc=$?
        set -e

        if ((kind_provider_rc != 0)) && [[ "${kind_provider_output}" != *'No kind clusters found.'* ]]; then
          if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
            e2e_die 'kind provider check failed for podman; verify kind supports podman on this host (KIND_EXPERIMENTAL_PROVIDER=podman)'
          else
            e2e_die "kind provider check failed for engine=${E2E_CONTAINER_ENGINE}"
          fi
          return 1
        fi
        ;;
      *)
        e2e_die "unsupported platform in preflight: ${E2E_PLATFORM}"
        return 1
        ;;
    esac
  fi
}
