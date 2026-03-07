#!/usr/bin/env bash

CASE_ID='operator-reconcile-webhook'
CASE_SCOPE='operator-main'
CASE_REQUIRES='managed-server=simple-api-server repo-type=git git-provider=gitea'

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_DIR}/lib/operator.sh"

operator_wait_remote_owner() {
  local logical_path=$1
  local expected_owner=$2

  case_run_declarest resource get "${logical_path}" --source managed-server -o json
  ((CASE_LAST_STATUS == 0)) || return 1

  jq -e --arg owner "${expected_owner}" '.owner == $owner' <<<"${CASE_LAST_STDOUT}" >/dev/null
}

case_run() {
  local resource_path='/api/projects/operator-webhook'
  local create_file="${E2E_CASE_TMP_DIR}/operator-webhook-create.json"
  local update_file="${E2E_CASE_TMP_DIR}/operator-webhook-update.json"

  cat >"${create_file}" <<'EOF'
{"id":"operator-webhook","name":"operator-webhook","displayName":"Operator Webhook","owner":"operator-webhook-initial"}
EOF
  case_run_declarest resource save "${resource_path}" -f "${create_file}" -i json --overwrite
  case_expect_success

  case_run_declarest repository commit -m 'operator webhook create'
  case_expect_success

  case_run_declarest repository push
  case_expect_success

  case_wait_until 180 3 "operator webhook create sync for ${resource_path}" operator_wait_remote_owner "${resource_path}" 'operator-webhook-initial'

  jq -c '.owner = "operator-webhook-updated" | .displayName = "Operator Webhook Updated"' <"${create_file}" >"${update_file}"
  case_run_declarest resource save "${resource_path}" -f "${update_file}" -i json --overwrite
  case_expect_success

  case_run_declarest repository commit -m 'operator webhook update'
  case_expect_success

  case_run_declarest repository push
  case_expect_success

  # ResourceRepository poll interval is 30s; webhook path should reconcile quicker than that.
  case_wait_until 25 2 "operator webhook update sync for ${resource_path}" operator_wait_remote_owner "${resource_path}" 'operator-webhook-updated'
}
