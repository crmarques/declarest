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
  printf 'repository:\n'
  printf '  resource-format: %s\n' "${REPO_RESOURCE_FORMAT:-json}"
  printf '  filesystem:\n'
  printf '    base-dir: %s\n' "${REPO_BASE_DIR}"
  metadata_base_dir=${E2E_METADATA_DIR:-${REPO_BASE_DIR}}

  printf 'metadata:\n'
  printf '  base-dir: %s\n' "${metadata_base_dir}"
} >"${fragment_file}"
