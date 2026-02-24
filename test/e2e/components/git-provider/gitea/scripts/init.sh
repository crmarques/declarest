#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

e2e_generate_gitea_admin_password() {
  local random_block
  while true; do
    random_block=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24 || true)
    if [[ ${#random_block} -eq 24 ]]; then
      printf '%s\n' "A9${random_block}"
      return 0
    fi
  done
}

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  gitea_port=$(e2e_pick_free_port)
  admin_username=$(e2e_env_optional 'DECLAREST_E2E_GITEA_ADMIN_USERNAME' 'E2E_GITEA_ADMIN_USERNAME' || true)
  admin_password=$(e2e_env_optional 'DECLAREST_E2E_GITEA_ADMIN_PASSWORD' 'E2E_GITEA_ADMIN_PASSWORD' || true)
  admin_email=$(e2e_env_optional 'DECLAREST_E2E_GITEA_ADMIN_EMAIL' 'E2E_GITEA_ADMIN_EMAIL' || true)

  admin_username=${admin_username:-root}
  admin_email=${admin_email:-declarest-e2e@example.local}

  if [[ -z "${admin_password}" ]]; then
    admin_password=$(e2e_generate_gitea_admin_password)
  fi

  base_url="http://127.0.0.1:${gitea_port}"
  repo_name='declarest-e2e'
  repo_owner="${admin_username}"
  repo_path="${repo_owner}/${repo_name}"

  e2e_write_state_value "${state_file}" GITEA_HTTP_PORT "${gitea_port}"
  e2e_write_state_value "${state_file}" GITEA_BASE_URL "${base_url}"
  e2e_write_state_value "${state_file}" GITEA_ADMIN_USERNAME "${admin_username}"
  e2e_write_state_value "${state_file}" GITEA_ADMIN_PASSWORD "${admin_password}"
  e2e_write_state_value "${state_file}" GITEA_ADMIN_EMAIL "${admin_email}"
  e2e_write_state_value "${state_file}" GITEA_REPO_OWNER "${repo_owner}"
  e2e_write_state_value "${state_file}" GITEA_REPO_NAME "${repo_name}"
  e2e_write_state_value "${state_file}" GITEA_REPO_PATH "${repo_path}"
  e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${base_url}/${repo_path}.git"
  e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "main"
  e2e_write_state_value "${state_file}" GIT_AUTH_MODE "basic"
  e2e_write_state_value "${state_file}" GIT_AUTH_USERNAME "${admin_username}"
  e2e_write_state_value "${state_file}" GIT_AUTH_PASSWORD "${admin_password}"
  exit 0
fi

gitea_remote_url=$(e2e_require_env 'DECLAREST_E2E_GITEA_REMOTE_URL' 'E2E_GITEA_REMOTE_URL') || exit 1
gitea_token=$(e2e_require_env 'DECLAREST_E2E_GITEA_TOKEN' 'E2E_GITEA_TOKEN') || exit 1
gitea_remote_branch=$(e2e_env_optional 'DECLAREST_E2E_GITEA_REMOTE_BRANCH' 'E2E_GITEA_REMOTE_BRANCH' || true)
gitea_remote_branch=${gitea_remote_branch:-main}

e2e_write_state_value "${state_file}" GIT_REMOTE_URL "${gitea_remote_url}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "${gitea_remote_branch}"
e2e_write_state_value "${state_file}" GIT_AUTH_MODE "access-key"
e2e_write_state_value "${state_file}" GIT_AUTH_TOKEN "${gitea_token}"
