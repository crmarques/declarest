#!/usr/bin/env bash

declare -Ag E2E_COMPONENT_PATH=()
declare -Ag E2E_COMPONENT_CONNECTIONS=()
declare -Ag E2E_COMPONENT_DEFAULT_CONNECTION=()
declare -Ag E2E_COMPONENT_REQUIRES_DOCKER=()
declare -Ag E2E_COMPONENT_CONTRACT_VERSION=()
declare -Ag E2E_COMPONENT_RUNTIME_KIND=()
declare -Ag E2E_COMPONENT_DEPENDS_ON=()
declare -Ag E2E_COMPONENT_DESCRIPTION=()
declare -Ag E2E_COMPONENT_DEFAULT_SELECTIONS=()
declare -Ag E2E_COMPONENT_PROJECT=()
declare -Ag E2E_COMPONENT_MANAGED_SERVICE_SECURITY_FEATURES=()
declare -Ag E2E_COMPONENT_MANAGED_SERVICE_REQUIRED_SECURITY_FEATURES=()
declare -Ag E2E_COMPONENT_SERVICE_PORT=()
declare -Ag E2E_COMPONENT_METADATA_BUNDLE_REF=()
declare -Ag E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH=()
declare -Ag E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD=()
declare -Ag E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER=()
declare -Ag E2E_COMPONENT_REPO_PROVIDER_LOGIN_PATH=()
declare -Ag E2E_COMPONENT_OPENAPI_SPEC=()
declare -Ag E2E_CAPABILITY_SET=()

declare -ag E2E_COMPONENT_KEYS=()
declare -ag E2E_SELECTED_COMPONENT_KEYS=()
declare -ag E2E_STARTED_COMPONENT_KEYS=()

e2e_proxy_component_name() {
  printf 'forward-proxy\n'
}

e2e_proxy_component_key() {
  e2e_component_key 'proxy' "$(e2e_proxy_component_name)"
}

