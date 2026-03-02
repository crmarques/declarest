#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_args_lib() {
  unset E2E_EXPLICIT \
    E2E_RESOURCE_SERVER E2E_RESOURCE_SERVER_CONNECTION E2E_RESOURCE_SERVER_AUTH_TYPE E2E_RESOURCE_SERVER_MTLS \
    E2E_MANAGED_SERVER_PROXY E2E_MANAGED_SERVER_PROXY_HTTP_URL E2E_MANAGED_SERVER_PROXY_HTTPS_URL E2E_MANAGED_SERVER_PROXY_NO_PROXY \
    E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD \
    E2E_METADATA \
    E2E_REPO_TYPE E2E_GIT_PROVIDER E2E_GIT_PROVIDER_CONNECTION E2E_SECRET_PROVIDER E2E_SECRET_PROVIDER_CONNECTION \
    E2E_PROFILE E2E_PLATFORM E2E_LIST_COMPONENTS E2E_VALIDATE_COMPONENTS E2E_KEEP_RUNTIME E2E_VERBOSE E2E_CLEAN_RUN_ID E2E_CLEAN_ALL \
    E2E_SELECTED_BY_PROFILE_DEFAULT || true
  source_e2e_lib "common"
  source_e2e_lib "args"
}

test_parses_validate_components_flag() {
  reload_args_lib
  e2e_parse_args --validate-components
  assert_eq "${E2E_VALIDATE_COMPONENTS}" "1" "expected --validate-components to be parsed"
}

test_defaults_metadata_mode_to_bundle() {
  reload_args_lib
  e2e_parse_args
  assert_eq "${E2E_METADATA}" "bundle" "expected metadata mode default to bundle"
}

test_defaults_platform_to_kubernetes() {
  reload_args_lib
  e2e_parse_args
  assert_eq "${E2E_PLATFORM}" "kubernetes" "expected platform default to kubernetes"
}

test_parses_platform_flag() {
  reload_args_lib
  e2e_parse_args --platform compose
  assert_eq "${E2E_PLATFORM}" "compose" "expected --platform compose to be parsed"

  reload_args_lib
  e2e_parse_args --platform kubernetes
  assert_eq "${E2E_PLATFORM}" "kubernetes" "expected --platform kubernetes to be parsed"
}

test_rejects_invalid_platform_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --platform nope 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "invalid --platform value"
}

test_parses_managed_server_auth_type_flag() {
  reload_args_lib
  e2e_parse_args --managed-server-auth-type custom-header
  assert_eq "${E2E_RESOURCE_SERVER_AUTH_TYPE}" "custom-header" "expected auth-type to be parsed"
}

test_parses_managed_server_proxy_flag() {
  reload_args_lib
  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'
  e2e_parse_args --managed-server-proxy true
  assert_eq "${E2E_MANAGED_SERVER_PROXY}" "true" "expected managed-server proxy flag to be parsed"

  reload_args_lib
  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'
  e2e_parse_args --managed-server-proxy false
  assert_eq "${E2E_MANAGED_SERVER_PROXY}" "false" "expected managed-server proxy flag to parse explicit false"
}

test_parses_metadata_mode_flag() {
  reload_args_lib
  e2e_parse_args --metadata bundle
  assert_eq "${E2E_METADATA}" "bundle" "expected metadata mode to be parsed"
}

test_rejects_invalid_metadata_mode_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --metadata nope 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "invalid --metadata value"
}

test_rejects_legacy_resource_server_auth_flags() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --resource-server-oauth2 false 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "unknown argument: --resource-server-oauth2"
}

test_rejects_managed_server_none() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --managed-server none 2>&1)
  status=$?
  set -e

  [[ -z "${output}" ]] || true
  assert_status "${status}" "1"
  assert_contains "${output}" "--managed-server none is not supported"
}

test_rejects_managed_server_proxy_without_urls() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --managed-server-proxy true 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "--managed-server-proxy requires DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL and/or DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL"
}

test_cleanup_parser_treats_validate_mode_as_workload_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_cleanup_args --clean-all --validate-components 2>&1)
  status=$?
  set -e

  assert_status "${status}" "2"
  assert_contains "${output}" "cannot be combined with workload flags"
}

test_cleanup_parser_treats_metadata_flag_as_workload_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_cleanup_args --clean-all --metadata bundle 2>&1)
  status=$?
  set -e

  assert_status "${status}" "2"
  assert_contains "${output}" "cannot be combined with workload flags"
}

test_cleanup_parser_treats_managed_server_proxy_flag_as_workload_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_cleanup_args --clean-all --managed-server-proxy true 2>&1)
  status=$?
  set -e

  assert_status "${status}" "2"
  assert_contains "${output}" "cannot be combined with workload flags"
}

test_cleanup_parser_treats_platform_flag_as_workload_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_cleanup_args --clean run-123 --platform compose 2>&1)
  status=$?
  set -e

  assert_status "${status}" "2"
  assert_contains "${output}" "cannot be combined with workload flags"

  set +e
  output=$(e2e_parse_cleanup_args --clean-all --platform compose 2>&1)
  status=$?
  set -e

  assert_status "${status}" "2"
  assert_contains "${output}" "cannot be combined with workload flags"
}

test_usage_mentions_validate_flag_and_no_none_resource_server() {
  reload_args_lib
  local output
  output=$(e2e_usage)
  assert_contains "${output}" "--validate-components"
  assert_contains "${output}" "--platform <compose|kubernetes>"
  assert_contains "${output}" "--metadata <bundle|local-dir>"
  assert_contains "${output}" "--managed-server-auth-type <none|basic|oauth2|custom-header>"
  assert_contains "${output}" "--managed-server-proxy [<true|false>]"
  assert_contains "${output}" "--managed-server <simple-api-server|keycloak|rundeck|vault>"
  assert_not_contains "${output}" "--resource-server-basic-auth"
  assert_not_contains "${output}" "--resource-server-oauth2"
  assert_not_contains "${output}" "--managed-server <simple-api-server|keycloak|rundeck|vault|none>"
}

test_parses_validate_components_flag
test_defaults_metadata_mode_to_bundle
test_defaults_platform_to_kubernetes
test_parses_platform_flag
test_rejects_invalid_platform_flag
test_parses_managed_server_auth_type_flag
test_parses_managed_server_proxy_flag
test_parses_metadata_mode_flag
test_rejects_invalid_metadata_mode_flag
test_rejects_legacy_resource_server_auth_flags
test_rejects_managed_server_none
test_rejects_managed_server_proxy_without_urls
test_cleanup_parser_treats_validate_mode_as_workload_flag
test_cleanup_parser_treats_metadata_flag_as_workload_flag
test_cleanup_parser_treats_managed_server_proxy_flag_as_workload_flag
test_cleanup_parser_treats_platform_flag_as_workload_flag
test_usage_mentions_validate_flag_and_no_none_resource_server
