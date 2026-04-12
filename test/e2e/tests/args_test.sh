#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_args_lib() {
  unset E2E_EXPLICIT \
    E2E_MANAGED_SERVICE E2E_MANAGED_SERVICE_CONNECTION E2E_MANAGED_SERVICE_AUTH_TYPE E2E_MANAGED_SERVICE_MTLS \
    E2E_PROXY_MODE E2E_PROXY_AUTH_TYPE E2E_PROXY_HTTP_URL E2E_PROXY_HTTPS_URL E2E_PROXY_NO_PROXY \
    E2E_PROXY_AUTH_USERNAME E2E_PROXY_AUTH_PASSWORD \
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

test_defaults_metadata_source_to_bundle() {
  reload_args_lib
  e2e_parse_args
  assert_eq "${E2E_METADATA}" "bundle" "expected metadata source default to bundle"
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

test_parses_operator_profile() {
  reload_args_lib
  e2e_parse_args --profile operator-manual
  assert_eq "${E2E_PROFILE}" "operator-manual" "expected --profile operator-manual to be parsed"
}

test_parses_operator_automated_profile() {
  reload_args_lib
  e2e_parse_args --profile operator-basic
  assert_eq "${E2E_PROFILE}" "operator-basic" "expected --profile operator-basic to be parsed"
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

test_parses_managed_service_auth_type_flag() {
  reload_args_lib
  e2e_parse_args --managed-service-auth-type custom-header
  assert_eq "${E2E_MANAGED_SERVICE_AUTH_TYPE}" "custom-header" "expected auth-type to be parsed"
}

test_parses_managed_service_auth_type_prompt_flag() {
  reload_args_lib
  e2e_parse_args --managed-service-auth-type prompt
  assert_eq "${E2E_MANAGED_SERVICE_AUTH_TYPE}" "prompt" "expected prompt auth-type to be parsed"
}

test_parses_proxy_mode_flag() {
  reload_args_lib
  e2e_parse_args --proxy-mode local
  assert_eq "${E2E_PROXY_MODE}" "local" "expected --proxy-mode local to be parsed"
  assert_eq "$(e2e_effective_proxy_auth_type)" "basic" "expected local proxy mode to default auth-type to basic"
}

test_parses_proxy_auth_type_flag() {
  reload_args_lib
  E2E_PROXY_HTTP_URL='http://proxy.example.com:3128'
  e2e_parse_args --proxy-mode external --proxy-auth-type prompt
  assert_eq "${E2E_PROXY_AUTH_TYPE}" "prompt" "expected --proxy-auth-type prompt to be parsed"
}

test_parses_metadata_source_flag() {
  reload_args_lib
  e2e_parse_args --metadata-source bundle
  assert_eq "${E2E_METADATA}" "bundle" "expected metadata source to be parsed"
}

test_parses_metadata_source_dir_flag() {
  reload_args_lib
  e2e_parse_args --metadata-source dir
  assert_eq "${E2E_METADATA}" "dir" "expected metadata source dir to be parsed"
}

test_rejects_invalid_metadata_source_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --metadata-source nope 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "invalid --metadata-source value"
}

test_rejects_unknown_argument() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --unknown-flag 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "unknown argument: --unknown-flag"
}

test_rejects_managed_service_none() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --managed-service none 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "--managed-service none is not supported"
}

test_rejects_proxy_mode_external_without_urls() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --proxy-mode external 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "--proxy-mode external requires DECLAREST_E2E_PROXY_HTTP_URL and/or DECLAREST_E2E_PROXY_HTTPS_URL"
}

test_rejects_proxy_auth_type_without_proxy() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_args --proxy-auth-type prompt 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "--proxy-auth-type requires --proxy-mode local or external"
}

