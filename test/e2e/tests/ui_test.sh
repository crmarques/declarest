#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_ui_libs() {
  unset DECLAREST_E2E_CONTAINER_ENGINE DECLAREST_E2E_EXECUTION_LOG \
    DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL DECLAREST_E2E_MANAGED_SERVER_PROXY_NO_PROXY \
    DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME DECLAREST_E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD || true
  unset E2E_EXPLICIT \
    E2E_MANAGED_SERVER E2E_MANAGED_SERVER_CONNECTION E2E_MANAGED_SERVER_AUTH_TYPE E2E_MANAGED_SERVER_MTLS \
    E2E_MANAGED_SERVER_PROXY E2E_MANAGED_SERVER_PROXY_AUTH_TYPE E2E_MANAGED_SERVER_PROXY_HTTP_URL E2E_MANAGED_SERVER_PROXY_HTTPS_URL E2E_MANAGED_SERVER_PROXY_NO_PROXY \
    E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD \
    E2E_METADATA \
    E2E_REPO_TYPE E2E_GIT_PROVIDER E2E_GIT_PROVIDER_CONNECTION E2E_SECRET_PROVIDER E2E_SECRET_PROVIDER_CONNECTION \
    E2E_PROFILE E2E_PLATFORM E2E_LIST_COMPONENTS E2E_VALIDATE_COMPONENTS E2E_KEEP_RUNTIME E2E_VERBOSE E2E_CLEAN_RUN_ID E2E_CLEAN_ALL \
    E2E_SELECTED_BY_PROFILE_DEFAULT || true
  source_e2e_lib "common"
  source_e2e_lib "args"
  source_e2e_lib "profile"
  source_e2e_lib "ui"
}

test_step_table_header_format() {
  load_ui_libs
  E2E_STEP_TABLE_HEADER_PRINTED=0
  local output
  output=$(ui_print_step_table_header)
  assert_contains "${output}" "STEP"
  assert_contains "${output}" "ACTION"
  assert_contains "${output}" "SPAN"
  assert_contains "${output}" "STATUS"
}

test_step_state_labels_match_contract() {
  load_ui_libs
  assert_contains "$(ui_step_state_label RUNNING)" "[RUNNING]"
  assert_contains "$(ui_step_state_label OK)" "[OK]"
  assert_contains "$(ui_step_state_label FAIL)" "[FAILED]"
  assert_contains "$(ui_step_state_label SKIP)" "[SKIP]"
}

test_running_step_line_shows_elapsed_span() {
  load_ui_libs
  local output
  output=$(ui_print_step_line 1 7 'Initializing' 'RUNNING' 61)
  assert_contains "${output}" "1m01s"
  assert_contains "${output}" "[RUNNING]"
}

test_summary_includes_required_fields() {
  load_ui_libs
  E2E_START_EPOCH=$(e2e_epoch_now)
  E2E_STEPS_TOTAL=2
  E2E_STEP_TITLES[1]='Initializing'
  E2E_STEP_TITLES[2]='Finalizing'
  E2E_STEP_STATUSES[1]='OK'
  E2E_STEP_STATUSES[2]='SKIP'
  E2E_STEP_DURATIONS[1]=0
  E2E_STEP_DURATIONS[2]=0
  E2E_CASE_TOTAL=3
  E2E_CASE_PASSED=2
  E2E_CASE_FAILED=1
  E2E_CASE_SKIPPED=0
  E2E_CONTEXT_FILE='/tmp/test-contexts.yaml'
  E2E_LOG_DIR='/tmp/test-logs'
  E2E_EXECUTION_LOG='/tmp/test-execution.log'
  E2E_STEP_LAST_LOG='/tmp/test-step.log'

  local output
  output=$(ui_print_summary)
  assert_contains "${output}" "E2E Summary"
  assert_contains "${output}" "Execution Parameters"
  assert_contains "${output}" "metadata-source: bundle (default)"
  assert_contains "${output}" "profile: cli-basic (default)"
  assert_contains "${output}" "repository-type: filesystem (default)"
  assert_contains "${output}" "container-engine: podman (default)"
  assert_contains "${output}" "cases total=3 passed=2 failed=1 skipped=0"
  assert_contains "${output}" "context:  /tmp/test-contexts.yaml"
  assert_contains "${output}" "logs:     /tmp/test-logs"
  assert_contains "${output}" "execution-log: /tmp/test-execution.log"
  assert_contains "${output}" "last-fail-log: /tmp/test-step.log"
}

test_summary_marks_explicit_and_component_default_parameters() {
  load_ui_libs
  E2E_START_EPOCH=$(e2e_epoch_now)
  E2E_STEPS_TOTAL=1
  E2E_STEP_TITLES[1]='Initializing'
  E2E_STEP_STATUSES[1]='OK'
  E2E_STEP_DURATIONS[1]=0

  e2e_parse_args --managed-server rundeck --repo-type git --git-provider gitlab
  E2E_MANAGED_SERVER_AUTH_TYPE='custom-header'

  local output
  output=$(ui_print_summary)

  assert_contains "${output}" "managed-server: rundeck (explicit)"
  assert_contains "${output}" "repository-type: git (explicit)"
  assert_contains "${output}" "git-provider: gitlab (explicit)"
  assert_contains "${output}" "managed-server-auth-type: custom-header (component-default)"
}

test_summary_marks_operator_profile_defaults() {
  load_ui_libs
  E2E_START_EPOCH=$(e2e_epoch_now)
  E2E_STEPS_TOTAL=1
  E2E_STEP_TITLES[1]='Initializing'
  E2E_STEP_STATUSES[1]='OK'
  E2E_STEP_DURATIONS[1]=0

  e2e_parse_args --profile operator-manual
  e2e_apply_profile_defaults
  E2E_MANAGED_SERVER_AUTH_TYPE='oauth2'

  local output
  output=$(ui_print_summary)

  assert_contains "${output}" "repository-type: git (profile-default)"
  assert_contains "${output}" "git-provider: gitea (profile-default)"
}

test_summary_marks_explicit_proxy_prompt_auth_type() {
  load_ui_libs
  E2E_START_EPOCH=$(e2e_epoch_now)
  E2E_STEPS_TOTAL=1
  E2E_STEP_TITLES[1]='Initializing'
  E2E_STEP_STATUSES[1]='OK'
  E2E_STEP_DURATIONS[1]=0

  E2E_MANAGED_SERVER_PROXY_HTTP_URL='http://proxy.example.com:3128'
  e2e_parse_args --managed-server-proxy true --managed-server-proxy-auth-type prompt

  local output
  output=$(ui_print_summary)

  assert_contains "${output}" "managed-server-proxy-auth-type: prompt (explicit)"
}

test_step_table_header_format
test_step_state_labels_match_contract
test_running_step_line_shows_elapsed_span
test_summary_includes_required_fields
test_summary_marks_explicit_and_component_default_parameters
test_summary_marks_operator_profile_defaults
test_summary_marks_explicit_proxy_prompt_auth_type
