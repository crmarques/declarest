#!/usr/bin/env bash

CASE_ID='simple-api-server-basic-auth-contract'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=simple-api-server has-resource-server-basic-auth resource-server-oauth2=false'

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

case_run() {
  simple_api_case_load_state || return 1

  simple_api_case_http_request \
    basic-auth-health-missing-auth \
    "${SIMPLE_API_SERVER_BASE_URL}/health"
  simple_api_case_expect_http_status 401 || return 1
  simple_api_case_expect_header_contains 'WWW-Authenticate: Basic' || return 1
  simple_api_case_expect_body_contains '"error":"invalid_client"' || return 1

  simple_api_case_http_request \
    basic-auth-health-invalid-creds \
    -u "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}:${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}-invalid" \
    "${SIMPLE_API_SERVER_BASE_URL}/health"
  simple_api_case_expect_http_status 401 || return 1
  simple_api_case_expect_body_contains '"error":"invalid_client"' || return 1

  simple_api_case_http_request \
    basic-auth-health-valid-creds \
    -u "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}:${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}" \
    "${SIMPLE_API_SERVER_BASE_URL}/health"
  simple_api_case_expect_http_status 200 || return 1
  simple_api_case_expect_body_contains '"status":"ok"' || return 1

  simple_api_case_http_request \
    basic-auth-token-disabled \
    -X POST "${SIMPLE_API_SERVER_TOKEN_URL}" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'grant_type=client_credentials' \
    --data-urlencode "client_id=${SIMPLE_API_SERVER_CLIENT_ID}" \
    --data-urlencode "client_secret=${SIMPLE_API_SERVER_CLIENT_SECRET}"
  simple_api_case_expect_http_status 404 || return 1
  simple_api_case_expect_body_contains '"oauth2 token endpoint is disabled"' || return 1
}
