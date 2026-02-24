#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_ui_libs() {
  source_e2e_lib "common"
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
  assert_contains "${output}" "cases total=3 passed=2 failed=1 skipped=0"
  assert_contains "${output}" "context:  /tmp/test-contexts.yaml"
  assert_contains "${output}" "logs:     /tmp/test-logs"
  assert_contains "${output}" "execution-log: /tmp/test-execution.log"
  assert_contains "${output}" "last-fail-log: /tmp/test-step.log"
}

test_step_table_header_format
test_step_state_labels_match_contract
test_summary_includes_required_fields
