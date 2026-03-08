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
  printf '  git:\n'
  printf '    local:\n'
  printf '      baseDir: %s\n' "${REPO_BASE_DIR}"
  printf '    remote:\n'
  printf '      url: %s\n' "${GIT_REMOTE_URL}"
  printf '      branch: %s\n' "${GIT_REMOTE_BRANCH:-main}"
  printf '      provider: %s\n' "${GIT_REMOTE_PROVIDER}"

  if [[ "${GIT_AUTH_MODE:-}" == 'basic' ]]; then
    printf '      auth:\n'
    printf '        basicAuth:\n'
    printf '          username: %s\n' "${GIT_AUTH_USERNAME}"
    printf '          password: %s\n' "${GIT_AUTH_PASSWORD}"
  fi

  if [[ "${GIT_AUTH_MODE:-}" == 'access-key' ]]; then
    printf '      auth:\n'
    printf '        accessKey:\n'
    printf '          token: %s\n' "${GIT_AUTH_TOKEN}"
  fi

  printf 'metadata:\n'
  if [[ -n "${E2E_METADATA_BUNDLE:-}" ]]; then
    printf '  bundle: %s\n' "${E2E_METADATA_BUNDLE}"
  else
    metadata_base_dir=${E2E_METADATA_DIR:-${REPO_BASE_DIR}}
    printf '  baseDir: %s\n' "${metadata_base_dir}"
  fi
} >"${fragment_file}"
