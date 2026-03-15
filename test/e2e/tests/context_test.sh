#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_context_libs() {
  unset \
    E2E_MANAGED_SERVER_PROXY \
    E2E_MANAGED_SERVER_PROXY_AUTH_TYPE \
    E2E_MANAGED_SERVER_PROXY_HTTP_URL \
    E2E_MANAGED_SERVER_PROXY_HTTPS_URL \
    E2E_MANAGED_SERVER_PROXY_NO_PROXY \
    E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME \
    E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD \
    E2E_MANAGED_SERVER_AUTH_TYPE \
    E2E_MANAGED_SERVER || true

  source_e2e_lib "common"
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
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: token-dev
currentContext: e2e-basic
EOF
}

test_inserts_proxy_block_when_managed_server_proxy_enabled() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_MANAGED_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_PROXY='true'
  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'
  E2E_MANAGED_SERVER_PROXY_HTTPS_URL='https://proxy.example.com:3128'
  E2E_MANAGED_SERVER_PROXY_NO_PROXY='localhost,127.0.0.1'
  E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME='proxy-user'
  E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD='proxy-pass'

  e2e_context_insert_managed_server_proxy "${context_file}"

  assert_file_contains "${context_file}" "proxy:"
  assert_file_contains "${context_file}" "httpURL: 'http://proxy.example.com:3128'"
  assert_file_contains "${context_file}" "httpsURL: 'https://proxy.example.com:3128'"
  assert_file_contains "${context_file}" "noProxy: 'localhost,127.0.0.1'"
  assert_file_contains "${context_file}" "username: 'proxy-user'"
  assert_file_contains "${context_file}" "password: 'proxy-pass'"
}

test_inserts_prompt_proxy_auth_block_when_requested() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_MANAGED_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_PROXY='true'
  E2E_MANAGED_SERVER_PROXY_AUTH_TYPE='prompt'
  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'

  e2e_context_insert_managed_server_proxy "${context_file}"

  assert_file_contains "${context_file}" "proxy:"
  assert_file_contains "${context_file}" "httpURL: 'http://proxy.example.com:3128'"
  assert_file_contains "${context_file}" "prompt: {}"
  assert_not_contains "$(cat "${context_file}")" "username: '"
  assert_not_contains "$(cat "${context_file}")" "password: '"
}

test_skips_proxy_block_when_managed_server_proxy_disabled() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_MANAGED_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_PROXY='false'
  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'

  e2e_context_insert_managed_server_proxy "${context_file}"

  assert_not_contains "$(cat "${context_file}")" "proxy:"
}

test_rejects_proxy_enable_without_proxy_urls() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_MANAGED_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_PROXY='true'

  local output status
  set +e
  output=$(e2e_context_insert_managed_server_proxy "${context_file}" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "--managedServer-proxy requires DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL and/or DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL"
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

test_inserts_proxy_block_when_managed_server_proxy_enabled
test_inserts_prompt_proxy_auth_block_when_requested
test_skips_proxy_block_when_managed_server_proxy_disabled
test_rejects_proxy_enable_without_proxy_urls
test_simple_api_server_context_emits_prompt_auth_block
