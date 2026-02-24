#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

github_remote_url=$(e2e_require_env 'DECLAREST_E2E_GITHUB_REMOTE_URL' 'E2E_GITHUB_REMOTE_URL') || exit 1
github_token=$(e2e_require_env 'DECLAREST_E2E_GITHUB_TOKEN' 'E2E_GITHUB_TOKEN') || exit 1
github_remote_branch=$(e2e_env_optional 'DECLAREST_E2E_GITHUB_REMOTE_BRANCH' 'E2E_GITHUB_REMOTE_BRANCH' || true)
github_remote_branch=${github_remote_branch:-main}

e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${github_remote_url}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "${github_remote_branch}"
e2e_write_state_value "${state_file}" GIT_AUTH_MODE "access-key"
e2e_write_state_value "${state_file}" GIT_AUTH_TOKEN "${github_token}"
