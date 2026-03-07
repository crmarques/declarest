#!/usr/bin/env bash

declare -Ag E2E_COMPONENT_PATH=()
declare -Ag E2E_COMPONENT_CONNECTIONS=()
declare -Ag E2E_COMPONENT_DEFAULT_CONNECTION=()
declare -Ag E2E_COMPONENT_REQUIRES_DOCKER=()
declare -Ag E2E_COMPONENT_CONTRACT_VERSION=()
declare -Ag E2E_COMPONENT_RUNTIME_KIND=()
declare -Ag E2E_COMPONENT_DEPENDS_ON=()
declare -Ag E2E_COMPONENT_DESCRIPTION=()
declare -Ag E2E_COMPONENT_PROJECT=()
declare -Ag E2E_COMPONENT_MANAGED_SERVER_SECURITY_FEATURES=()
declare -Ag E2E_COMPONENT_MANAGED_SERVER_REQUIRED_SECURITY_FEATURES=()
declare -Ag E2E_COMPONENT_OPENAPI_SPEC=()
declare -Ag E2E_CAPABILITY_SET=()

declare -ag E2E_COMPONENT_KEYS=()
declare -ag E2E_SELECTED_COMPONENT_KEYS=()
declare -ag E2E_STARTED_COMPONENT_KEYS=()

e2e_managed_server_security_feature_is_auth() {
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

e2e_managed_server_auth_feature_for_type() {
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
    *)
      return 1
      ;;
  esac
}

e2e_managed_server_auth_type_for_feature() {
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

e2e_managed_server_feature_enabled() {
  local feature=$1

  case "${feature}" in
    none|basic-auth|oauth2|custom-header)
      local selected_auth_type
      local selected_auth_feature
      selected_auth_type=${E2E_MANAGED_SERVER_AUTH_TYPE:-}
      [[ -n "${selected_auth_type}" ]] || return 1
      selected_auth_feature=$(e2e_managed_server_auth_feature_for_type "${selected_auth_type}") || return 1
      [[ "${selected_auth_feature}" == "${feature}" ]]
      ;;
    mtls)
      [[ "${E2E_MANAGED_SERVER_MTLS}" == 'true' ]]
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_managed_server_feature_spec_supports() {
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
    managed-server)
      printf '%s\n' "${E2E_MANAGED_SERVER_CONNECTION}"
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

e2e_default_metadata_bundle_for_managed_server() {
  local managed_server=$1

  case "${managed_server}" in
    keycloak)
      printf 'keycloak-bundle:0.0.1\n'
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_seed_local_metadata_bundle_cache() {
  local bundle_ref=$1
  local metadata_source=$2
  local openapi_source=${3:-}
  local bundle_name=${bundle_ref%%:*}
  local bundle_version=${bundle_ref#*:}
  local cache_dir="${HOME}/.declarest/metadata-bundles/${bundle_name}-${bundle_version}"
  local metadata_file_name

  [[ -d "${metadata_source}" ]] || return 0

  metadata_file_name=$(e2e_metadata_file_name_for_root "${metadata_source}") || return 1

  rm -rf -- "${cache_dir}"
  mkdir -p "${cache_dir}/metadata" || return 1

  if ! cp -R "${metadata_source}/." "${cache_dir}/metadata/"; then
    e2e_die "failed to seed metadata bundle cache from ${metadata_source}"
    return 1
  fi

  if [[ -f "${openapi_source}" ]]; then
    cp "${openapi_source}" "${cache_dir}/openapi.yaml" || {
      e2e_die "failed to copy bundle openapi source ${openapi_source}"
      return 1
    }
  fi

  {
    printf 'apiVersion: declarest.io/v1alpha1\n'
    printf 'kind: MetadataBundle\n'
    printf 'name: %s\n' "${bundle_name}"
    printf 'version: %s\n' "${bundle_version}"
    printf 'description: E2E metadata bundle for %s.\n' "${E2E_MANAGED_SERVER:-managed-server}"
    printf 'declarest:\n'
    printf '  shorthand: %s\n' "${bundle_name}"
    printf '  metadataRoot: metadata\n'
    printf '  metadataFileName: %s\n' "${metadata_file_name}"
    if [[ -f "${openapi_source}" ]]; then
      printf '  openapi: openapi.yaml\n'
    fi
    printf 'distribution:\n'
    printf '  artifactTemplate: %s-{version}.tar.gz\n' "${bundle_name}"
  } >"${cache_dir}/bundle.yaml"

  : >"${cache_dir}/.declarest-bundle-ready"
  e2e_info "seeded local metadata bundle cache bundle=${bundle_ref} dir=${cache_dir}"
}

e2e_prepare_metadata_workspace() {
  unset E2E_METADATA_DIR
  unset E2E_METADATA_BUNDLE

  if [[ "${E2E_MANAGED_SERVER:-}" == 'none' ]]; then
    return 0
  fi

  local resource_component_key
  resource_component_key=$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")
  local component_dir="${E2E_COMPONENT_PATH[${resource_component_key}]:-}"
  if [[ -z "${component_dir}" ]]; then
    return 0
  fi

  case "${E2E_METADATA:-bundle}" in
    bundle)
      local metadata_bundle
      local metadata_source="${component_dir}/metadata"
      local openapi_source="${component_dir}/openapi.yaml"
      if ! metadata_bundle=$(e2e_default_metadata_bundle_for_managed_server "${E2E_MANAGED_SERVER}"); then
        if [[ -d "${metadata_source}" ]]; then
          E2E_METADATA_DIR="${metadata_source}"
          export E2E_METADATA_DIR
          e2e_info "metadata type bundle has no shorthand mapping for managed-server=${E2E_MANAGED_SERVER}; using component metadata dir=${metadata_source}"
          return 0
        fi

        e2e_info "metadata type bundle has no shorthand mapping for managed-server=${E2E_MANAGED_SERVER}; continuing without metadata.bundle"
        return 0
      fi
      e2e_seed_local_metadata_bundle_cache "${metadata_bundle}" "${metadata_source}" "${openapi_source}" || return 1
      E2E_METADATA_BUNDLE="${metadata_bundle}"
      export E2E_METADATA_BUNDLE
      e2e_info "managed-server metadata bundle selected bundle=${metadata_bundle}"
      return 0
      ;;
    base-dir)
      local metadata_source="${component_dir}/metadata"
      if [[ ! -d "${metadata_source}" ]]; then
        return 0
      fi

      E2E_METADATA_DIR="${metadata_source}"
      export E2E_METADATA_DIR
      e2e_info "managed-server metadata directory selected dir=${metadata_source}"
      return 0
      ;;
    *)
      e2e_die "invalid metadata type: ${E2E_METADATA:-}"
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
  e2e_info "managed-server openapi spec key=${component_key} file=${dest}"
  return 0
}

e2e_prepare_component_openapi_specs() {
  if [[ "${E2E_METADATA:-bundle}" == 'bundle' ]]; then
    E2E_COMPONENT_OPENAPI_SPEC=()
    e2e_info 'managed-server openapi spec copy skipped: metadata type bundle'
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
