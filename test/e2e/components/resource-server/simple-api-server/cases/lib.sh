#!/usr/bin/env bash

SIMPLE_API_CASE_HTTP_LAST_STATUS=''
SIMPLE_API_CASE_HTTP_LAST_BODY=''
SIMPLE_API_CASE_HTTP_LAST_HEADERS=''
SIMPLE_API_CASE_HTTP_LAST_CURL_STATUS=0

simple_api_case_load_state() {
  local state_file
  state_file=$(e2e_component_state_file 'resource-server:simple-api-server')
  if [[ ! -f "${state_file}" ]]; then
    printf 'simple-api-server state file not found: %s\n' "${state_file}" >&2
    return 1
  fi

  # shellcheck disable=SC1090
  source "${state_file}"
}

simple_api_case_http_request() {
  local label=$1
  shift

  local body_file="${E2E_CASE_TMP_DIR}/${label}.body"
  local headers_file="${E2E_CASE_TMP_DIR}/${label}.headers"
  local -a args=(
    curl -sS
    -o "${body_file}"
    -D "${headers_file}"
    -w '%{http_code}'
  )

  if [[ "${SIMPLE_API_SERVER_ENABLE_MTLS:-false}" == 'true' ]]; then
    args+=(
      --cacert "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST}"
      --cert "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST}"
      --key "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST}"
    )
  fi

  local status
  local rc
  set +e
  status=$("${args[@]}" "$@")
  rc=$?
  set -e

  SIMPLE_API_CASE_HTTP_LAST_CURL_STATUS=${rc}
  SIMPLE_API_CASE_HTTP_LAST_STATUS=${status}
  SIMPLE_API_CASE_HTTP_LAST_BODY=$(cat "${body_file}" 2>/dev/null || true)
  SIMPLE_API_CASE_HTTP_LAST_HEADERS=$(cat "${headers_file}" 2>/dev/null || true)
  return 0
}

simple_api_case_expect_curl_ok() {
  if ((SIMPLE_API_CASE_HTTP_LAST_CURL_STATUS != 0)); then
    printf 'expected curl command to succeed but got rc=%d\n' "${SIMPLE_API_CASE_HTTP_LAST_CURL_STATUS}" >&2
    printf 'headers:\n%s\n' "${SIMPLE_API_CASE_HTTP_LAST_HEADERS}" >&2
    printf 'body:\n%s\n' "${SIMPLE_API_CASE_HTTP_LAST_BODY}" >&2
    return 1
  fi
}

simple_api_case_expect_http_status() {
  local expected=$1
  simple_api_case_expect_curl_ok || return 1
  if [[ "${SIMPLE_API_CASE_HTTP_LAST_STATUS}" != "${expected}" ]]; then
    printf 'expected HTTP status %s but got %s\n' "${expected}" "${SIMPLE_API_CASE_HTTP_LAST_STATUS}" >&2
    printf 'headers:\n%s\n' "${SIMPLE_API_CASE_HTTP_LAST_HEADERS}" >&2
    printf 'body:\n%s\n' "${SIMPLE_API_CASE_HTTP_LAST_BODY}" >&2
    return 1
  fi
}

simple_api_case_expect_body_contains() {
  local expected=$1
  if ! grep -Fq -- "${expected}" <<<"${SIMPLE_API_CASE_HTTP_LAST_BODY}"; then
    printf 'expected HTTP body to contain: %s\n' "${expected}" >&2
    printf 'body:\n%s\n' "${SIMPLE_API_CASE_HTTP_LAST_BODY}" >&2
    return 1
  fi
}

simple_api_case_expect_header_contains() {
  local expected=$1
  if ! grep -Fiq -- "${expected}" <<<"${SIMPLE_API_CASE_HTTP_LAST_HEADERS}"; then
    printf 'expected HTTP headers to contain: %s\n' "${expected}" >&2
    printf 'headers:\n%s\n' "${SIMPLE_API_CASE_HTTP_LAST_HEADERS}" >&2
    return 1
  fi
}
