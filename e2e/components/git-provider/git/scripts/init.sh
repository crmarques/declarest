#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

remote_repo="${E2E_RUN_DIR}/git-provider-local/remote.git"
mkdir -p "$(dirname -- "${remote_repo}")"

git init --bare "${remote_repo}" >/dev/null

e2e_write_state_value "${state_file}" GIT_REMOTE_URL "file://${remote_repo}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "main"
