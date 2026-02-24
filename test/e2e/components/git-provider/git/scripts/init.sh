#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

remote_repo="${E2E_RUN_DIR}/git-provider-local/remote.git"
mkdir -p "$(dirname -- "${remote_repo}")"

git init --bare "${remote_repo}" >/dev/null

seed_repo="${E2E_RUN_DIR}/git-provider-local/seed"
mkdir -p "${seed_repo}"

(
  cd "${seed_repo}"
  git init >/dev/null
  git config user.email "declarest-e2e@example.local"
  git config user.name "Declarest E2E"
  git checkout -B main >/dev/null
  printf 'declarest e2e seed\n' >README.md
  git add README.md
  git commit -m "seed main branch" >/dev/null
  git remote add origin "${remote_repo}"
  git push -u origin main >/dev/null
)

rm -rf "${seed_repo}"

e2e_write_state_value "${state_file}" GIT_REMOTE_URL "file://${remote_repo}"
e2e_write_state_value "${state_file}" GIT_REMOTE_BRANCH "main"
