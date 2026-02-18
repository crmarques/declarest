#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

repo_dir="${E2E_RUN_DIR}/repo-filesystem"
mkdir -p "${repo_dir}"

e2e_write_state_value "${state_file}" REPO_BASE_DIR "${repo_dir}"
e2e_write_state_value "${state_file}" REPO_RESOURCE_FORMAT "json"
