#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_context_libs() {
  unset \
    DECLAREST_E2E_LOCAL_ACCESS_HOST \
    E2E_PROXY_MODE \
    E2E_PROXY_AUTH_TYPE \
    E2E_PROXY_HTTP_URL \
    E2E_PROXY_HTTPS_URL \
    E2E_PROXY_NO_PROXY \
    E2E_PROXY_AUTH_USERNAME \
    E2E_PROXY_AUTH_PASSWORD \
    E2E_MANAGED_SERVER_AUTH_TYPE \
    E2E_MANAGED_SERVER \
    E2E_MANAGED_SERVER_CONNECTION \
    E2E_REPO_TYPE \
    E2E_GIT_PROVIDER \
    E2E_GIT_PROVIDER_CONNECTION \
    E2E_SECRET_PROVIDER \
    E2E_SECRET_PROVIDER_CONNECTION \
    E2E_PLATFORM \
    E2E_K8S_NAMESPACE \
    E2E_STATE_DIR || true

  source_e2e_lib "common"
  source_e2e_lib "args"
  source_e2e_lib "components"
  source_e2e_lib "operator"
  source_e2e_lib "context"
}

write_context_fixture() {
  local path=$1
  cat >"${path}" <<'EOF'
contexts:
  - name: e2e-basic
    managedServer:
      http:
        baseURL: http://127.0.0.1:8080
        auth:
          oauth2:
            tokenURL: http://127.0.0.1:8080/oauth/token
            grantType: client_credentials
            clientID: demo
            clientSecret: secret
    repository:
      git:
        local:
          baseDir: /tmp/repo
        remote:
          url: http://127.0.0.1:3000/acme/repo.git
          branch: main
          provider: gitea
          auth:
            basicAuth:
              username: git-user
              password: git-pass
    secretStore:
      vault:
        address: http://127.0.0.1:8200
        mount: secret
        pathPrefix: declarest-e2e
        kvVersion: 2
        auth:
          token: root-token
    metadata:
      bundle: keycloak-bundle:0.0.1
currentContext: e2e-basic
EOF
}

write_git_ssh_context_fixture() {
  local path=$1
  cat >"${path}" <<'EOF'
contexts:
  - name: e2e-basic
    managedServer:
      http:
        baseURL: http://127.0.0.1:8080
        auth:
          customHeaders:
            - header: Authorization
              value: token
    repository:
      git:
        local:
          baseDir: /tmp/repo
        remote:
          url: git@github.com:acme/repo.git
          branch: main
          provider: github
    metadata:
      bundle: keycloak-bundle:0.0.1
currentContext: e2e-basic
EOF
}

write_service_manifest() {
  local dir=$1
  local name=$2
  local mapping=$3

  mkdir -p "${dir}"
  cat >"${dir}/service.yaml" <<EOF
apiVersion: v1
kind: Service
metadata:
  name: ${name}
  annotations:
    declarest.e2e/port-forward: "${mapping}"
EOF
}

