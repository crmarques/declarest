#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

fragment_file=${1:-${E2E_COMPONENT_CONTEXT_FRAGMENT:-}}
[[ -n "${fragment_file}" ]] || {
  printf 'missing context fragment output path\n' >&2
  exit 1
}

{
  printf 'resource-server:\n'
  printf '  http:\n'
  printf '    base-url: %s\n' "${KEYCLOAK_BASE_URL}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi
  printf '    auth:\n'
  printf '      oauth2:\n'
    printf '        token-url: %s\n' "${KEYCLOAK_TOKEN_URL}"
  printf '        grant-type: client_credentials\n'
  printf '        client-id: %s\n' "${KEYCLOAK_CLIENT_ID}"
  printf '        client-secret: %s\n' "${KEYCLOAK_CLIENT_SECRET}"

  if [[ -n "${KEYCLOAK_SCOPE:-}" ]]; then
    printf '        scope: %s\n' "${KEYCLOAK_SCOPE}"
  fi
  if [[ -n "${KEYCLOAK_AUDIENCE:-}" ]]; then
    printf '        audience: %s\n' "${KEYCLOAK_AUDIENCE}"
  fi
} >"${fragment_file}"
