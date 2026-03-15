#!/usr/bin/env bash
set -euo pipefail

CASE_ID='proxy-managed-server-traffic'
CASE_SCOPE='main'
CASE_REQUIRES='proxy-mode=local managed-server=simple-api-server'

case_run() {
  local logical_path log_path
  logical_path=$(case_repo_template_first_resource_path 'simple-api-server') || return 1
  log_path="${E2E_RUN_DIR}/proxy/access.log"

  case_run_declarest resource get "${logical_path}" --source managed-server -o json
  case_expect_success || return 1

  case_wait_until 20 1 "proxy log contains managed-server path ${logical_path}" \
    grep -Fq -- "${logical_path}" "${log_path}"
}
