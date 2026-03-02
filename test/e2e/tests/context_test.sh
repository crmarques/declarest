#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_context_libs() {
  unset \
    E2E_MANAGED_SERVER_PROXY \
    E2E_MANAGED_SERVER_PROXY_HTTP_URL \
    E2E_MANAGED_SERVER_PROXY_HTTPS_URL \
    E2E_MANAGED_SERVER_PROXY_NO_PROXY \
    E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME \
    E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD \
    E2E_RESOURCE_SERVER || true

  source_e2e_lib "common"
  source_e2e_lib "context"
}

write_context_fixture() {
  local path=$1
  cat >"${path}" <<'EOF'
contexts:
  - name: e2e-basic
    resource-server:
      http:
        base-url: http://127.0.0.1:8080
        auth:
          bearer-token:
            token: token-dev
current-ctx: e2e-basic
EOF
}

test_inserts_proxy_block_when_managed_server_proxy_enabled() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_RESOURCE_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_PROXY='true'
  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'
  E2E_MANAGED_SERVER_PROXY_HTTPS_URL='https://proxy.example.com:3128'
  E2E_MANAGED_SERVER_PROXY_NO_PROXY='localhost,127.0.0.1'
  E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME='proxy-user'
  E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD='proxy-pass'

  e2e_context_insert_resource_server_proxy "${context_file}"

  assert_file_contains "${context_file}" "proxy:"
  assert_file_contains "${context_file}" "http-url: 'http://proxy.example.com:3128'"
  assert_file_contains "${context_file}" "https-url: 'https://proxy.example.com:3128'"
  assert_file_contains "${context_file}" "no-proxy: 'localhost,127.0.0.1'"
  assert_file_contains "${context_file}" "username: 'proxy-user'"
  assert_file_contains "${context_file}" "password: 'proxy-pass'"
}

test_skips_proxy_block_when_managed_server_proxy_disabled() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_RESOURCE_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_PROXY='false'
  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'

  e2e_context_insert_resource_server_proxy "${context_file}"

  assert_not_contains "$(cat "${context_file}")" "proxy:"
}

test_rejects_proxy_enable_without_proxy_urls() {
  reload_context_libs
  local tmp
  tmp=$(new_temp_dir)
  local context_file="${tmp}/contexts.yaml"
  write_context_fixture "${context_file}"

  E2E_RESOURCE_SERVER='simple-api-server'
  E2E_MANAGED_SERVER_PROXY='true'

  local output status
  set +e
  output=$(e2e_context_insert_resource_server_proxy "${context_file}" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "--managed-server-proxy requires DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL and/or DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL"
}

test_inserts_proxy_block_when_managed_server_proxy_enabled
test_skips_proxy_block_when_managed_server_proxy_disabled
test_rejects_proxy_enable_without_proxy_urls
