#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_args_lib() {
  unset E2E_EXPLICIT \
    E2E_RESOURCE_SERVER E2E_RESOURCE_SERVER_CONNECTION E2E_RESOURCE_SERVER_BASIC_AUTH E2E_RESOURCE_SERVER_OAUTH2 E2E_RESOURCE_SERVER_MTLS \
    E2E_REPO_TYPE E2E_GIT_PROVIDER E2E_GIT_PROVIDER_CONNECTION E2E_SECRET_PROVIDER E2E_SECRET_PROVIDER_CONNECTION \
    E2E_PROFILE E2E_LIST_COMPONENTS E2E_VALIDATE_COMPONENTS E2E_KEEP_RUNTIME E2E_VERBOSE E2E_CLEAN_RUN_ID E2E_CLEAN_ALL \
    E2E_SELECTED_BY_PROFILE_DEFAULT || true
  source_e2e_lib "common"
  source_e2e_lib "args"
}

test_parses_validate_components_flag() {
  reload_args_lib
  e2e_parse_args --validate-components
  assert_eq "${E2E_VALIDATE_COMPONENTS}" "1" "expected --validate-components to be parsed"
}

test_rejects_resource_server_none() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --resource-server none 2>&1)
  status=$?
  set -e

  [[ -z "${output}" ]] || true
  assert_status "${status}" "1"
  assert_contains "${output}" "--resource-server none is not supported"
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

test_usage_mentions_validate_flag_and_no_none_resource_server() {
  reload_args_lib
  local output
  output=$(e2e_usage)
  assert_contains "${output}" "--validate-components"
  assert_not_contains "${output}" "--resource-server <simple-api-server|keycloak|rundeck|vault|none>"
}

test_parses_validate_components_flag
test_rejects_resource_server_none
test_cleanup_parser_treats_validate_mode_as_workload_flag
test_usage_mentions_validate_flag_and_no_none_resource_server
