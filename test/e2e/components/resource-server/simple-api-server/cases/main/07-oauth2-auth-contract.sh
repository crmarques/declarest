#!/usr/bin/env bash

CASE_ID='simple-api-server-oauth2-auth-contract'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=simple-api-server has-resource-server-oauth2 resource-server-basic-auth=false'

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

case_run() {
  local token_response
  local access_token

  simple_api_case_load_state || return 1

  simple_api_case_http_request \
    oauth2-health-missing-auth \
    "${SIMPLE_API_SERVER_BASE_URL}/health"
  simple_api_case_expect_http_status 401 || return 1
  simple_api_case_expect_header_contains 'WWW-Authenticate: Bearer' || return 1
  simple_api_case_expect_body_contains '"error":"invalid_token"' || return 1

  simple_api_case_http_request \
    oauth2-health-invalid-token \
    -H 'Authorization: Bearer invalid-token' \
    "${SIMPLE_API_SERVER_BASE_URL}/health"
  simple_api_case_expect_http_status 401 || return 1
  simple_api_case_expect_body_contains '"error":"invalid_token"' || return 1

  simple_api_case_http_request \
    oauth2-token-invalid-client \
    -X POST "${SIMPLE_API_SERVER_TOKEN_URL}" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode "client_id=${SIMPLE_API_SERVER_CLIENT_ID}" \
    --data-urlencode "client_secret=${SIMPLE_API_SERVER_CLIENT_SECRET}-invalid"
  simple_api_case_expect_http_status 401 || return 1
  simple_api_case_expect_body_contains '"error":"invalid_client"' || return 1

  simple_api_case_http_request \
    oauth2-token-valid-client \
    -X POST "${SIMPLE_API_SERVER_TOKEN_URL}" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode "client_id=${SIMPLE_API_SERVER_CLIENT_ID}" \
    --data-urlencode "client_secret=${SIMPLE_API_SERVER_CLIENT_SECRET}"
  simple_api_case_expect_http_status 200 || return 1
  simple_api_case_expect_body_contains '"access_token":"' || return 1
  simple_api_case_expect_body_contains '"token_type":"Bearer"' || return 1

  token_response=${SIMPLE_API_CASE_HTTP_LAST_BODY}
  access_token=$(jq -r '.access_token // empty' <<<"${token_response}" 2>/dev/null || true)
  if [[ -z "${access_token}" ]]; then
    printf 'oauth2 token response missing access_token: %s\n' "${token_response}" >&2
    return 1
  fi

  simple_api_case_http_request \
    oauth2-health-valid-token \
    -H "Authorization: Bearer ${access_token}" \
    "${SIMPLE_API_SERVER_BASE_URL}/health"
  simple_api_case_expect_http_status 200 || return 1
  simple_api_case_expect_body_contains '"status":"ok"' || return 1
}
