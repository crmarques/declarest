#!/usr/bin/env bash

CASE_ID='save-list-input-modes'
CASE_SCOPE='main'
CASE_REQUIRES=''

case_run() {
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata.json"
  local list_payload_file="${E2E_CASE_TMP_DIR}/list.json"
  local as_items_collection='/save-input-modes-items'
  local as_one_resource_path='/save-input-modes-one'

  case_write_json "${metadata_file}" '{
    "idFromAttribute": "id",
    "aliasFromAttribute": "id",
    "operations": {}
  }'

  case_write_json "${list_payload_file}" '[
    {"id": "zeta", "tier": "pro"},
    {"id": "alpha", "tier": "free"}
  ]'

  case_run_declarest metadata set "${as_items_collection}/_" -f "${metadata_file}" -i json
  case_expect_success
  case_repo_commit_setup_changes_if_git

  case_run_declarest resource save "${as_items_collection}" -f "${list_payload_file}" -i json --as-items
  case_expect_success

  case_run_declarest resource list "${as_items_collection}" --repository -r -o json
  case_expect_success
  if ! jq -e 'type == "array" and (map(.id) == ["alpha", "zeta"])' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected --as-items to fan out list payload into deterministic sorted items\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource get /save-input-modes-items/alpha --repository -o json
  case_expect_success
  if ! jq -e '.id == "alpha" and .tier == "free"' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected --as-items to persist alpha payload\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource save "${as_one_resource_path}" -f "${list_payload_file}" -i json --as-one-resource
  case_expect_success

  case_run_declarest resource get "${as_one_resource_path}" --repository -o json
  case_expect_success
  if ! jq -e 'type == "array" and length == 2 and (map(.id) | sort) == ["alpha", "zeta"]' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected --as-one-resource to persist list payload at one logical path\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource get "${as_one_resource_path}/alpha" --repository -o json
  if ((CASE_LAST_STATUS == 0)); then
    if ! jq -e 'type == "array" and length == 0' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
      printf 'expected no child resource persisted under --as-one-resource target\n' >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi
  else
    case_expect_output_contains 'not found'
  fi
}
