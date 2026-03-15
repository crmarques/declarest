#!/usr/bin/env bash
set -euo pipefail

CASE_ID='proxy-context'
CASE_SCOPE='main'
CASE_REQUIRES='has-proxy'

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

  case_assert_context_contains 'managedServer:'
  case_assert_context_contains 'proxy:'

  if [[ -n "${E2E_PROXY_HTTP_URL:-}" ]]; then
    case_assert_context_contains "httpURL: '${E2E_PROXY_HTTP_URL}'"
  elif [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    case_assert_context_contains "httpURL: 'http://127.0.0.1:"
  fi
  if [[ -n "${E2E_PROXY_HTTPS_URL:-}" ]]; then
    case_assert_context_contains "httpsURL: '${E2E_PROXY_HTTPS_URL}'"
  fi
  if [[ -n "${E2E_PROXY_NO_PROXY:-}" ]]; then
    case_assert_context_contains "noProxy: '${E2E_PROXY_NO_PROXY}'"
  fi
  if [[ "$(e2e_effective_proxy_auth_type)" == 'prompt' ]]; then
    case_assert_context_contains 'prompt: {}'
  elif [[ -n "${E2E_PROXY_AUTH_USERNAME:-}" ]]; then
    case_assert_context_contains "username: '${E2E_PROXY_AUTH_USERNAME}'"
    case_assert_context_contains "password: '${E2E_PROXY_AUTH_PASSWORD}'"
  fi

  if [[ "${E2E_METADATA:-bundle}" == 'bundle' ]]; then
    case_assert_context_contains 'metadata:'
  fi
}
