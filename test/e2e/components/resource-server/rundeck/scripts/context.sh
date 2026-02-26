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
selected_auth_type=${E2E_RESOURCE_SERVER_AUTH_TYPE:-custom-header}

{
  printf 'resource-server:\n'
  printf '  http:\n'
  printf '    base-url: %s\n' "${base_url}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi
  printf '    auth:\n'
  if [[ "${selected_auth_type}" == 'custom-header' ]]; then
    if [[ -z "${RUNDECK_API_TOKEN:-}" ]]; then
      printf 'missing RUNDECK_API_TOKEN for rundeck auth-type custom-header\n' >&2
      exit 1
    fi
    printf '      custom-header:\n'
    printf '        header: %s\n' "${RUNDECK_AUTH_HEADER:-X-Rundeck-Auth-Token}"
    printf '        value: %s\n' "${RUNDECK_API_TOKEN}"
  elif [[ "${selected_auth_type}" == 'basic' ]]; then
    printf '      basic-auth:\n'
    printf '        username: %s\n' "${RUNDECK_ADMIN_USER}"
    printf '        password: %s\n' "${RUNDECK_ADMIN_PASSWORD}"
  else
    printf 'resource-server rundeck does not support auth-type %s (supported: basic, custom-header)\n' "${selected_auth_type}" >&2
    exit 1
  fi
} >"${fragment_file}"
