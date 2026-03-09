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

test_case_repo_template_metadata_input_format_uses_file_extension() {
  source_e2e_lib "assert"

  assert_eq "$(case_repo_template_metadata_input_format '/tmp/example.json')" "json"
  assert_eq "$(case_repo_template_metadata_input_format '/tmp/example.yaml')" "yaml"
  assert_eq "$(case_repo_template_metadata_input_format '/tmp/example.yml')" "yaml"
}

test_case_expect_sorted_resource_list_payloads_accepts_client_id_ordering() {
  source_e2e_lib "assert"

  local payload='[{"clientId":"account"},{"clientId":"billing"}]'
  case_expect_sorted_resource_list_payloads "${payload}"
}

test_case_repo_template_write_update_payload_updates_client_id_payloads() {
  source_e2e_lib "assert"

  local tmp source_file target_file rendered
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  source_file="${tmp}/source.json"
  target_file="${tmp}/target.json"

  cat >"${source_file}" <<'EOF'
{"clientId":"declarest-cli"}
EOF

  case_repo_template_write_update_payload "${source_file}" "${target_file}" "rev-1"
  rendered=$(<"${target_file}")

  assert_contains "${rendered}" '"description": "rev-1"'
}

test_case_wait_until_succeeds_after_retries
test_case_wait_until_times_out_with_diagnostics
test_case_repo_template_metadata_input_format_uses_file_extension
test_case_expect_sorted_resource_list_payloads_accepts_client_id_ordering
test_case_repo_template_write_update_payload_updates_client_id_payloads
