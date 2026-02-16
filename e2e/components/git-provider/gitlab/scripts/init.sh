#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  gitlab_port=$(e2e_pick_free_port)
  root_password="root-${RANDOM}${RANDOM}${RANDOM}"
  base_url="http://127.0.0.1:${gitlab_port}"

  e2e_write_state_value "${state_file}" GITLAB_HTTP_PORT "${gitlab_port}"
  e2e_write_state_value "${state_file}" GITLAB_ROOT_PASSWORD "${root_password}"
  e2e_write_state_value "${state_file}" GITLAB_BASE_URL "${base_url}"
  e2e_write_state_value "${state_file}" GITLAB_PROJECT_NAME "declarest-e2e"
  e2e_write_state_value "${state_file}" GITLAB_PROJECT_PATH "root/declarest-e2e"
  e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${base_url}/root/declarest-e2e.git"
  e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "main"
  e2e_write_state_value "${state_file}" GIT_AUTH_MODE "basic"
  e2e_write_state_value "${state_file}" GIT_AUTH_USERNAME "root"
  e2e_write_state_value "${state_file}" GIT_AUTH_PASSWORD "${root_password}"
  exit 0
fi

: "${E2E_GITLAB_REMOTE_URL:?missing env E2E_GITLAB_REMOTE_URL}"
: "${E2E_GITLAB_TOKEN:?missing env E2E_GITLAB_TOKEN}"

e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${E2E_GITLAB_REMOTE_URL}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "${E2E_GITLAB_REMOTE_BRANCH:-main}"
e2e_write_state_value "${state_file}" GIT_AUTH_MODE "access-key"
e2e_write_state_value "${state_file}" GIT_AUTH_TOKEN "${E2E_GITLAB_TOKEN}"
