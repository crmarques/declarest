#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

fragment_file=${1:-${E2E_COMPONENT_CONTEXT_FRAGMENT:-}}
[[ -n "${fragment_file}" ]] || {
  printf 'missing context fragment output path\n' >&2
  exit 1
}

selected_auth_type=${E2E_RESOURCE_SERVER_AUTH_TYPE:-custom-header}
if [[ "${selected_auth_type}" != 'custom-header' ]]; then
  printf 'resource-server vault does not support auth-type %s (supported: custom-header)\n' "${selected_auth_type}" >&2
  exit 1
fi

{
  printf 'resource-server:\n'
  printf '  http:\n'
  printf '    base-url: %s\n' "${VAULT_ADDRESS}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi
  printf '    auth:\n'
  printf '      custom-header:\n'
  printf '        header: X-Vault-Token\n'
  printf '        value: %s\n' "${VAULT_TOKEN}"
} >"${fragment_file}"
