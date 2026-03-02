#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_compose_gitea_admin_exec_invokes_gitea_binary() {
  local tmp
  tmp=$(new_temp_dir)

  local fake_e2e="${tmp}/e2e"
  mkdir -p "${fake_e2e}/lib"
  cat >"${fake_e2e}/lib/common.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

e2e_compose_cmd() {
  local cmd_string
  printf -v cmd_string '%q ' "$@"
  cmd_string=${cmd_string% }
  printf '%s\n' "${cmd_string}" >> "${E2E_TEST_CAPTURE_FILE}"
}
EOF

  local bin_dir="${tmp}/bin"
  mkdir -p "${bin_dir}"
  cat >"${bin_dir}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

url=''
for arg in "$@"; do
  case "${arg}" in
    http://*|https://*)
      url=${arg}
      ;;
  esac
done

case "${url}" in
  */user/login)
    exit 0
    ;;
  */api/v1/users/*)
    exit 22
    ;;
  */api/v1/repos/*)
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
EOF
  chmod +x "${bin_dir}/curl"

  local state_file="${tmp}/state.env"
  cat >"${state_file}" <<'EOF'
GITEA_BASE_URL=http://127.0.0.1:3000
GITEA_ADMIN_USERNAME=root
GITEA_ADMIN_PASSWORD=test-password
GITEA_ADMIN_EMAIL=declarest-e2e@example.local
GITEA_REPO_OWNER=root
GITEA_REPO_NAME=declarest-e2e
GITEA_REPO_PATH=root/declarest-e2e
EOF

  local compose_capture="${tmp}/compose-command.log"
  local output status
  set +e
  output=$(
    PATH="${bin_dir}:${PATH}" \
      E2E_DIR="${fake_e2e}" \
      E2E_COMPONENT_STATE_FILE="${state_file}" \
      E2E_COMPONENT_CONNECTION='local' \
      E2E_PLATFORM='compose' \
      E2E_COMPONENT_PROJECT_NAME='declarest-test-gitea' \
      E2E_COMPONENT_COMPOSE_FILE='/tmp/compose.yaml' \
      E2E_TEST_CAPTURE_FILE="${compose_capture}" \
      bash "${E2E_SCRIPT_DIR}/components/git-provider/gitea/scripts/configure-auth.sh" 2>&1
  )
  status=$?
  set -e

  assert_status "${status}" "0"
  assert_eq "${output}" "" "expected script output to stay empty when stubs succeed"

  local captured
  captured=$(cat "${compose_capture}")
  assert_contains "${captured}" "exec -T --user git gitea /usr/local/bin/gitea admin user create"
}

test_compose_gitea_admin_exec_invokes_gitea_binary
