#!/usr/bin/env bash

CASE_ID='list-deterministic'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=keycloak'

case_run() {
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata.json"
  local payload_a="${E2E_CASE_TMP_DIR}/a.json"
  local payload_b="${E2E_CASE_TMP_DIR}/b.json"

  case_write_json "${metadata_file}" '{
    "idFromAttribute": "id",
    "aliasFromAttribute": "alias",
    "operations": {
      "create": {"method": "POST", "path": "/customers"},
      "list": {"method": "GET", "path": "/customers"}
    },
    "filter": [],
    "suppress": []
  }'

  case_write_json "${payload_a}" '{"id": "list-b", "alias": "beta", "name": "Beta"}'
  case_write_json "${payload_b}" '{"id": "list-a", "alias": "alpha", "name": "Alpha"}'

  case_run_declarest metadata set /customers-list -f "${metadata_file}" -i json
  case_expect_success

  case_run_declarest resource create /customers-list/beta -f "${payload_a}" -i json
  case_expect_success

  case_run_declarest resource create /customers-list/alpha -f "${payload_b}" -i json
  case_expect_success

  case_run_declarest resource list /customers-list --source remote -o json
  case_expect_success

  if ! jq -e 'map(.LogicalPath) as $paths | $paths == ($paths | sort)' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
    printf 'expected deterministic sorted list order\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}
