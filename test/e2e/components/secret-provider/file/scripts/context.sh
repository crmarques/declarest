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
  printf '  file:\n'
  printf '    path: %s\n' "${SECRET_FILE_PATH}"
  printf '    passphrase: %s\n' "${SECRET_FILE_PASSPHRASE}"
} >"${fragment_file}"
