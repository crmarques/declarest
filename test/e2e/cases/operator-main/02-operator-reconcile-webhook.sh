#!/usr/bin/env bash

CASE_ID='operator-reconcile-webhook'
CASE_SCOPE='operator-main'
CASE_REQUIRES='has-managed-service repo-type=git'

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_DIR}/lib/operator.sh"

operator_wait_remote_available() {
  local logical_path=$1

  case_run_declarest resource get "${logical_path}" --source managed-service -o json
  ((CASE_LAST_STATUS == 0))
}

operator_wait_remote_update_marker() {
  local logical_path=$1
  local marker=$2

  case_run_declarest resource get "${logical_path}" --source managed-service -o json
  ((CASE_LAST_STATUS == 0)) || return 1

  e2e_operator_example_resource_output_has_update_marker "${CASE_LAST_STDOUT}" "${marker}"
}

case_run() {
  local base_resource_path
  local base_payload
  local resource_path
  local resource_payload
  local create_file="${E2E_CASE_TMP_DIR}/operator-webhook-create.json"
  local update_file="${E2E_CASE_TMP_DIR}/operator-webhook-update.json"
  local update_marker

  base_resource_path=$(e2e_operator_example_resource_path)
  base_payload=$(e2e_operator_example_resource_payload)
  resource_path=$(e2e_operator_example_resource_path_for_name "${base_resource_path}" 'operator-webhook')
  resource_payload=$(e2e_operator_example_resource_payload_for_name "${base_payload}" 'operator-webhook')
  update_marker="operator-webhook-updated-${E2E_MANAGED_SERVICE}"

  printf '%s\n' "${resource_payload}" >"${create_file}"
  case_run_declarest resource save "${resource_path}" -f "${create_file}" -i json --force
  case_expect_success

  case_run_declarest repository push
  case_expect_success

  case_wait_until 180 3 "operator webhook create sync for ${resource_path}" operator_wait_remote_available "${resource_path}"

  e2e_operator_example_resource_payload_with_update_marker "$(cat "${create_file}")" "${update_marker}" >"${update_file}"
  case_run_declarest resource save "${resource_path}" -f "${update_file}" -i json --force
  case_expect_success

  case_run_declarest repository push
  case_expect_success

  case_wait_until 90 2 "operator webhook update sync for ${resource_path}" operator_wait_remote_update_marker "${resource_path}" "${update_marker}"
}
