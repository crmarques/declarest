#!/usr/bin/env bash
set -euo pipefail

CASE_ID='managed-server-proxy-context'
CASE_SCOPE='main'
CASE_REQUIRES='managed-server-proxy=true'

case_assert_context_contains() {
  local expected=$1
  if ! grep -Fq -- "${expected}" "${E2E_CONTEXT_FILE}"; then
    printf 'expected context file %s to contain %q\n' "${E2E_CONTEXT_FILE}" "${expected}" >&2
    return 1
  fi
}

case_run() {
  [[ -n "${E2E_CONTEXT_FILE:-}" && -f "${E2E_CONTEXT_FILE}" ]] || {
    printf 'context file not found: %s\n' "${E2E_CONTEXT_FILE:-<empty>}" >&2
    return 1
  }

  case_assert_context_contains 'proxy:'

  if [[ -n "${E2E_MANAGED_SERVER_PROXY_HTTP_URL:-}" ]]; then
    case_assert_context_contains "http-url: '${E2E_MANAGED_SERVER_PROXY_HTTP_URL}'"
  fi
  if [[ -n "${E2E_MANAGED_SERVER_PROXY_HTTPS_URL:-}" ]]; then
    case_assert_context_contains "https-url: '${E2E_MANAGED_SERVER_PROXY_HTTPS_URL}'"
  fi
  if [[ -n "${E2E_MANAGED_SERVER_PROXY_NO_PROXY:-}" ]]; then
    case_assert_context_contains "no-proxy: '${E2E_MANAGED_SERVER_PROXY_NO_PROXY}'"
  fi
  if [[ "${E2E_MANAGED_SERVER_PROXY_AUTH_TYPE:-}" == 'prompt' ]]; then
    case_assert_context_contains 'prompt: {}'
  elif [[ -n "${E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME:-}" ]]; then
    case_assert_context_contains "username: '${E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME}'"
    case_assert_context_contains "password: '${E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD}'"
  fi
}
