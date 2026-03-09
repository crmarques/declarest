#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_configure_auth_enables_local_requests_before_registering_gitlab_webhook() {
  local tmp
  tmp=$(new_temp_dir)

  local bin_dir="${tmp}/bin"
  mkdir -p "${bin_dir}"
  cat >"${bin_dir}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

method='GET'
url=''
for ((idx = 1; idx <= $#; idx++)); do
  arg=${!idx}
  case "${arg}" in
    -X)
      idx=$((idx + 1))
      method=${!idx}
      ;;
    http://*|https://*)
      url=${arg}
      ;;
  esac
done

printf '%s %s\n' "${method}" "${url}" >> "${E2E_TEST_CAPTURE_FILE}"

case "${url}" in
  */users/sign_in)
    exit 0
    ;;
  */oauth/token)
    printf '{"access_token":"test-token"}\n'
    exit 0
    ;;
  */api/v4/projects?search=*)
    printf '[{"id":1,"path_with_namespace":"root/declarest-e2e"}]\n'
    exit 0
    ;;
  */api/v4/application/settings)
    printf '{}\n'
    exit 0
    ;;
  */api/v4/projects/1/hooks)
    if [[ "${method}" == 'GET' ]]; then
      printf '[]\n'
    else
      printf '{}\n'
    fi
    exit 0
    ;;
  *)
    printf '{}\n'
    exit 0
    ;;
esac
EOF
  chmod +x "${bin_dir}/curl"

  local state_file="${tmp}/state.env"
  cat >"${state_file}" <<'EOF'
GITLAB_BASE_URL=http://127.0.0.1:3000
GITLAB_ROOT_PASSWORD=test-password
GITLAB_PROJECT_NAME=declarest-e2e
GITLAB_PROJECT_PATH=root/declarest-e2e
GIT_REMOTE_BRANCH=main
EOF

  local curl_capture="${tmp}/curl.log"
  local output status
  set +e
  output=$(
    PATH="${bin_dir}:${PATH}" \
      E2E_COMPONENT_STATE_FILE="${state_file}" \
      E2E_COMPONENT_CONNECTION='local' \
      E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER='gitlab' \
      E2E_OPERATOR_REPOSITORY_WEBHOOK_URL='http://declarest-operator-repo-webhook.default.svc.cluster.local:18082/webhooks/repository/default/declarest-e2e-repository' \
      E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET='hook-secret' \
      E2E_TEST_CAPTURE_FILE="${curl_capture}" \
      bash "${E2E_SCRIPT_DIR}/components/git-provider/gitlab/scripts/configure-auth.sh" 2>&1
  )
  status=$?
  set -e

  assert_status "${status}" "0"
  assert_eq "${output}" "" "expected script output to stay empty when webhook setup succeeds"

  local captured
  captured=$(cat "${curl_capture}")
  assert_contains "${captured}" "PUT http://127.0.0.1:3000/api/v4/application/settings"
  assert_contains "${captured}" "GET http://127.0.0.1:3000/api/v4/projects/1/hooks"
  assert_contains "${captured}" "POST http://127.0.0.1:3000/api/v4/projects/1/hooks"

  local settings_line hooks_post_line
  settings_line=$(printf '%s\n' "${captured}" | grep -n 'PUT http://127.0.0.1:3000/api/v4/application/settings' | head -n 1 | cut -d: -f1)
  hooks_post_line=$(printf '%s\n' "${captured}" | grep -n 'POST http://127.0.0.1:3000/api/v4/projects/1/hooks' | head -n 1 | cut -d: -f1)
  if [[ -z "${settings_line}" || -z "${hooks_post_line}" ]] || ((settings_line >= hooks_post_line)); then
    fail 'expected GitLab local-request setting to be updated before webhook creation'
  fi
}

test_configure_auth_enables_local_requests_before_registering_gitlab_webhook

test_configure_auth_waits_for_git_receive_pack_before_exit() {
  local tmp
  tmp=$(new_temp_dir)

  local bin_dir="${tmp}/bin"
  mkdir -p "${bin_dir}"
  local receive_pack_counter="${tmp}/receive-pack-count"
  printf '0\n' >"${receive_pack_counter}"

  cat >"${bin_dir}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

method='GET'
url=''
for ((idx = 1; idx <= $#; idx++)); do
  arg=${!idx}
  case "${arg}" in
    -X)
      idx=$((idx + 1))
      method=${!idx}
      ;;
    http://*|https://*)
      url=${arg}
      ;;
  esac
done

printf '%s %s\n' "${method}" "${url}" >> "${E2E_TEST_CAPTURE_FILE}"

case "${url}" in
  */users/sign_in)
    exit 0
    ;;
  */oauth/token)
    printf '{"access_token":"test-token"}\n'
    exit 0
    ;;
  */api/v4/projects?search=*)
    printf '[{"id":1,"path_with_namespace":"root/declarest-e2e"}]\n'
    exit 0
    ;;
  */root/declarest-e2e.git/info/refs?service=git-receive-pack)
    count=$(cat "${E2E_TEST_RECEIVE_PACK_COUNTER}")
    count=$((count + 1))
    printf '%s\n' "${count}" > "${E2E_TEST_RECEIVE_PACK_COUNTER}"
    if ((count < 2)); then
      exit 22
    fi
    printf '001f# service=git-receive-pack\n0000'
    exit 0
    ;;
  *)
    printf '{}\n'
    exit 0
    ;;
esac
EOF
  chmod +x "${bin_dir}/curl"

  local state_file="${tmp}/state.env"
  cat >"${state_file}" <<'EOF'
GITLAB_BASE_URL=http://127.0.0.1:3000
GITLAB_ROOT_PASSWORD=test-password
GITLAB_PROJECT_NAME=declarest-e2e
GITLAB_PROJECT_PATH=root/declarest-e2e
GIT_REMOTE_BRANCH=main
EOF

  local curl_capture="${tmp}/curl.log"
  local output status
  set +e
  output=$(
    PATH="${bin_dir}:${PATH}" \
      E2E_COMPONENT_STATE_FILE="${state_file}" \
      E2E_COMPONENT_CONNECTION='local' \
      DECLAREST_E2E_GITLAB_HEALTH_ATTEMPTS='3' \
      DECLAREST_E2E_GITLAB_HEALTH_INTERVAL_SECONDS='1' \
      E2E_TEST_CAPTURE_FILE="${curl_capture}" \
      E2E_TEST_RECEIVE_PACK_COUNTER="${receive_pack_counter}" \
      bash "${E2E_SCRIPT_DIR}/components/git-provider/gitlab/scripts/configure-auth.sh" 2>&1
  )
  status=$?
  set -e

  assert_status "${status}" "0"
  assert_eq "${output}" "" "expected script output to stay empty when git receive-pack becomes ready"
  assert_eq "$(cat "${receive_pack_counter}")" "2" "expected git receive-pack readiness to be retried once"

  local captured
  captured=$(cat "${curl_capture}")
  assert_contains "${captured}" "GET http://127.0.0.1:3000/root/declarest-e2e.git/info/refs?service=git-receive-pack"
}

test_configure_auth_waits_for_git_receive_pack_before_exit
