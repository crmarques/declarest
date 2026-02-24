#!/usr/bin/env bash

CASE_ID='list-delete-recursion'
CASE_SCOPE='corner'
CASE_REQUIRES=''

case_run() {
  local payload_direct="${E2E_CASE_TMP_DIR}/direct.json"
  local payload_nested="${E2E_CASE_TMP_DIR}/nested.json"

  case_write_json "${payload_direct}" '{"id": "direct-item"}'
  case_write_json "${payload_nested}" '{"id": "nested-item"}'

  case_run_declarest repo init
  case_expect_success

  case_run_declarest resource save /customers-recursion/acme -f "${payload_direct}" -i json
  case_expect_success

  case_run_declarest resource save /customers-recursion/east/zen -f "${payload_nested}" -i json
  case_expect_success

  case_run_declarest resource list /customers-recursion --repository -o json
  case_expect_success
  case_expect_output_contains '/customers-recursion/acme'

  case_run_declarest resource delete /customers-recursion -y
  case_expect_success

  case_run_declarest resource list /customers-recursion --repository -r -o json
  case_expect_success
  case_expect_output_contains '/customers-recursion/east/zen'

  case_run_declarest resource delete /customers-recursion -y -r
  case_expect_success

  case_run_declarest resource list /customers-recursion --repository -r -o json
  case_expect_success
  case_expect_output_not_contains '/customers-recursion/east/zen'
}