write_state_fixture() {
  local path=$1
  shift

  : >"${path}"
  while (($# > 1)); do
    e2e_write_state_value "${path}" "$1" "$2"
    shift 2
  done
}

test_inserts_proxy_blocks_across_proxiable_sections() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_PROXY_MODE='external'
  E2E_PROXY_HTTP_URL='http://proxy.example.com:3128'
  E2E_PROXY_HTTPS_URL='https://proxy.example.com:3128'
  E2E_PROXY_NO_PROXY='localhost,127.0.0.1'
  E2E_PROXY_AUTH_USERNAME='proxy-user'
  E2E_PROXY_AUTH_PASSWORD='proxy-pass'

  e2e_context_insert_proxy_config "${context_file}"

  assert_file_contains "${context_file}" "httpURL: 'http://proxy.example.com:3128'"
  assert_file_contains "${context_file}" "httpsURL: 'https://proxy.example.com:3128'"
  assert_file_contains "${context_file}" "noProxy: 'localhost,127.0.0.1'"
  assert_file_contains "${context_file}" "basic:"
  assert_file_contains "${context_file}" "username: 'proxy-user'"
  assert_file_contains "${context_file}" "password: 'proxy-pass'"

  local proxy_count
  proxy_count=$(grep -c '^[[:space:]]*proxy:$' "${context_file}" || true)
  assert_eq "${proxy_count}" "4" "expected proxy blocks for managedServer, repository, secretStore, and metadata"
}

test_inserts_prompt_proxy_auth_block_for_local_proxy() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  local proxy_state="${tmp}/state/proxy-forward-proxy.env"
  mkdir -p "${tmp}/state"
  write_context_fixture "${context_file}"

  E2E_STATE_DIR="${tmp}/state"
  E2E_PROXY_MODE='local'
  E2E_PROXY_AUTH_TYPE='prompt'

  write_state_fixture "${proxy_state}" \
    PROXY_HTTP_URL "http://127.0.0.1:3128" \
    PROXY_HTTPS_URL "http://127.0.0.1:3128" \
    PROXY_AUTH_TYPE "prompt" \
    PROXY_AUTH_USERNAME "generated-user" \
    PROXY_AUTH_PASSWORD "generated-pass"

  e2e_context_insert_proxy_config "${context_file}"

  assert_file_contains "${context_file}" "httpURL: 'http://127.0.0.1:3128'"
  assert_file_contains "${context_file}" "prompt:"
  assert_file_contains "${context_file}" "keepCredentialsForSession: true"
  assert_not_contains "$(cat "${context_file}")" "username: 'generated-user'"
  assert_not_contains "$(cat "${context_file}")" "password: 'generated-pass'"
}

test_skips_git_proxy_block_for_non_http_remote_url() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_git_ssh_context_fixture "${context_file}"

  E2E_PROXY_MODE='external'
  E2E_PROXY_HTTP_URL='http://proxy.example.com:3128'

  e2e_context_insert_proxy_config "${context_file}"

  local proxy_count
  proxy_count=$(grep -c '^[[:space:]]*proxy:$' "${context_file}" || true)
  assert_eq "${proxy_count}" "2" "expected proxy blocks only for managedServer and metadata"
}

test_rewrites_local_kubernetes_targets_for_local_proxy() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  local state_dir="${tmp}/state"
  local managed_rendered="${tmp}/rendered/managed-server"
  local repo_rendered="${tmp}/rendered/git-provider"
  local secret_rendered="${tmp}/rendered/secret-provider"
  mkdir -p "${state_dir}" "${tmp}/rendered"
  write_context_fixture "${context_file}"
  write_service_manifest "${managed_rendered}" "managed-server-simple-api-server" "18080:8080"
  write_service_manifest "${repo_rendered}" "git-provider-gitea" "13000:3000"
  write_service_manifest "${secret_rendered}" "secret-provider-vault" "18200:8200"

  E2E_STATE_DIR="${state_dir}"
  E2E_PLATFORM='kubernetes'
  E2E_K8S_NAMESPACE='declarest-test'
  E2E_PROXY_MODE='local'
  E2E_MANAGED_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_CONNECTION='local'
  E2E_REPO_TYPE='git'
  E2E_GIT_PROVIDER='gitea'
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_SECRET_PROVIDER='vault'
  E2E_SECRET_PROVIDER_CONNECTION='local'

  write_state_fixture "$(e2e_component_state_file 'proxy:forward-proxy')" \
    PROXY_HTTP_URL "http://127.0.0.1:3128" \
    PROXY_HTTPS_URL "http://127.0.0.1:3128" \
    PROXY_AUTH_TYPE "basic" \
    PROXY_AUTH_USERNAME "proxy-user" \
    PROXY_AUTH_PASSWORD "proxy-pass"
  write_state_fixture "$(e2e_component_state_file 'managed-server:simple-api-server')" \
    K8S_RENDERED_DIR "${managed_rendered}"
  write_state_fixture "$(e2e_component_state_file 'git-provider:gitea')" \
    K8S_RENDERED_DIR "${repo_rendered}"
  write_state_fixture "$(e2e_component_state_file 'secret-provider:vault')" \
    K8S_RENDERED_DIR "${secret_rendered}"

  e2e_context_insert_proxy_config "${context_file}"

  assert_file_contains "${context_file}" "baseURL: http://managed-server-simple-api-server.declarest-test.svc.cluster.local:8080"
  assert_file_contains "${context_file}" "tokenURL: http://managed-server-simple-api-server.declarest-test.svc.cluster.local:8080/oauth/token"
  assert_file_contains "${context_file}" "url: http://git-provider-gitea.declarest-test.svc.cluster.local:3000/acme/repo.git"
  assert_file_contains "${context_file}" "address: http://secret-provider-vault.declarest-test.svc.cluster.local:8200"
}

