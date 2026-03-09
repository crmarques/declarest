#!/usr/bin/env bash

CASE_ID='save-secret-plaintext-guard'
CASE_SCOPE='corner'
CASE_REQUIRES=''

case_run() {
  local plaintext_payload_file="${E2E_CASE_TMP_DIR}/plaintext.json"
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata.json"
  local metadata_plaintext_payload_file="${E2E_CASE_TMP_DIR}/metadata-plaintext.json"
  local metadata_placeholder_payload_file="${E2E_CASE_TMP_DIR}/metadata-placeholder.json"
  local repo_dir
  local heuristic_repo_file

  local heuristic_path='/save-secret-guard/heuristic'
  local metadata_path='/save-secret-guard/metadata'

  case_write_json "${plaintext_payload_file}" '{"password": "plain-secret", "name": "acme"}'

  case_run_declarest resource save "${heuristic_path}" -f "${plaintext_payload_file}" -i json
  case_expect_failure
  case_expect_output_contains 'potential plaintext secrets detected'
  case_expect_output_contains '--allow-plaintext'

  repo_dir=$(case_context_repo_base_dir) || return 1
  heuristic_repo_file="${repo_dir}/save-secret-guard/heuristic/resource.json"
  if [[ -e "${heuristic_repo_file}" ]]; then
    printf 'expected failed save to leave repository path absent: %s\n' "${heuristic_repo_file}" >&2
    return 1
  fi

  case_run_declarest resource save "${heuristic_path}" -f "${plaintext_payload_file}" -i json --allow-plaintext
  case_expect_success

  case_run_declarest resource get "${heuristic_path}" --source repository -o json
  case_expect_success
  if ! jq -e '.password == "plain-secret" and .name == "acme"' <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected --allow-plaintext save to persist plaintext payload\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_write_json "${metadata_file}" '{
    "resource": {
      "secretAttributes": ["/credentials/authValue"]
    }
  }'

  case_run_declarest metadata set "${metadata_path}" -f "${metadata_file}" -i json
  case_expect_success
  case_repo_commit_setup_changes_if_git

  case_write_json "${metadata_plaintext_payload_file}" '{
    "credentials": {
      "authValue": "plain-metadata-secret"
    }
  }'

  case_run_declarest resource save "${metadata_path}" -f "${metadata_plaintext_payload_file}" -i json
  case_expect_success

  case_run_declarest resource get "${metadata_path}" --source repository -o json
  case_expect_success
  case_expect_output_contains '{{secret .}}'

  case_write_json "${metadata_placeholder_payload_file}" '{
    "credentials": {
      "authValue": "{{secret .}}"
    }
  }'

  case_run_declarest resource save "${metadata_path}" -f "${metadata_placeholder_payload_file}" -i json --force
  case_expect_success

  case_run_declarest resource get "${metadata_path}" --source repository -o json
  case_expect_success
  case_expect_output_contains '{{secret .}}'
}
