#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_components_libs() {
  source_e2e_lib "common"
  source_e2e_lib "args"
  source_e2e_lib "components"
}

write_hook_script() {
  local path=$1
  cat >"${path}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF
  chmod +x "${path}"
}

create_component_common() {
  local component_dir=$1
  mkdir -p "${component_dir}/scripts"
  write_hook_script "${component_dir}/scripts/init.sh"
  write_hook_script "${component_dir}/scripts/configure-auth.sh"
  write_hook_script "${component_dir}/scripts/context.sh"
}

create_repo_type_component() {
  local root=$1
  local include_contract_version=${2:-true}
  local component_dir="${root}/components/repo-type/filesystem"
  create_component_common "${component_dir}"
  {
    printf 'COMPONENT_TYPE=repo-type\n'
    printf 'COMPONENT_NAME=filesystem\n'
    if [[ "${include_contract_version}" == 'true' ]]; then
      printf 'COMPONENT_CONTRACT_VERSION=1\n'
    fi
    printf 'SUPPORTED_CONNECTIONS="local"\n'
    printf 'DEFAULT_CONNECTION=local\n'
    printf 'REQUIRES_DOCKER=false\n'
    printf 'COMPONENT_RUNTIME_KIND=native\n'
    printf 'COMPONENT_DEPENDS_ON=""\n'
    printf 'DESCRIPTION="Filesystem repo"\n'
  } >"${component_dir}/component.env"
}

create_managed_server_component() {
  local root=$1
  local include_identity_fields=${2:-true}
  local include_compose_artifacts=${3:-true}
  local include_k8s_artifacts=${4:-true}
  local metadata_extension=${5:-json}
  local payload_extension=${6:-json}
  local component_dir="${root}/components/managed-server/demo"
  local metadata_file="${component_dir}/repo-template/api/items/_/metadata.${metadata_extension}"
  create_component_common "${component_dir}"
  write_hook_script "${component_dir}/scripts/health.sh"
  if [[ "${include_compose_artifacts}" == 'true' ]]; then
    mkdir -p "${component_dir}/compose"
    cat >"${component_dir}/compose/compose.yaml" <<'EOF'
services: {}
EOF
  fi
  if [[ "${include_k8s_artifacts}" == 'true' ]]; then
    mkdir -p "${component_dir}/k8s"
    cat >"${component_dir}/k8s/service.yaml" <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: demo
EOF
  fi
  mkdir -p "${component_dir}/repo-template/api/items/alpha"
  mkdir -p "${component_dir}/repo-template/api/items/_"
  cat >"${component_dir}/repo-template/api/items/alpha/resource.${payload_extension}" <<'EOF'
{"id":"alpha","name":"alpha"}
EOF
  if [[ "${metadata_extension}" == 'yaml' ]]; then
    if [[ "${include_identity_fields}" == 'true' ]]; then
      cat >"${metadata_file}" <<'EOF'
resourceInfo:
  idFromAttribute: id
  aliasFromAttribute: name
EOF
    else
      cat >"${metadata_file}" <<'EOF'
resourceInfo:
  idFromAttribute: id
EOF
    fi
  elif [[ "${include_identity_fields}" == 'true' ]]; then
    cat >"${metadata_file}" <<'EOF'
{"resourceInfo":{"idFromAttribute":"id","aliasFromAttribute":"name"}}
EOF
  else
    cat >"${metadata_file}" <<'EOF'
{"resourceInfo":{"idFromAttribute":"id"}}
EOF
  fi
  {
    printf 'COMPONENT_TYPE=managed-server\n'
    printf 'COMPONENT_NAME=demo\n'
    printf 'COMPONENT_CONTRACT_VERSION=1\n'
    printf 'SUPPORTED_CONNECTIONS="local"\n'
    printf 'DEFAULT_CONNECTION=local\n'
    printf 'REQUIRES_DOCKER=true\n'
    printf 'COMPONENT_RUNTIME_KIND=compose\n'
    printf 'COMPONENT_DEPENDS_ON=""\n'
    printf 'SUPPORTED_SECURITY_FEATURES="oauth2"\n'
    printf 'REQUIRED_SECURITY_FEATURES=""\n'
    printf 'DESCRIPTION="Demo managed server"\n'
  } >"${component_dir}/component.env"
}

