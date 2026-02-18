#!/usr/bin/env bash

CASE_ID='git-push-status'
CASE_SCOPE='main'
CASE_REQUIRES='repo-type=git'

case_run() {
  local payload_file="${E2E_CASE_TMP_DIR}/payload.json"

  case_write_json "${payload_file}" '{"id": "git-main", "name": "Git Main"}'

  case_run_declarest repo init
  case_expect_success

  case_run_declarest resource save /git-main/check -f "${payload_file}" -i json
  case_expect_success

  case_run_declarest repo push
  case_expect_success

  case_run_declarest repo status -o json
  case_expect_success
  case_expect_output_contains '"state"'
}
