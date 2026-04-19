#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

fragment_file=${1:-${E2E_COMPONENT_CONTEXT_FRAGMENT:-}}
[[ -n "${fragment_file}" ]] || {
  printf 'missing context fragment output path\n' >&2
  exit 1
}

base_url="${MANAGED_SERVICE_BASE_URL:-${HAPROXY_BASE_URL%/}/v3}"
selected_auth_type=${E2E_MANAGED_SERVICE_AUTH_TYPE:-basic}

{
  printf 'managedService:\n'
  printf '  http:\n'
  printf '    url: %s\n' "${base_url}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi
  printf '    auth:\n'
  case "${selected_auth_type}" in
    basic)
      printf '      basic:\n'
      printf '        username: %s\n' "${HAPROXY_ADMIN_USER}"
      printf '        password: %s\n' "${HAPROXY_ADMIN_PASSWORD}"
      ;;
    prompt)
      printf '      prompt: {}\n'
      ;;
    *)
      printf 'managed-service haproxy does not support auth-type %s (supported: basic, prompt)\n' "${selected_auth_type}" >&2
      exit 1
      ;;
  esac
} >"${fragment_file}"
