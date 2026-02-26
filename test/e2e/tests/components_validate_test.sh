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

create_resource_server_component() {
  local root=$1
  local include_identity_fields=${2:-true}
  local component_dir="${root}/components/resource-server/demo"
  create_component_common "${component_dir}"
  write_hook_script "${component_dir}/scripts/health.sh"
  cat >"${component_dir}/compose.yaml" <<'EOF'
services: {}
EOF
  mkdir -p "${component_dir}/repo-template/api/items/alpha"
  mkdir -p "${component_dir}/repo-template/api/items/_"
  cat >"${component_dir}/repo-template/api/items/alpha/resource.json" <<'EOF'
{"id":"alpha","name":"alpha"}
EOF
  if [[ "${include_identity_fields}" == 'true' ]]; then
    cat >"${component_dir}/repo-template/api/items/_/metadata.json" <<'EOF'
{"resourceInfo":{"idFromAttribute":"id","aliasFromAttribute":"name"}}
EOF
  else
    cat >"${component_dir}/repo-template/api/items/_/metadata.json" <<'EOF'
{"resourceInfo":{"idFromAttribute":"id"}}
EOF
  fi
  {
    printf 'COMPONENT_TYPE=resource-server\n'
    printf 'COMPONENT_NAME=demo\n'
    printf 'COMPONENT_CONTRACT_VERSION=1\n'
    printf 'SUPPORTED_CONNECTIONS="local"\n'
    printf 'DEFAULT_CONNECTION=local\n'
    printf 'REQUIRES_DOCKER=true\n'
    printf 'COMPONENT_RUNTIME_KIND=compose\n'
    printf 'COMPONENT_DEPENDS_ON=""\n'
    printf 'SUPPORTED_SECURITY_FEATURES="oauth2"\n'
    printf 'REQUIRED_SECURITY_FEATURES=""\n'
    printf 'DESCRIPTION="Demo resource server"\n'
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
  create_resource_server_component "${E2E_DIR}" true
  e2e_discover_components
  e2e_validate_all_discovered_component_contracts >/dev/null
}

test_validate_all_discovered_components_rejects_missing_fixture_identity() {
  load_components_libs
  with_temp_e2e_dir _test_validate_all_discovered_components_rejects_missing_fixture_identity_impl
}

_test_validate_all_discovered_components_rejects_missing_fixture_identity_impl() {
  create_repo_type_component "${E2E_DIR}" true
  create_resource_server_component "${E2E_DIR}" false
  e2e_discover_components

  local output status
  set +e
  output=$(e2e_validate_all_discovered_component_contracts 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "metadata fixture missing resourceInfo.idFromAttribute or resourceInfo.aliasFromAttribute"
}

test_resource_server_auth_type_defaults_prefer_oauth2() {
  load_components_libs

  E2E_EXPLICIT=()
  E2E_RESOURCE_SERVER='demo'
  E2E_RESOURCE_SERVER_AUTH_TYPE=''
  E2E_RESOURCE_SERVER_MTLS='false'
  E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES=()
  E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES=()
  E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES['resource-server:demo']='none basic-auth oauth2 mtls'
  E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES['resource-server:demo']=''

  e2e_validate_resource_server_security_selection >/dev/null

  assert_eq "${E2E_RESOURCE_SERVER_AUTH_TYPE}" "oauth2" "expected default auth-type election to prefer oauth2"
}

test_resource_server_auth_type_rejects_unsupported_selection() {
  load_components_libs

  E2E_EXPLICIT=()
  E2E_RESOURCE_SERVER='demo'
  E2E_RESOURCE_SERVER_AUTH_TYPE='custom-header'
  E2E_RESOURCE_SERVER_MTLS='false'
  e2e_mark_explicit 'resource-server-auth-type'
  E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES=()
  E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES=()
  E2E_COMPONENT_RESOURCE_SERVER_SECURITY_FEATURES['resource-server:demo']='none oauth2'
  E2E_COMPONENT_RESOURCE_SERVER_REQUIRED_SECURITY_FEATURES['resource-server:demo']=''

  local output status
  set +e
  output=$(e2e_validate_resource_server_security_selection 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "does not support selected auth-type: custom-header"
}

test_discover_rejects_missing_contract_version
test_validate_all_discovered_components_accepts_valid_fixture_identity
test_validate_all_discovered_components_rejects_missing_fixture_identity
test_resource_server_auth_type_defaults_prefer_oauth2
test_resource_server_auth_type_rejects_unsupported_selection