test_rejects_proxy_auth_type_basic_without_credentials() {
  reload_args_lib
  local output status
  E2E_PROXY_HTTP_URL='http://proxy.example.com:3128'
  set +e
  output=$(e2e_parse_args --proxy-mode external --proxy-auth-type basic 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "proxy auth-type basic requires DECLAREST_E2E_PROXY_AUTH_USERNAME and DECLAREST_E2E_PROXY_AUTH_PASSWORD"
}

test_rejects_proxy_auth_type_prompt_with_inline_credentials() {
  reload_args_lib
  local output status
  E2E_PROXY_HTTP_URL='http://proxy.example.com:3128'
  E2E_PROXY_AUTH_USERNAME='proxy-user'
  E2E_PROXY_AUTH_PASSWORD='proxy-pass'
  set +e
  output=$(e2e_parse_args --proxy-mode external --proxy-auth-type prompt 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "proxy auth-type prompt cannot be combined with DECLAREST_E2E_PROXY_AUTH_USERNAME or DECLAREST_E2E_PROXY_AUTH_PASSWORD"
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

test_cleanup_parser_treats_metadata_source_flag_as_workload_flag() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_cleanup_args --clean-all --metadata-source bundle 2>&1)
  status=$?
  set -e

  assert_status "${status}" "2"
  assert_contains "${output}" "cannot be combined with workload flags"
}

test_cleanup_parser_treats_proxy_mode_and_auth_flags_as_workload_flags() {
  reload_args_lib
  local output status
  set +e
  output=$(e2e_parse_cleanup_args --clean-all --proxy-mode local 2>&1)
  status=$?
  set -e

  assert_status "${status}" "2"
  assert_contains "${output}" "cannot be combined with workload flags"

  set +e
  output=$(e2e_parse_cleanup_args --clean-all --proxy-auth-type prompt 2>&1)
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

test_usage_mentions_validate_flag_and_canonical_options() {
  reload_args_lib
  local output
  output=$(e2e_usage)
  assert_contains "${output}" "--profile <cli-basic|cli-full|cli-manual|operator-manual|operator-basic|operator-full>"
  assert_contains "${output}" "--validate-components"
  assert_contains "${output}" "--platform <compose|kubernetes>"
  assert_contains "${output}" "--metadata-source <bundle|dir>"
  assert_contains "${output}" "--managed-service-auth-type <none|basic|oauth2|custom-header|prompt>"
  assert_contains "${output}" "--proxy-mode <none|local|external>"
  assert_contains "${output}" "--proxy-auth-type <none|basic|prompt>"
  assert_contains "${output}" "--managed-service <name>"
  assert_contains "${output}" "--repo-type <name>"
  assert_contains "${output}" "--git-provider <name>"
  assert_contains "${output}" "--secret-provider <name|none>"
  assert_contains "${output}" "Use --list-components to inspect available managed-service components"
  assert_contains "${output}" "DECLAREST_E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS=<seconds>"
  assert_contains "${output}" "DECLAREST_E2E_OPERATOR_READY_TIMEOUT_SECONDS=<seconds>"
  assert_not_contains "${output}" "--managed-service-basic-auth"
  assert_not_contains "${output}" "--managed-service-oauth2"
  assert_not_contains "${output}" "--metadata <bundle|local-dir>"
  assert_not_contains "${output}" "--managed-service <name|none>"
}

test_parses_validate_components_flag
test_defaults_metadata_source_to_bundle
test_defaults_platform_to_kubernetes
test_parses_platform_flag
test_parses_operator_profile
test_parses_operator_automated_profile
test_rejects_invalid_platform_flag
test_parses_managed_service_auth_type_flag
test_parses_managed_service_auth_type_prompt_flag
test_parses_proxy_mode_flag
test_parses_proxy_auth_type_flag
test_parses_metadata_source_flag
test_parses_metadata_source_dir_flag
test_rejects_invalid_metadata_source_flag
test_rejects_unknown_argument
test_rejects_managed_service_none
test_rejects_proxy_mode_external_without_urls
test_rejects_proxy_auth_type_without_proxy
test_rejects_proxy_auth_type_basic_without_credentials
test_rejects_proxy_auth_type_prompt_with_inline_credentials
test_cleanup_parser_treats_validate_mode_as_workload_flag
test_cleanup_parser_treats_metadata_source_flag_as_workload_flag
test_cleanup_parser_treats_proxy_mode_and_auth_flags_as_workload_flags
test_cleanup_parser_treats_platform_flag_as_workload_flag
test_usage_mentions_validate_flag_and_canonical_options
