#!/usr/bin/env bash

CASE_ID='path-traversal-rejection'
CASE_SCOPE='corner'
CASE_REQUIRES=''

case_run() {
  local payload_file="${E2E_CASE_TMP_DIR}/payload.json"

  case_write_json "${payload_file}" '{"id": "x"}'

  case_run_declarest resource save /customers/../passwd -f "${payload_file}" -i json
  case_expect_failure
  case_expect_output_contains 'logical path must not contain traversal segments'
}
