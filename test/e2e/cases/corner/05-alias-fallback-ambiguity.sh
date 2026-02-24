#!/usr/bin/env bash

CASE_ID='alias-fallback-ambiguity'
CASE_SCOPE='corner'
CASE_REQUIRES='resource-server=keycloak'

case_run() {
  local metadata_file="${E2E_CASE_TMP_DIR}/metadata.json"
  local item_one="${E2E_CASE_TMP_DIR}/one.json"
  local item_two="${E2E_CASE_TMP_DIR}/two.json"
  local local_payload="${E2E_CASE_TMP_DIR}/local.json"
  local suffix="${RANDOM}${RANDOM}"
  local client_one="e2e-amb-one-${suffix}"
  local client_two="e2e-amb-two-${suffix}"
  local local_alias="e2e-amb-local-${suffix}"
  local remote_id_two

  case_write_json "${metadata_file}" '{
    "idFromAttribute": "id",
    "aliasFromAttribute": "clientId",
    "operations": {
      "create": {"method": "POST", "path": "/admin/realms/master/clients"}
    },
    "filter": [],
    "suppress": []
  }'

  case_write_json "${item_one}" "{\"clientId\": \"${client_one}\", \"name\": \"One\", \"enabled\": true, \"publicClient\": true, \"protocol\": \"openid-connect\"}"
  case_write_json "${item_two}" "{\"clientId\": \"${client_two}\", \"name\": \"Two\", \"enabled\": true, \"publicClient\": true, \"protocol\": \"openid-connect\"}"

  case_run_declarest metadata set /admin/realms/master/clients/_ -f "${metadata_file}" -i json
  case_expect_success

  case_run_declarest resource create "/admin/realms/master/clients/${client_one}" -f "${item_one}" -i json
  case_expect_success

  case_run_declarest resource create "/admin/realms/master/clients/${client_two}" -f "${item_two}" -i json
  case_expect_success

  case_run_declarest resource get "/admin/realms/master/clients/${client_two}" --remote-server -o json
  case_expect_success
  remote_id_two=$(jq -r '.id // empty' <<<"${CASE_LAST_OUTPUT}")
  if [[ -z "${remote_id_two}" || "${remote_id_two}" == 'null' ]]; then
    printf 'could not resolve remote id for %s\n' "${client_two}" >&2
    return 1
  fi

  case_write_json "${local_payload}" "{\"id\": \"${remote_id_two}\", \"clientId\": \"${client_one}\", \"name\": \"Local\", \"enabled\": true, \"publicClient\": true, \"protocol\": \"openid-connect\"}"
  case_run_declarest resource save "/admin/realms/master/clients/${local_alias}" -f "${local_payload}" -i json
  case_expect_success

  case_run_declarest resource diff "/admin/realms/master/clients/${local_alias}"
  case_expect_failure
  case_expect_output_contains 'ambiguous'
}