create_native_secret_provider_component() {
  local root=$1
  local component_dir="${root}/components/secret-provider/file"
  create_component_common "${component_dir}"
  {
    printf 'COMPONENT_TYPE=secret-provider\n'
    printf 'COMPONENT_NAME=file\n'
    printf 'COMPONENT_CONTRACT_VERSION=1\n'
    printf 'SUPPORTED_CONNECTIONS=\"local\"\n'
    printf 'DEFAULT_CONNECTION=local\n'
    printf 'REQUIRES_DOCKER=false\n'
    printf 'COMPONENT_RUNTIME_KIND=native\n'
    printf 'COMPONENT_DEPENDS_ON=\"\"\n'
    printf 'DESCRIPTION=\"Native secret provider\"\n'
  } >"${component_dir}/component.env"
}

with_temp_e2e_dir() {
  local tmp
  tmp=$(new_temp_dir)
  trap "rm -rf '${tmp}'" RETURN
  mkdir -p "${tmp}/components"
  E2E_DIR="${tmp}"
  E2E_RUNS_DIR="${tmp}/.runs"
  "$@"
}

test_discover_rejects_missing_contract_version() {
  load_components_libs
  with_temp_e2e_dir _test_discover_rejects_missing_contract_version_impl
}

_test_discover_rejects_missing_contract_version_impl() {
  create_repo_type_component "${E2E_DIR}" false
  local output status
  set +e
  output=$(e2e_discover_components 2>&1)
  status=$?
  set -e
  assert_status "${status}" "1"
  assert_contains "${output}" "must declare COMPONENT_CONTRACT_VERSION"
}

test_validate_all_discovered_components_accepts_valid_fixture_identity() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_accepts_valid_fixture_identity_impl
}

_test_validate_all_discovered_components_accepts_valid_fixture_identity_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_managed_server_component "${E2E_DIR}" true
  e2e_discover_components
  e2e_validate_all_discovered_component_contracts >/dev/null
}

test_validate_all_discovered_components_accepts_valid_yaml_fixture_identity() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_accepts_valid_yaml_fixture_identity_impl
}

_test_validate_all_discovered_components_accepts_valid_yaml_fixture_identity_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_managed_server_component "${E2E_DIR}" true true true yaml
  e2e_discover_components
  e2e_validate_all_discovered_component_contracts >/dev/null
}

test_validate_all_discovered_components_accepts_valid_yaml_resource_payload() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_accepts_valid_yaml_resource_payload_impl
}

_test_validate_all_discovered_components_accepts_valid_yaml_resource_payload_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_managed_server_component "${E2E_DIR}" true true true json yaml
  e2e_discover_components
  e2e_validate_all_discovered_component_contracts >/dev/null
}

test_validate_all_discovered_components_allows_empty_managed_server_metadata_dir() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_allows_empty_managed_server_metadata_dir_impl
}

_test_validate_all_discovered_components_allows_empty_managed_server_metadata_dir_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_managed_server_component "${E2E_DIR}" true

  local component_dir="${E2E_DIR}/components/managed-server/demo"
  rm -f "${component_dir}/repo-template/api/items/_/metadata.json" "${component_dir}/repo-template/api/items/_/metadata.yaml"
  mkdir -p "${component_dir}/metadata"

  e2e_discover_components
  e2e_validate_all_discovered_component_contracts >/dev/null
}

test_validate_all_discovered_components_rejects_missing_fixture_identity() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_rejects_missing_fixture_identity_impl
}

