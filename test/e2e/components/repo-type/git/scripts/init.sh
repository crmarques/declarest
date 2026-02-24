#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

repo_dir="${E2E_RUN_DIR}/repo-git"
mkdir -p "${repo_dir}"

provider_state="${E2E_STATE_DIR}/git-provider-${E2E_GIT_PROVIDER}.env"
[[ -f "${provider_state}" ]] || {
  printf 'missing git provider state file: %s\n' "${provider_state}" >&2
  exit 1
}

# shellcheck disable=SC1090
source "${provider_state}"

: "${GIT_REMOTE_URL:?git provider state missing GIT_REMOTE_URL}"

e2e_write_state_value "${state_file}" REPO_BASE_DIR "${repo_dir}"
e2e_write_state_value "${state_file}" REPO_RESOURCE_FORMAT "json"
e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${GIT_REMOTE_URL}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "${GIT_REMOTE_BRANCH:-main}"
e2e_write_state_value "${state_file}" GIT_REMOTE_PROVIDER "${E2E_GIT_PROVIDER}"

if [[ -n "${GIT_AUTH_MODE:-}" ]]; then
  e2e_write_state_value "${state_file}" GIT_AUTH_MODE "${GIT_AUTH_MODE}"
fi
if [[ -n "${GIT_AUTH_USERNAME:-}" ]]; then
  e2e_write_state_value "${state_file}" GIT_AUTH_USERNAME "${GIT_AUTH_USERNAME}"
fi
if [[ -n "${GIT_AUTH_PASSWORD:-}" ]]; then
  e2e_write_state_value "${state_file}" GIT_AUTH_PASSWORD "${GIT_AUTH_PASSWORD}"
fi
if [[ -n "${GIT_AUTH_TOKEN:-}" ]]; then
  e2e_write_state_value "${state_file}" GIT_AUTH_TOKEN "${GIT_AUTH_TOKEN}"
fi
