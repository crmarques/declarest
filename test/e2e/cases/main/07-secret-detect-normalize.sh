#!/usr/bin/env bash

CASE_ID='secret-detect-normalize'
CASE_SCOPE='main'
CASE_REQUIRES='has-secret-provider'

case_run() {
  local detect_payload_file="${E2E_CASE_TMP_DIR}/detect-payload.json"
  local normalize_payload_file="${E2E_CASE_TMP_DIR}/normalize-payload.json"
  local metadata_target_path='/secret-detect-fix/acme'
  local repo_detect_scope='/secret-detect-repo-scan'
  local repo_detect_acme_payload_file="${E2E_CASE_TMP_DIR}/repo-detect-acme.json"
  local repo_detect_beta_payload_file="${E2E_CASE_TMP_DIR}/repo-detect-beta.json"

  case_write_json "${detect_payload_file}" '{
    "name": "acme",
    "apiToken": "token-123",
    "password": "pw-456",
    "nested": {
      "apiToken": "{{secret \"apiToken\"}}"
    }
  }'

  case_run_declarest secret detect -f "${detect_payload_file}" -i json -o json
  case_expect_success
  if ! jq -e '. == ["apiToken", "password"]' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected deterministic detected secret keys\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_write_json "${normalize_payload_file}" '{
    "apiToken": "{{ secret . }}",
    "nested": {
      "clientSecret": "{{secret .}}"
    }
  }'

  case_run_declarest secret normalize -f "${normalize_payload_file}" -i json -o json
  case_expect_success
  if ! jq -e '.apiToken == "{{secret .}}" and .nested.clientSecret == "{{secret .}}"' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected normalized placeholders with resolved keys\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest secret detect --path "${metadata_target_path}" --fix --secret-attribute password -f "${detect_payload_file}" -i json -o json
  case_expect_success
  if ! jq -e '. == ["password"]' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected --secret-attribute to filter detect output when using --fix\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest metadata get "${metadata_target_path}" -o json
  case_expect_success
  if ! jq -e '.resourceInfo.secretInAttributes == ["password"]' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected --fix to write resourceInfo.secretInAttributes metadata\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_write_json "${repo_detect_acme_payload_file}" '{
    "name": "acme",
    "password": "pw-001"
  }'
  case_write_json "${repo_detect_beta_payload_file}" '{
    "name": "beta",
    "apiToken": "token-002"
  }'

  case_run_declarest resource save "${repo_detect_scope}/acme" --ignore -f "${repo_detect_acme_payload_file}" -i json
  case_expect_success
  case_run_declarest resource save "${repo_detect_scope}/beta" --ignore -f "${repo_detect_beta_payload_file}" -i json
  case_expect_success

  case_run_declarest secret detect -o json
  case_expect_success
  if ! jq -e --arg scope "${repo_detect_scope}" '
    any(.[]; .LogicalPath == ($scope + "/acme") and .Attributes == ["password"]) and
    any(.[]; .LogicalPath == ($scope + "/beta") and .Attributes == ["apiToken"])
  ' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected repo-wide detect to include saved secret candidates\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest secret detect "${repo_detect_scope}" -o json
  case_expect_success
  if ! jq -e --arg scope "${repo_detect_scope}" '
    length >= 2 and all(.[]; (.LogicalPath | startswith($scope + "/")))
  ' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected scoped detect to include only resources under requested path\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}