_test_validate_all_discovered_components_rejects_missing_fixture_identity_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_managed_server_component "${E2E_DIR}" false
  e2e_discover_components

  local output status
  set +e
  output=$(e2e_validate_all_discovered_component_contracts 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "metadata fixture missing resourceInfo.idFromAttribute or resourceInfo.aliasFromAttribute"
}

test_managed_server_auth_type_defaults_prefer_oauth2() {
  load_components_libs

  E2E_EXPLICIT=()
  E2E_MANAGED_SERVER='demo'
  E2E_MANAGED_SERVER_AUTH_TYPE=''
  E2E_MANAGED_SERVER_MTLS='false'
  E2E_COMPONENT_MANAGED_SERVER_SECURITY_FEATURES=()
  E2E_COMPONENT_MANAGED_SERVER_REQUIRED_SECURITY_FEATURES=()
  E2E_COMPONENT_MANAGED_SERVER_SECURITY_FEATURES['managed-server:demo']='none basic-auth oauth2 mtls'
  E2E_COMPONENT_MANAGED_SERVER_REQUIRED_SECURITY_FEATURES['managed-server:demo']=''

  e2e_validate_managed_server_security_selection >/dev/null

  assert_eq "${E2E_MANAGED_SERVER_AUTH_TYPE}" "oauth2" "expected default auth-type election to prefer oauth2"
}

test_managed_server_auth_type_rejects_unsupported_selection() {
  load_components_libs

  E2E_EXPLICIT=()
  E2E_MANAGED_SERVER='demo'
  E2E_MANAGED_SERVER_AUTH_TYPE='custom-header'
  E2E_MANAGED_SERVER_MTLS='false'
  e2e_mark_explicit 'managed-server-auth-type'
  E2E_COMPONENT_MANAGED_SERVER_SECURITY_FEATURES=()
  E2E_COMPONENT_MANAGED_SERVER_REQUIRED_SECURITY_FEATURES=()
  E2E_COMPONENT_MANAGED_SERVER_SECURITY_FEATURES['managed-server:demo']='none oauth2'
  E2E_COMPONENT_MANAGED_SERVER_REQUIRED_SECURITY_FEATURES['managed-server:demo']=''

  local output status
  set +e
  output=$(e2e_validate_managed_server_security_selection 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "does not support selected auth-type: custom-header"
}

test_validate_all_discovered_components_rejects_missing_compose_artifacts() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_rejects_missing_compose_artifacts_impl
}

_test_validate_all_discovered_components_rejects_missing_compose_artifacts_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_managed_server_component "${E2E_DIR}" true false true

  local output status
  set +e
  output=$(e2e_discover_components 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "missing compose/compose.yaml"
}

test_validate_all_discovered_components_rejects_missing_k8s_artifacts() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_rejects_missing_k8s_artifacts_impl
}

_test_validate_all_discovered_components_rejects_missing_k8s_artifacts_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_managed_server_component "${E2E_DIR}" true true false

  local output status
  set +e
  output=$(e2e_discover_components 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "missing k8s artifact directory"
}

test_validate_all_discovered_components_accepts_native_without_runtime_artifacts() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_accepts_native_without_runtime_artifacts_impl
}

_test_validate_all_discovered_components_accepts_native_without_runtime_artifacts_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_native_secret_provider_component "${E2E_DIR}"
  e2e_discover_components
  e2e_validate_all_discovered_component_contracts >/dev/null
}

test_discover_rejects_missing_contract_version
test_validate_all_discovered_components_accepts_valid_fixture_identity
test_validate_all_discovered_components_rejects_missing_fixture_identity
test_managed_server_auth_type_defaults_prefer_oauth2
test_managed_server_auth_type_rejects_unsupported_selection
test_validate_all_discovered_components_rejects_missing_compose_artifacts
test_validate_all_discovered_components_accepts_valid_yaml_fixture_identity
test_validate_all_discovered_components_accepts_valid_yaml_resource_payload
test_validate_all_discovered_components_rejects_missing_k8s_artifacts
test_validate_all_discovered_components_accepts_native_without_runtime_artifacts
