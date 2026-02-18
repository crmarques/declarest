#!/usr/bin/env bash

CASE_ID='save-secret-plaintext-guard'
CASE_SCOPE='corner'
CASE_REQUIRES=''

case_run() {
  local plaintext_payload_file="${E2E_CASE_TMP_DIR}/plaintext.json"
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata.json"
  local metadata_plaintext_payload_file="${E2E_CASE_TMP_DIR}/metadata-plaintext.json"
  local metadata_placeholder_payload_file="${E2E_CASE_TMP_DIR}/metadata-placeholder.json"

  local heuristic_path='/save-secret-guard/heuristic'
  local metadata_path='/save-secret-guard/metadata'

  case_write_json "${plaintext_payload_file}" '{"password": "plain-secret", "name": "acme"}'

  case_run_declarest resource save "${heuristic_path}" -f "${plaintext_payload_file}" -i json
  case_expect_failure
  case_expect_output_contains 'potential plaintext secrets detected'
  case_expect_output_contains '--ignore'

  case_run_declarest resource get "${heuristic_path}" --repository
  case_expect_failure
  case_expect_output_contains 'not found'

  case_run_declarest resource save "${heuristic_path}" -f "${plaintext_payload_file}" -i json --ignore
  case_expect_success

  case_run_declarest resource get "${heuristic_path}" --repository -o json
  case_expect_success
  if ! jq -e '.password == "plain-secret" and .name == "acme"' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected --ignore save to persist plaintext payload\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_write_json "${metadata_file}" '{
    "secretsFromAttributes": ["credentials.authValue"],
    "operations": {}
  }'

  case_run_declarest metadata set "${metadata_path}" -f "${metadata_file}" -i json
  case_expect_success

  case_write_json "${metadata_plaintext_payload_file}" '{
    "credentials": {
      "authValue": "plain-metadata-secret"
    }
  }'

  case_run_declarest resource save "${metadata_path}" -f "${metadata_plaintext_payload_file}" -i json
  case_expect_failure
  case_expect_output_contains 'credentials.authValue'

  case_write_json "${metadata_placeholder_payload_file}" '{
    "credentials": {
      "authValue": "{{secret .}}"
    }
  }'

  case_run_declarest resource save "${metadata_path}" -f "${metadata_placeholder_payload_file}" -i json
  case_expect_success

  case_run_declarest resource get "${metadata_path}" --repository -o json
  case_expect_success
  case_expect_output_contains '{{secret .}}'
}