test_rewrites_local_compose_targets_for_local_proxy() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  local state_dir="${tmp}/state"
  mkdir -p "${state_dir}"
  write_context_fixture "${context_file}"

  E2E_STATE_DIR="${state_dir}"
  E2E_PLATFORM='compose'
  E2E_PROXY_MODE='local'
  E2E_MANAGED_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_CONNECTION='local'
  E2E_REPO_TYPE='git'
  E2E_GIT_PROVIDER='gitea'
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_SECRET_PROVIDER='vault'
  E2E_SECRET_PROVIDER_CONNECTION='local'
  DECLAREST_E2E_LOCAL_ACCESS_HOST='192.0.2.10'

  write_state_fixture "$(e2e_component_state_file 'proxy:forward-proxy')" \
    PROXY_HTTP_URL "http://127.0.0.1:3128" \
    PROXY_HTTPS_URL "http://127.0.0.1:3128" \
    PROXY_AUTH_TYPE "basic" \
    PROXY_AUTH_USERNAME "proxy-user" \
    PROXY_AUTH_PASSWORD "proxy-pass"
  write_state_fixture "$(e2e_component_state_file 'managed-server:simple-api-server')" \
    MANAGED_SERVER_BASE_URL "http://127.0.0.1:18080"
  write_state_fixture "$(e2e_component_state_file 'git-provider:gitea')" \
    GIT_REMOTE_URL "http://127.0.0.1:13000/acme/repo.git"
  write_state_fixture "$(e2e_component_state_file 'secret-provider:vault')" \
    VAULT_ADDRESS "http://127.0.0.1:18200"

  e2e_context_insert_proxy_config "${context_file}"

  assert_file_contains "${context_file}" "baseURL: http://192.0.2.10:18080"
  assert_file_contains "${context_file}" "tokenURL: http://192.0.2.10:18080/oauth/token"
  assert_file_contains "${context_file}" "url: http://192.0.2.10:13000/acme/repo.git"
  assert_file_contains "${context_file}" "address: http://192.0.2.10:18200"
}

test_rejects_proxy_enable_without_proxy_urls() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_PROXY_MODE='external'

  local output status
  set +e
  output=$(e2e_context_insert_proxy_config "${context_file}" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "--proxy-mode requires proxy HTTP and/or HTTPS URL configuration"
}

test_simple_api_server_context_emits_prompt_auth_block() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local state_file="${tmp}/simple-api.env"
  local fragment_file="${tmp}/simple-api-context.yaml"
  cat >"${state_file}" <<'EOF'
SIMPLE_API_SERVER_ENABLE_BASIC_AUTH=true
SIMPLE_API_SERVER_ENABLE_OAUTH2=false
SIMPLE_API_SERVER_ENABLE_MTLS=false
SIMPLE_API_SERVER_BASE_URL=http://127.0.0.1:18080
SIMPLE_API_SERVER_BASIC_AUTH_USERNAME=admin
SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD=admin
EOF

  E2E_COMPONENT_STATE_FILE="${state_file}" \
    E2E_COMPONENT_CONTEXT_FRAGMENT="${fragment_file}" \
    E2E_MANAGED_SERVER_AUTH_TYPE='prompt' \
    bash "${E2E_SCRIPT_DIR}/components/managed-server/simple-api-server/scripts/context.sh" "${fragment_file}"

  assert_file_contains "${fragment_file}" "prompt: {}"
  assert_not_contains "$(cat "${fragment_file}")" "basicAuth:"
  assert_not_contains "$(cat "${fragment_file}")" "username: admin"
  assert_not_contains "$(cat "${fragment_file}")" "password: admin"
}

test_inserts_proxy_blocks_across_proxiable_sections
test_inserts_prompt_proxy_auth_block_for_local_proxy
test_skips_git_proxy_block_for_non_http_remote_url
test_rewrites_local_kubernetes_targets_for_local_proxy
test_rewrites_local_compose_targets_for_local_proxy
test_rejects_proxy_enable_without_proxy_urls
test_simple_api_server_context_emits_prompt_auth_block
