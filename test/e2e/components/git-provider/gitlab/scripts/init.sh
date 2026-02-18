#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

e2e_generate_gitlab_root_password() {
  local candidate
  local random_block
  local lowered

  while true; do
    random_block=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 22 || true)
    if [[ ${#random_block} -ne 22 ]]; then
      continue
    fi

    candidate="X9!${random_block}#"
    lowered=${candidate,,}

    if [[ "${lowered}" == *password* || "${lowered}" == *root* || "${lowered}" == *admin* || "${lowered}" == *gitlab* ]]; then
      continue
    fi

    printf '%s\n' "${candidate}"
    return 0
  done
}

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  gitlab_port=$(e2e_pick_free_port)
  root_password=$(e2e_env_optional 'DECLAREST_E2E_GITLAB_ROOT_PASSWORD' 'E2E_GITLAB_ROOT_PASSWORD' || true)
  if [[ -z "${root_password}" ]]; then
    root_password=$(e2e_generate_gitlab_root_password)
  fi
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

gitlab_remote_url=$(e2e_require_env 'DECLAREST_E2E_GITLAB_REMOTE_URL' 'E2E_GITLAB_REMOTE_URL') || exit 1
gitlab_token=$(e2e_require_env 'DECLAREST_E2E_GITLAB_TOKEN' 'E2E_GITLAB_TOKEN') || exit 1
gitlab_remote_branch=$(e2e_env_optional 'DECLAREST_E2E_GITLAB_REMOTE_BRANCH' 'E2E_GITLAB_REMOTE_BRANCH' || true)
gitlab_remote_branch=${gitlab_remote_branch:-main}

e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${gitlab_remote_url}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "${gitlab_remote_branch}"
e2e_write_state_value "${state_file}" GIT_AUTH_MODE "access-key"
e2e_write_state_value "${state_file}" GIT_AUTH_TOKEN "${gitlab_token}"
