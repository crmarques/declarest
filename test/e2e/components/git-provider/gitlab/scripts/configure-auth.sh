#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

wait_attempts=${DECLAREST_E2E_GITLAB_HEALTH_ATTEMPTS:-${E2E_GITLAB_HEALTH_ATTEMPTS:-120}}
wait_interval_seconds=${DECLAREST_E2E_GITLAB_HEALTH_INTERVAL_SECONDS:-${E2E_GITLAB_HEALTH_INTERVAL_SECONDS:-5}}

if ! [[ "${wait_attempts}" =~ ^[0-9]+$ ]] || ((wait_attempts <= 0)); then
  printf 'invalid gitlab health attempts value: %s\n' "${wait_attempts}" >&2
  exit 1
fi

if ! [[ "${wait_interval_seconds}" =~ ^[0-9]+$ ]] || ((wait_interval_seconds <= 0)); then
  printf 'invalid gitlab health interval value: %s\n' "${wait_interval_seconds}" >&2
  exit 1
fi

wait_for() {
  local url=$1
  local attempts=${2:-${wait_attempts}}
  local interval_seconds=${3:-${wait_interval_seconds}}
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi

    if ((i % 12 == 0)); then
      printf 'gitlab readiness pending (%d/%d): %s\n' "${i}" "${attempts}" "${url}" >&2
    fi

    sleep "${interval_seconds}"
  done

  printf 'gitlab did not become ready after %d attempts (%ss interval): %s\n' "${attempts}" "${interval_seconds}" "${url}" >&2
  return 1
}

wait_for_git_receive_pack() {
  local url="${GITLAB_BASE_URL}/${GITLAB_PROJECT_PATH}.git/info/refs?service=git-receive-pack"
  local i

  for ((i = 1; i <= wait_attempts; i++)); do
    if curl -fsS -u "root:${GITLAB_ROOT_PASSWORD}" "${url}" >/dev/null 2>&1; then
      return 0
    fi

    if ((i % 12 == 0)); then
      printf 'gitlab receive-pack pending (%d/%d): %s\n' "${i}" "${wait_attempts}" "${url}" >&2
    fi

    sleep "${wait_interval_seconds}"
  done

  printf 'gitlab receive-pack did not become ready after %d attempts (%ss interval): %s\n' "${wait_attempts}" "${wait_interval_seconds}" "${url}" >&2
  return 1
}

gitlab_should_retry_api_status() {
  local status=$1
  local stderr_output=$2

  case "${status}" in
    7|52|56)
      return 0
      ;;
    22)
      if [[ -z "${stderr_output}" ]]; then
        return 0
      fi
      if grep -Eq 'The requested URL returned error: 50[234]' <<<"${stderr_output}"; then
        return 0
      fi
      ;;
  esac

  return 1
}

gitlab_api_retry_curl() {
  local target=$1
  shift

  local attempts=${wait_attempts}
  local interval_seconds=${wait_interval_seconds}
  local stderr_file
  local response=''
  local stderr_output=''
  local status=0
  local i

  stderr_file=$(mktemp)
  for ((i = 1; i <= attempts; i++)); do
    : >"${stderr_file}"

    set +e
    response=$("$@" 2>"${stderr_file}")
    status=$?
    set -e

    if ((status == 0)); then
      rm -f "${stderr_file}"
      printf '%s' "${response}"
      return 0
    fi

    stderr_output=$(<"${stderr_file}")
    if ! gitlab_should_retry_api_status "${status}" "${stderr_output}"; then
      rm -f "${stderr_file}"
      if [[ -n "${stderr_output}" ]]; then
        printf '%s\n' "${stderr_output}" >&2
      fi
      return "${status}"
    fi

    if ((i % 12 == 0)); then
      printf 'gitlab api pending (%d/%d): %s\n' "${i}" "${attempts}" "${target}" >&2
    fi

    sleep "${interval_seconds}"
  done

  rm -f "${stderr_file}"
  if [[ -n "${stderr_output}" ]]; then
    printf '%s\n' "${stderr_output}" >&2
  fi
  printf 'gitlab api did not become ready after %d attempts (%ss interval): %s\n' "${attempts}" "${interval_seconds}" "${target}" >&2
  return 1
}

wait_for "${GITLAB_BASE_URL}/users/sign_in"

oauth_token=''
for _ in $(seq 1 24); do
  oauth_response=$(
    curl -fsS \
      -X POST "${GITLAB_BASE_URL}/oauth/token" \
      --data-urlencode 'grant_type=password' \
      --data-urlencode 'username=root' \
      --data-urlencode "password=${GITLAB_ROOT_PASSWORD}" 2>/dev/null || true
  )
  oauth_token=$(jq -r '.access_token // empty' <<<"${oauth_response}" 2>/dev/null || true)
  if [[ -n "${oauth_token}" ]]; then
    break
  fi
  sleep 5