e2e_managed_service_security_feature_is_auth() {
  local feature=$1

  case "${feature}" in
    none|basic-auth|oauth2|custom-header)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_managed_service_auth_feature_for_type() {
  local auth_type=$1

  case "${auth_type}" in
    none)
      printf 'none\n'
      ;;
    basic)
      printf 'basic-auth\n'
      ;;
    oauth2)
      printf 'oauth2\n'
      ;;
    custom-header)
      printf 'custom-header\n'
      ;;
    prompt)
      printf 'basic-auth\n'
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_managed_service_auth_type_for_feature() {
  local feature=$1

  case "${feature}" in
    none)
      printf 'none\n'
      ;;
    basic-auth)
      printf 'basic\n'
      ;;
    oauth2)
      printf 'oauth2\n'
      ;;
    custom-header)
      printf 'custom-header\n'
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_managed_service_feature_enabled() {
  local feature=$1

  case "${feature}" in
    none|basic-auth|oauth2|custom-header)
      local selected_auth_type
      local selected_auth_feature
      selected_auth_type=${E2E_MANAGED_SERVICE_AUTH_TYPE:-}
      [[ -n "${selected_auth_type}" ]] || return 1
      selected_auth_feature=$(e2e_managed_service_auth_feature_for_type "${selected_auth_type}") || return 1
      [[ "${selected_auth_feature}" == "${feature}" ]]
      ;;
    mtls)
      [[ "${E2E_MANAGED_SERVICE_MTLS}" == 'true' ]]
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_managed_service_feature_spec_supports() {
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

e2e_component_default_selection_supports() {
  local component_key=$1
  local selection=$2
  local declared=" ${E2E_COMPONENT_DEFAULT_SELECTIONS[${component_key}]:-} "

  [[ "${declared}" == *" ${selection} "* ]]
}

e2e_component_default_name_for_type() {
  local component_type=$1
  local selection=${2:-base}
  local component_key
  local selected_name=''

  for component_key in "${E2E_COMPONENT_KEYS[@]}"; do
    [[ "$(e2e_component_type "${component_key}")" == "${component_type}" ]] || continue
    e2e_component_default_selection_supports "${component_key}" "${selection}" || continue

    if [[ -n "${selected_name}" ]]; then
      e2e_die "multiple ${component_type} components declare DEFAULT_SELECTIONS=${selection}"
      return 1
    fi

    selected_name=$(e2e_component_name "${component_key}")
  done

  if [[ -z "${selected_name}" ]]; then
    e2e_die "no ${component_type} component declares DEFAULT_SELECTIONS=${selection}"
    return 1
  fi

  printf '%s\n' "${selected_name}"
}

e2e_component_catalog_ensure_discovered() {
  if ((${#E2E_COMPONENT_KEYS[@]} > 0)); then
    return 0
  fi

  e2e_discover_components
}

e2e_component_connection_for_key() {
  local component_key=$1
  local component_type

  component_type=$(e2e_component_type "${component_key}")
  case "${component_type}" in
    managed-service)
      printf '%s\n' "${E2E_MANAGED_SERVICE_CONNECTION}"
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

e2e_component_compose_file() {
  local component_key=$1
  printf '%s/compose/compose.yaml\n' "${E2E_COMPONENT_PATH[${component_key}]}"
}

e2e_component_k8s_dir() {
  local component_key=$1
  printf '%s/k8s\n' "${E2E_COMPONENT_PATH[${component_key}]}"
}

e2e_component_is_containerized() {
  local component_key=$1
  [[ "${E2E_COMPONENT_RUNTIME_KIND[${component_key}]:-native}" == 'compose' ]]
}

e2e_component_k8s_label_key() {
  local component_key=$1
  local component_type
  local component_name
  component_type=$(e2e_component_type "${component_key}")
  component_name=$(e2e_component_name "${component_key}")
  printf '%s-%s\n' "${component_type}" "${component_name}"
}

e2e_seed_local_metadata_bundle_cache_locked() {
  local bundle_ref=$1
  local metadata_source=$2
  local openapi_source=${3:-}
  local bundle_name=${bundle_ref%%:*}
  local bundle_version=${bundle_ref#*:}
  local cache_root="${HOME}/.declarest/metadata-bundles"
  local cache_dir="${cache_root}/${bundle_name}-${bundle_version}"

  [[ -d "${metadata_source}" ]] || return 0

  mkdir -p "${cache_root}" || return 1

  local stage_dir
  stage_dir=$(mktemp -d "${cache_dir}.stage.XXXXXX") || return 1

  mkdir -p "${stage_dir}/metadata" || {
    rm -rf -- "${stage_dir}"
    return 1
  }

  if ! cp -R "${metadata_source}/." "${stage_dir}/metadata/"; then
    rm -rf -- "${stage_dir}"
    e2e_die "failed to seed metadata bundle cache from ${metadata_source}"
    return 1
  fi

  if [[ -f "${openapi_source}" ]]; then
    if ! cp "${openapi_source}" "${stage_dir}/openapi.yaml"; then
      rm -rf -- "${stage_dir}"
      e2e_die "failed to copy bundle openapi source ${openapi_source}"
      return 1
    fi
  fi

  {
    printf 'apiVersion: declarest.io/v1alpha1\n'
    printf 'kind: MetadataBundle\n'
    printf 'name: %s\n' "${bundle_name}"
    printf 'version: %s\n' "${bundle_version}"
    printf 'description: E2E metadata bundle for %s.\n' "${E2E_MANAGED_SERVICE:-managed-service}"
    printf 'declarest:\n'
    printf '  metadataRoot: metadata\n'
    if [[ -f "${openapi_source}" ]]; then
      printf '  openapi: openapi.yaml\n'
    fi
    printf 'distribution:\n'
    printf '  artifactTemplate: %s-{version}.tar.gz\n' "${bundle_name}"
  } >"${stage_dir}/bundle.yaml" || {
    rm -rf -- "${stage_dir}"
    return 1
  }

  : >"${stage_dir}/.declarest-bundle-ready" || {
    rm -rf -- "${stage_dir}"
    return 1
  }

  local retired_dir="${cache_dir}.retired.$$"
  rm -rf -- "${retired_dir}"
  if [[ -d "${cache_dir}" ]] && ! mv "${cache_dir}" "${retired_dir}"; then
    rm -rf -- "${stage_dir}"
    e2e_die "failed to retire stale metadata bundle cache at ${cache_dir}"
    return 1
  fi
  if ! mv "${stage_dir}" "${cache_dir}"; then
    [[ -d "${retired_dir}" ]] && mv "${retired_dir}" "${cache_dir}"
    rm -rf -- "${stage_dir}"
    e2e_die "failed to finalize metadata bundle cache at ${cache_dir}"
    return 1
  fi
  rm -rf -- "${retired_dir}"
  e2e_info "seeded local metadata bundle cache bundle=${bundle_ref} dir=${cache_dir}"
}

e2e_seed_local_metadata_bundle_cache() {
  local bundle_ref=$1
  local bundle_name=${bundle_ref%%:*}
  local bundle_version=${bundle_ref#*:}

  e2e_with_lock "metadata-bundle-${bundle_name}-${bundle_version}" \
    e2e_seed_local_metadata_bundle_cache_locked \
    "$@"
}

e2e_prepare_metadata_workspace_copy() {
  local metadata_source=$1
  local workspace_dir="${E2E_RUN_DIR}/managed-service-metadata"

  [[ -d "${metadata_source}" ]] || return 0

  rm -rf -- "${workspace_dir}"
  mkdir -p "${workspace_dir}" || {
    e2e_die "failed to create metadata workspace dir=${workspace_dir}"
    return 1
  }
  cp -R "${metadata_source}/." "${workspace_dir}/" || {
    e2e_die "failed to copy metadata workspace from ${metadata_source} to ${workspace_dir}"
    return 1
  }

  E2E_METADATA_DIR="${workspace_dir}"
  export E2E_METADATA_DIR
  e2e_info "managed-service metadata workspace prepared source=${metadata_source} dir=${workspace_dir}"
}

e2e_prepare_metadata_workspace() {
  unset E2E_METADATA_DIR
  unset E2E_METADATA_BUNDLE

  if [[ "${E2E_MANAGED_SERVICE:-}" == 'none' ]]; then
    return 0
  fi

  local resource_component_key
  resource_component_key=$(e2e_component_key 'managed-service' "${E2E_MANAGED_SERVICE}")
  local component_dir="${E2E_COMPONENT_PATH[${resource_component_key}]:-}"
  if [[ -z "${component_dir}" ]]; then
    return 0
  fi

  case "${E2E_METADATA:-bundle}" in
    bundle)
      local metadata_bundle
      local metadata_source="${component_dir}/metadata"
      local openapi_source="${component_dir}/openapi.yaml"
      metadata_bundle=${E2E_COMPONENT_METADATA_BUNDLE_REF[${resource_component_key}]:-}
      if [[ -z "${metadata_bundle}" ]]; then
        if [[ -d "${metadata_source}" ]]; then
          e2e_prepare_metadata_workspace_copy "${metadata_source}" || return 1
          e2e_info "metadata source bundle has no component-declared bundle ref for managed-service=${E2E_MANAGED_SERVICE}; using metadata workspace copy"
          return 0
        fi

        e2e_info "metadata source bundle has no component-declared bundle ref for managed-service=${E2E_MANAGED_SERVICE}; continuing without metadata.bundle"
        return 0
      fi
      e2e_seed_local_metadata_bundle_cache "${metadata_bundle}" "${metadata_source}" "${openapi_source}" || return 1
      E2E_METADATA_BUNDLE="${metadata_bundle}"
      export E2E_METADATA_BUNDLE
      e2e_info "managed-service metadata bundle selected bundle=${metadata_bundle}"
      return 0
      ;;
    dir)
      local metadata_source="${component_dir}/metadata"
      if [[ ! -d "${metadata_source}" ]]; then
        return 0
      fi

      e2e_prepare_metadata_workspace_copy "${metadata_source}" || return 1
      e2e_info "managed-service metadata directory selected from source=${metadata_source}"
      return 0
      ;;
    *)
      e2e_die "invalid metadata source: ${E2E_METADATA:-}"
      return 1
      ;;
  esac
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

e2e_component_openapi_source_path() {
  local component_key=$1
  local component_dir="${E2E_COMPONENT_PATH[${component_key}]:-}"

  if [[ -z "${component_dir}" ]]; then
    return 1
  fi

  local spec_path="${component_dir}/openapi.yaml"
  if [[ ! -f "${spec_path}" ]]; then
    return 1
  fi

  printf '%s\n' "${spec_path}"
  return 0
}

e2e_component_install_openapi_spec() {
  local component_key=$1
  local spec_src

  spec_src=$(e2e_component_openapi_source_path "${component_key}") || return 0

  local component_name
  component_name=$(e2e_component_name "${component_key}")
  local dest="${E2E_RUN_DIR}/${component_name}-openapi.yaml"

  rm -f "${dest}"
  if ! cp -- "${spec_src}" "${dest}"; then
    e2e_die "failed to copy openapi spec for component ${component_key}"
    return 1
  fi

  E2E_COMPONENT_OPENAPI_SPEC["${component_key}"]="${dest}"
  e2e_info "managed-service openapi spec key=${component_key} file=${dest}"
  return 0
}

e2e_prepare_component_openapi_specs() {
  if [[ "${E2E_METADATA:-bundle}" == 'bundle' ]]; then
    E2E_COMPONENT_OPENAPI_SPEC=()
    e2e_info 'managed-service openapi spec copy skipped: metadata source bundle'
    return 0
  fi

  local component_key

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    if ! e2e_component_install_openapi_spec "${component_key}"; then
      return 1
    fi
  done

  return 0
}

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components_catalog.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components_validate.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components_hooks.sh"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib/components_runtime.sh"
