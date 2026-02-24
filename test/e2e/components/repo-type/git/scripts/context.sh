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
  printf '  git:\n'
  printf '    local:\n'
  printf '      base-dir: %s\n' "${REPO_BASE_DIR}"
  printf '    remote:\n'
  printf '      url: %s\n' "${GIT_REMOTE_URL}"
  printf '      branch: %s\n' "${GIT_REMOTE_BRANCH:-main}"
  printf '      provider: %s\n' "${GIT_REMOTE_PROVIDER}"

  if [[ "${GIT_AUTH_MODE:-}" == 'basic' ]]; then
    printf '      auth:\n'
    printf '        basic-auth:\n'
    printf '          username: %s\n' "${GIT_AUTH_USERNAME}"
    printf '          password: %s\n' "${GIT_AUTH_PASSWORD}"
  fi

  if [[ "${GIT_AUTH_MODE:-}" == 'access-key' ]]; then
    printf '      auth:\n'
    printf '        access-key:\n'
    printf '          token: %s\n' "${GIT_AUTH_TOKEN}"
  fi

  metadata_base_dir=${E2E_METADATA_DIR:-${REPO_BASE_DIR}}

  printf 'metadata:\n'
  printf '  base-dir: %s\n' "${metadata_base_dir}"
} >"${fragment_file}"