done

api_auth_mode='bearer'
if [[ -z "${oauth_token}" ]]; then
  # Fallback for images/configurations where password grant is unavailable.
  api_auth_mode='basic'
  printf '[WARN] gitlab oauth token bootstrap unavailable; falling back to basic auth for project bootstrap\n' >&2
fi

gitlab_api_get() {
  local url=$1
  if [[ "${api_auth_mode}" == 'bearer' ]]; then
    gitlab_api_retry_curl "${url}" curl -fsS -H "Authorization: Bearer ${oauth_token}" "${url}"
    return 0
  fi

  gitlab_api_retry_curl "${url}" curl -fsS -u "root:${GITLAB_ROOT_PASSWORD}" "${url}"
}

gitlab_api_post() {
  local url=$1
  shift
  if [[ "${api_auth_mode}" == 'bearer' ]]; then
    gitlab_api_retry_curl "${url}" curl -fsS -X POST -H "Authorization: Bearer ${oauth_token}" "${url}" "$@"
    return 0
  fi

  gitlab_api_retry_curl "${url}" curl -fsS -X POST -u "root:${GITLAB_ROOT_PASSWORD}" "${url}" "$@"
}

gitlab_api_put() {
  local url=$1
  shift
  if [[ "${api_auth_mode}" == 'bearer' ]]; then
    gitlab_api_retry_curl "${url}" curl -fsS -X PUT -H "Authorization: Bearer ${oauth_token}" "${url}" "$@"
    return 0
  fi

  gitlab_api_retry_curl "${url}" curl -fsS -X PUT -u "root:${GITLAB_ROOT_PASSWORD}" "${url}" "$@"
}

branch_name=${GIT_REMOTE_BRANCH:-main}

project_response=$(gitlab_api_get "${GITLAB_BASE_URL}/api/v4/projects?search=${GITLAB_PROJECT_NAME}")
project_id=$(jq -r ".[] | select(.path_with_namespace == \"${GITLAB_PROJECT_PATH}\") | .id" <<<"${project_response}" 2>/dev/null | head -n 1 || true)

if [[ -z "${project_id}" ]]; then
  gitlab_api_post "${GITLAB_BASE_URL}/api/v4/projects" \
    --data-urlencode "name=${GITLAB_PROJECT_NAME}" \
    --data-urlencode 'visibility=private' \
    --data-urlencode "default_branch=${branch_name}" >/dev/null
fi

project_response=$(gitlab_api_get "${GITLAB_BASE_URL}/api/v4/projects?search=${GITLAB_PROJECT_NAME}")
project_id=$(jq -r ".[] | select(.path_with_namespace == \"${GITLAB_PROJECT_PATH}\") | .id" <<<"${project_response}" 2>/dev/null | head -n 1 || true)

if [[ -z "${project_id}" ]]; then
  printf 'failed to provision gitlab project %s\n' "${GITLAB_PROJECT_PATH}" >&2
  exit 1
fi

wait_for_git_receive_pack

webhook_url=${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL:-}
webhook_secret=${E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET:-}
webhook_provider=${E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER:-}
if [[ -n "${webhook_url}" || -n "${webhook_secret}" || -n "${webhook_provider}" ]]; then
  if [[ "${webhook_provider}" != 'gitlab' ]]; then
    exit 0
  fi
  if [[ -z "${webhook_url}" || -z "${webhook_secret}" ]]; then
    printf 'operator repository webhook config for gitlab requires URL and secret\n' >&2
    exit 1
  fi

  gitlab_api_put "${GITLAB_BASE_URL}/api/v4/application/settings" \
    --data 'allow_local_requests_from_web_hooks_and_services=true' >/dev/null

  hooks_url="${GITLAB_BASE_URL}/api/v4/projects/${project_id}/hooks"
  hooks_response=$(gitlab_api_get "${hooks_url}")
  hook_id=$(jq -r --arg url "${webhook_url}" '.[] | select((.url // "") == $url) | .id' <<<"${hooks_response}" | head -n 1 || true)

  if [[ -n "${hook_id}" && "${hook_id}" != 'null' ]]; then
    gitlab_api_put "${hooks_url}/${hook_id}" \
      --data-urlencode "url=${webhook_url}" \
      --data "push_events=true" \
      --data-urlencode "token=${webhook_secret}" \
      --data "enable_ssl_verification=false" >/dev/null
  else
    gitlab_api_post "${hooks_url}" \
      --data-urlencode "url=${webhook_url}" \
      --data "push_events=true" \
      --data-urlencode "token=${webhook_secret}" \
      --data "enable_ssl_verification=false" >/dev/null
  fi
fi
