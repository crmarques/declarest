#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_parallel_runner_returns_success_when_all_commands_pass() {
  local tmp
  local matrix
  local output
  local status

  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  matrix="${tmp}/matrix.txt"

  cat >"${matrix}" <<'EOF'
printf 'job-one\n'
printf 'job-two\n'
EOF

  set +e
  output=$("${REPO_ROOT}/test/e2e/run-e2e-parallel.sh" --matrix-file "${matrix}" --log-dir "${tmp}/logs" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "0"
  assert_contains "${output}" "[PASS] job-01"
  assert_contains "${output}" "[PASS] job-02"
}

test_parallel_runner_returns_failure_when_any_command_fails() {
  local tmp
  local matrix
  local output
  local status

  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  matrix="${tmp}/matrix.txt"

  cat >"${matrix}" <<'EOF'
printf 'job-one\n'
false
EOF

  set +e
  output=$("${REPO_ROOT}/test/e2e/run-e2e-parallel.sh" --matrix-file "${matrix}" --log-dir "${tmp}/logs" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "[PASS] job-01"
  assert_contains "${output}" "[FAIL] job-02"
}

test_parallel_runner_returns_success_when_all_commands_pass
test_parallel_runner_returns_failure_when_any_command_fails
