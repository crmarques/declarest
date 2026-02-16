#!/usr/bin/env bash

CASE_ID='alias-fallback-ambiguity'
CASE_SCOPE='corner'
CASE_REQUIRES='resource-server=keycloak'

case_run() {
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata.json"
  local item_one="${E2E_CASE_TMP_DIR}/one.json"
  local item_two="${E2E_CASE_TMP_DIR}/two.json"
  local local_payload="${E2E_CASE_TMP_DIR}/local.json"

  case_write_json "${metadata_file}" '{
    "idFromAttribute": "uuid",
    "aliasFromAttribute": "alias",
    "operations": {
      "create": {"method": "POST", "path": "/customers"},
      "get": {"method": "GET", "path": "/customers/{{.id}}"},
      "list": {"method": "GET", "path": "/customers"}
    },
    "filter": [],
    "suppress": []
  }'

  case_write_json "${item_one}" '{"id": "cust-1", "uuid": "uuid-1", "alias": "duplicate-alias", "name": "One"}'
  case_write_json "${item_two}" '{"id": "cust-2", "uuid": "uuid-2", "alias": "other-alias", "name": "Two"}'
  case_write_json "${local_payload}" '{"id": "missing-id", "uuid": "uuid-2", "alias": "duplicate-alias", "name": "Local"}'

  case_run_declarest metadata set /customers-ambiguity/_ -f "${metadata_file}" -i json
  case_expect_success

  case_run_declarest resource create /customers-ambiguity/one -f "${item_one}" -i json
  case_expect_success

  case_run_declarest resource create /customers-ambiguity/two -f "${item_two}" -i json
  case_expect_success

  case_run_declarest resource save /customers-ambiguity/local -f "${local_payload}" -i json
  case_expect_success

  case_run_declarest resource diff /customers-ambiguity/local
  case_expect_failure
  case_expect_output_contains 'ambiguous'
}
