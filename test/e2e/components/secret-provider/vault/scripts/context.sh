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
  printf 'secret-store:\n'
  printf '  vault:\n'
  printf '    address: %s\n' "${VAULT_ADDRESS}"
  printf '    mount: %s\n' "${VAULT_MOUNT:-secret}"
  printf '    path-prefix: %s\n' "${VAULT_PATH_PREFIX:-declarest-e2e}"
  printf '    kv-version: %s\n' "${VAULT_KV_VERSION:-2}"
  printf '    auth:\n'

  case "${VAULT_AUTH_MODE:-token}" in
    token)
      printf '      token: %s\n' "${VAULT_TOKEN}"
      ;;
    password)
      printf '      password:\n'
      printf '        username: %s\n' "${VAULT_USERNAME}"
      printf '        password: %s\n' "${VAULT_PASSWORD}"
      printf '        mount: %s\n' "${VAULT_AUTH_MOUNT:-userpass}"
      ;;
    approle)
      printf '      approle:\n'
      printf '        role-id: %s\n' "${VAULT_ROLE_ID}"
      printf '        secret-id: %s\n' "${VAULT_SECRET_ID}"
      printf '        mount: %s\n' "${VAULT_AUTH_MOUNT:-approle}"
      ;;
  esac
} >"${fragment_file}"
