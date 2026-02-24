#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

fragment_file=${1:-${E2E_COMPONENT_CONTEXT_FRAGMENT:-}}
[[ -n "${fragment_file}" ]] || {
  printf 'missing context fragment output path\n' >&2
  exit 1
}

api_version="${RUNDECK_API_VERSION:-45}"
base_url="${RUNDECK_BASE_URL%/}/api/${api_version}"

{
  printf 'resource-server:\n'
  printf '  http:\n'
  printf '    base-url: %s\n' "${base_url}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi
  printf '    auth:\n'

  if [[ "${RUNDECK_AUTH_MODE:-}" == 'token' && -n "${RUNDECK_API_TOKEN:-}" ]]; then
    printf '      custom-header:\n'
    printf '        header: %s\n' "${RUNDECK_AUTH_HEADER:-X-Rundeck-Auth-Token}"
    printf '        token: %s\n' "${RUNDECK_API_TOKEN}"
  else
    printf '      basic-auth:\n'
    printf '        username: %s\n' "${RUNDECK_ADMIN_USER}"
    printf '        password: %s\n' "${RUNDECK_ADMIN_PASSWORD}"
  fi
} >"${fragment_file}"
