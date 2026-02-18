#!/usr/bin/env bash

CASE_ID='secret-mask-resolve'
CASE_SCOPE='main'
CASE_REQUIRES='has-secret-provider'

case_run() {
  local payload_file="${E2E_CASE_TMP_DIR}/secret-payload.json"

  case_run_declarest secret init
  case_expect_success

  case_run_declarest secret store apiToken super-secret
  case_expect_success

  case_write_json "${payload_file}" '{"apiToken": "super-secret", "name": "acme"}'

  case_run_declarest secret mask -f "${payload_file}" -i json -o json
  case_expect_success
  case_expect_output_contains '{{secret .}}'

  case_write_json "${payload_file}" '{"apiToken": "{{secret .}}", "name": "acme"}'

  case_run_declarest secret resolve -f "${payload_file}" -i json -o json
  case_expect_success
  case_expect_output_contains 'super-secret'
}
