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
  printf '    base-url: %s\n' "${VAULT_ADDRESS}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi
  printf '    auth:\n'
  printf '      bearer-token:\n'
  printf '        token: %s\n' "${VAULT_TOKEN}"
} >"${fragment_file}"
