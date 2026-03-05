#!/usr/bin/env bash

CASE_ID='operator-reconcile-create-update'
CASE_SCOPE='operator-main'
CASE_REQUIRES='managed-server=simple-api-server repo-type=git'

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_DIR}/lib/profile.sh"
# shellcheck disable=SC1091
source "${E2E_DIR}/lib/operator.sh"

operator_wait_remote_owner() {
  local logical_path=$1
  local expected_owner=$2

  case_run_declarest resource get "${logical_path}" --source remote-server -o json
  ((CASE_LAST_STATUS == 0)) || return 1

  jq -e --arg owner "${expected_owner}" '.owner == $owner' <<<"${CASE_LAST_STDOUT}" >/dev/null
}

case_run() {
  local resource_path
  local resource_payload
  local create_file
  local update_file

  resource_path=$(e2e_operator_example_resource_path)
  resource_payload=$(e2e_operator_example_resource_payload)
  create_file="${E2E_CASE_TMP_DIR}/operator-create.json"
  update_file="${E2E_CASE_TMP_DIR}/operator-update.json"

  if [[ "${resource_path}" != '/api/projects/operator-demo' ]]; then
    printf 'unexpected operator demo path for simple-api-server: %s\n' "${resource_path}" >&2
    return 1
  fi

  jq -c '. + {"owner":"operator-e2e"}' <<<"${resource_payload}" >"${create_file}"
  case_run_declarest resource save "${resource_path}" -f "${create_file}" -i json --overwrite
  case_expect_success

  case_run_declarest repository commit -m 'operator e2e create'
  case_expect_success

  case_run_declarest repository push
  case_expect_success

  case_wait_until 180 3 "operator create sync for ${resource_path}" operator_wait_remote_owner "${resource_path}" 'operator-e2e'

  jq -c '. + {"owner":"operator-e2e-updated","displayName":"Operator Demo Updated"}' <"${create_file}" >"${update_file}"
  case_run_declarest resource save "${resource_path}" -f "${update_file}" -i json --overwrite
  case_expect_success

  case_run_declarest repository commit -m 'operator e2e update'
  case_expect_success

  case_run_declarest repository push
  case_expect_success

  case_wait_until 180 3 "operator update sync for ${resource_path}" operator_wait_remote_owner "${resource_path}" 'operator-e2e-updated'
}
