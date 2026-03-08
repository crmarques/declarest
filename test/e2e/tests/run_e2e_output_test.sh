#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_run_e2e() {
  export E2E_SOURCE_ONLY=1
  # shellcheck disable=SC1091
  source "${REPO_ROOT}/test/e2e/run-e2e.sh"
}

test_startup_prints_execution_parameters_before_step_table() {
  load_run_e2e
  E2E_CLI_ARGS=(--list-components)
  E2E_EXECUTION_LOG='/tmp/test-e2e-execution.log'
  E2E_STEP_TABLE_HEADER_PRINTED=0

  local output
  output=$(
    printf 'E2E execution log: %s\n' "${E2E_EXECUTION_LOG}"
    e2e_print_startup_execution_parameters
    ui_print_step_table_header
  )

  assert_contains "${output}" "E2E execution log: /tmp/test-e2e-execution.log"
  assert_contains "${output}" "Execution Parameters"
  assert_contains "${output}" "STEP"

  local execution_log_line
  local execution_parameters_line
  local step_header_line
  execution_log_line=$(printf '%s\n' "${output}" | awk '/^E2E execution log:/ { print NR; exit }')
  execution_parameters_line=$(printf '%s\n' "${output}" | awk '/^Execution Parameters$/ { print NR; exit }')
  step_header_line=$(printf '%s\n' "${output}" | awk '/STEP/ { print NR; exit }')

  if [[ -z "${execution_log_line}" || -z "${execution_parameters_line}" || -z "${step_header_line}" ]]; then
    fail_test "expected execution log, execution parameters, and step table header"
  fi

  if ((execution_log_line >= execution_parameters_line || execution_parameters_line >= step_header_line)); then
    fail_test "expected Execution Parameters after execution log and before step table"
  fi
}

test_startup_prints_execution_parameters_before_step_table
