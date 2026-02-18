#!/usr/bin/env bash

CASE_ID='secret-detect-fix-validation'
CASE_SCOPE='corner'
CASE_REQUIRES='has-secret-provider'

case_run() {
  local payload_file="${E2E_CASE_TMP_DIR}/detect-payload.json"
  local repo_payload_file="${E2E_CASE_TMP_DIR}/repo-detect-payload.json"
  local repo_scope='/secret-detect-fix-validation'

  case_write_json "${payload_file}" '{"password": "pw-123", "apiToken": "token-123"}'

  case_run_declarest secret detect --fix -f "${payload_file}" -i json
  case_expect_failure
  case_expect_output_contains 'path is required'

  case_run_declarest secret detect /secret-detect-fix-validation/acme --fix --secret-attribute clientSecret -f "${payload_file}" -i json
  case_expect_failure
  case_expect_output_contains '--secret-attribute'

  case_write_json "${repo_payload_file}" '{"password":"pw-123"}'
  case_run_declarest resource save "${repo_scope}/acme" --ignore -f "${repo_payload_file}" -i json
  case_expect_success

  case_run_declarest secret detect "${repo_scope}" --secret-attribute clientSecret -o json
  case_expect_failure
  case_expect_output_contains '--secret-attribute'
}
