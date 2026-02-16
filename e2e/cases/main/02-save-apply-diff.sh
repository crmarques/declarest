#!/usr/bin/env bash

CASE_ID='save-apply-diff'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=keycloak'

case_run() {
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata.json"
  local payload_file="${E2E_CASE_TMP_DIR}/payload.json"

  case_write_json "${metadata_file}" '{
    "idFromAttribute": "id",
    "aliasFromAttribute": "alias",
    "operations": {
      "create": {"method": "POST", "path": "/customers"},
      "get": {"method": "GET", "path": "/customers/{{.id}}"},
      "update": {"method": "PUT", "path": "/customers/{{.id}}"},
      "delete": {"method": "DELETE", "path": "/customers/{{.id}}"},
      "list": {"method": "GET", "path": "/customers"}
    },
    "filter": [],
    "suppress": []
  }'

  case_write_json "${payload_file}" '{
    "id": "main-apply-acme",
    "alias": "main-apply-acme",
    "name": "Main Apply"
  }'

  case_run_declarest metadata set /customers-main-apply -f "${metadata_file}" -i json
  case_expect_success

  case_run_declarest resource save /customers-main-apply/acme -f "${payload_file}" -i json
  case_expect_success

  case_run_declarest resource apply /customers-main-apply/acme
  case_expect_success

  case_run_declarest resource diff /customers-main-apply/acme -o json
  case_expect_success
  case_expect_output_contains '[]'
}
