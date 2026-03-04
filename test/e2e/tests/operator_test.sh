#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_operator_libs() {
  source_e2e_libs common profile operator
}

prepare_operator_handoff_env() {
  local tmp=$1

  export E2E_PROFILE='operator'
  export E2E_RUN_ID='operator-handoff-test'
  export E2E_RUNS_DIR="${tmp}/runs"
  export E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  export E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  export E2E_CONTEXT_FILE="${E2E_RUN_DIR}/contexts.yaml"
  export E2E_BIN="${E2E_RUN_DIR}/bin/declarest"
  export E2E_PLATFORM='kubernetes'
  export E2E_KUBECONFIG="${tmp}/kubeconfig"
  export E2E_KIND_CLUSTER_NAME='declarest-e2e-operator'
  export E2E_K8S_NAMESPACE='declarest-operator'
  export E2E_OPERATOR_NAMESPACE='declarest-operator'
  export E2E_OPERATOR_MANAGER_DEPLOYMENT='declarest-operator'
  export E2E_OPERATOR_MANAGER_POD='declarest-operator-77b8f6fcb9-l9j6k'
  export E2E_OPERATOR_IMAGE='localhost/declarest/e2e-operator-manager:operator-handoff-test'
  export E2E_OPERATOR_SYNC_POLICY_NAME='declarest-e2e-sync-policy'
  export E2E_REPO_TYPE='git'
  export E2E_GIT_PROVIDER='gitea'
  export E2E_GIT_PROVIDER_CONNECTION='local'

  mkdir -p "${E2E_STATE_DIR}" "$(dirname -- "${E2E_BIN}")"
  : >"${E2E_CONTEXT_FILE}"
  : >"${E2E_KUBECONFIG}"
  cat >"${E2E_BIN}" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${E2E_BIN}"

  local provider_state="${E2E_STATE_DIR}/git-provider-gitea.env"
  : >"${provider_state}"
  e2e_write_state_value "${provider_state}" 'GIT_REMOTE_URL' 'http://127.0.0.1:3000/declarest-e2e/declarest-e2e.git'
  e2e_write_state_value "${provider_state}" 'GITEA_BASE_URL' 'http://127.0.0.1:3000'
  e2e_write_state_value "${provider_state}" 'GITEA_ADMIN_USERNAME' 'gitea-admin'
  e2e_write_state_value "${provider_state}" 'GITEA_ADMIN_PASSWORD' 'gitea-pass'
}

test_operator_example_resource_mapping() {
  load_operator_libs

  E2E_MANAGED_SERVER='simple-api-server'
  assert_eq "$(e2e_operator_example_resource_path)" "/api/projects/operator-demo"
  assert_contains "$(e2e_operator_example_resource_payload)" "\"id\":\"operator-demo\""

  E2E_MANAGED_SERVER='keycloak'
  assert_eq "$(e2e_operator_example_resource_path)" "/admin/realms/operator-demo"
  assert_contains "$(e2e_operator_example_resource_payload)" "\"realm\":\"operator-demo\""
}

test_operator_handoff_prints_managed_server_specific_commands() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  prepare_operator_handoff_env "${tmp}"
  E2E_MANAGED_SERVER='simple-api-server'

  local output
  output=$(e2e_profile_operator_handoff 'e2e-operator')

  assert_contains "${output}" "resource save '/api/projects/operator-demo' --payload"
  assert_contains "${output}" "resource get '/api/projects/operator-demo' --source remote-server"
  assert_contains "${output}" "manager-deployment: declarest-operator"
  assert_contains "${output}" "kubectl --kubeconfig \"${E2E_KUBECONFIG}\" -n \"${E2E_OPERATOR_NAMESPACE}\" logs deployment/\"${E2E_OPERATOR_MANAGER_DEPLOYMENT}\" --tail=80"
  assert_contains "${output}" "How to connect kubectl to this kind cluster:"
  assert_contains "${output}" "Repository provider access:"
  assert_contains "${output}" "web login: http://127.0.0.1:3000/user/login"
  assert_not_contains "${output}" "/customers/demo"

  local setup_script reset_script
  setup_script=$(e2e_manual_env_setup_script_path)
  reset_script=$(e2e_manual_env_reset_script_path)
  assert_path_exists "${setup_script}"
  assert_path_exists "${reset_script}"
}

test_operator_rewrites_local_urls_for_cluster_services() {
  load_operator_libs

  E2E_PLATFORM='kubernetes'
  E2E_K8S_NAMESPACE='declarest-test'
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_GIT_PROVIDER='gitea'

  local rewritten
  rewritten=$(e2e_operator_rewrite_local_url_to_service 'http://127.0.0.1:3000/root/repo.git' 'git-provider-gitea' '3000')
  assert_eq "${rewritten}" "http://git-provider-gitea.declarest-test.svc.cluster.local:3000/root/repo.git"

  rewritten=$(e2e_operator_rewrite_repo_url_for_cluster 'http://localhost:3000/root/repo.git')
  assert_eq "${rewritten}" "http://git-provider-gitea.declarest-test.svc.cluster.local:3000/root/repo.git"

  rewritten=$(e2e_operator_rewrite_local_url_to_service 'https://example.com/api' 'managed-server-keycloak' '8080')
  assert_eq "${rewritten}" "https://example.com/api"
}

test_operator_example_resource_mapping
test_operator_handoff_prints_managed_server_specific_commands
test_operator_rewrites_local_urls_for_cluster_services
