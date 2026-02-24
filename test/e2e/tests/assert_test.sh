#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_case_wait_until_succeeds_after_retries() {
  source_e2e_lib "assert"

  local attempts=0
  eventually_succeeds() {
    attempts=$((attempts + 1))
    if ((attempts < 3)); then
      printf 'attempt=%d not ready yet\n' "${attempts}" >&2
      return 1
    fi
    printf 'attempt=%d ready\n' "${attempts}"
    return 0
  }

  case_wait_until 2 0 'eventual success in assert test' eventually_succeeds
  assert_eq "${attempts}" "3"
}

test_case_wait_until_times_out_with_diagnostics() {
  source_e2e_lib "assert"

  always_fails() {
    printf 'still failing\n' >&2
    return 1
  }

  local output status
  set +e
  output=$(case_wait_until 1 0 'forced timeout in assert test' always_fails 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" 'timed out waiting for forced timeout in assert test'
  assert_contains "${output}" 'last attempt output:'
  assert_contains "${output}" 'still failing'
}

test_case_wait_until_succeeds_after_retries
test_case_wait_until_times_out_with_diagnostics
