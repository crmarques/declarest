#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

: "${E2E_GITHUB_REMOTE_URL:?missing env E2E_GITHUB_REMOTE_URL}"
: "${E2E_GITHUB_TOKEN:?missing env E2E_GITHUB_TOKEN}"

e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${E2E_GITHUB_REMOTE_URL}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "${E2E_GITHUB_REMOTE_BRANCH:-main}"
e2e_write_state_value "${state_file}" GIT_AUTH_MODE "access-key"
e2e_write_state_value "${state_file}" GIT_AUTH_TOKEN "${E2E_GITHUB_TOKEN}"
