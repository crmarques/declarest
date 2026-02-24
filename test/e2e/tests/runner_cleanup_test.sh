#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_cleanup_libs() {
  source_e2e_lib "common"
  source_e2e_lib "runner_cleanup"
}

test_cleanup_run_id_validation() {
  load_cleanup_libs

  e2e_validate_cleanup_run_id "20260223-090000-12345"

  local output status
  set +e
  output=$(e2e_validate_cleanup_run_id "../bad" 2>&1)
  status=$?
  set -e
  assert_status "${status}" "1"
  assert_contains "${output}" "invalid cleanup run-id"
}

test_runner_cmdline_and_env_parsers_support_fake_proc_root() {
  load_cleanup_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  mkdir -p "${tmp}/1234"
  printf 'bash\0./test/e2e/run-e2e.sh\0--profile\0basic\0' >"${tmp}/1234/cmdline"
  printf 'USER=test\0E2E_RUNNER_PID=1234\0E2E_RUN_ID=test-run\0' >"${tmp}/1234/environ"

  E2E_PROC_ROOT="${tmp}"
  e2e_runner_cmdline_matches 1234
  e2e_runner_pid_marker_matches 1234
  e2e_runner_pid_matches_run_id 1234 test-run

  local output status
  set +e
  output=$(e2e_runner_pid_matches_run_id 1234 other-run 2>&1)
  status=$?
  set -e
  assert_status "${status}" "1"
  [[ -z "${output}" ]] || true
}

test_remove_run_bin_entry_from_path() {
  load_cleanup_libs

  local original_path="${PATH}"
  local run_id='cleanup-path-test'
  local run_bin="${E2E_RUNS_DIR}/${run_id}/bin"

  PATH="${run_bin}:${original_path}"
  e2e_remove_run_bin_from_path "${run_id}"
  assert_eq "${PATH}" "${original_path}"

  PATH=":${run_bin}:${original_path}"
  e2e_remove_run_bin_from_path "${run_id}"
  assert_eq "${PATH}" ":${original_path}"

  PATH="${original_path}"
}

test_cleanup_run_id_validation
test_runner_cmdline_and_env_parsers_support_fake_proc_root
test_remove_run_bin_entry_from_path
