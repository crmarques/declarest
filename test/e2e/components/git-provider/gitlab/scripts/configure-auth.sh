#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

wait_attempts=${DECLAREST_E2E_GITLAB_HEALTH_ATTEMPTS:-${E2E_GITLAB_HEALTH_ATTEMPTS:-180}}
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
    curl -fsS -H "Authorization: Bearer ${oauth_token}" "${url}"
    return 0
  fi

  curl -fsS -u "root:${GITLAB_ROOT_PASSWORD}" "${url}"
}

gitlab_api_post() {
  local url=$1
  shift
  if [[ "${api_auth_mode}" == 'bearer' ]]; then
    curl -fsS -X POST -H "Authorization: Bearer ${oauth_token}" "${url}" "$@"
    return 0
  fi

  curl -fsS -X POST -u "root:${GITLAB_ROOT_PASSWORD}" "${url}" "$@"
}

branch_name=${GIT_REMOTE_BRANCH:-main}

project_response=$(gitlab_api_get "${GITLAB_BASE_URL}/api/v4/projects?search=${GITLAB_PROJECT_NAME}")
project_id=$(jq -r ".[] | select(.path_with_namespace == \"${GITLAB_PROJECT_PATH}\") | .id" <<<"${project_response}" 2>/dev/null | head -n 1 || true)

if [[ -z "${project_id}" ]]; then
  gitlab_api_post "${GITLAB_BASE_URL}/api/v4/projects" \
    --data-urlencode "name=${GITLAB_PROJECT_NAME}" \
    --data-urlencode 'visibility=private' \
    --data-urlencode 'initialize_with_readme=true' \
    --data-urlencode "default_branch=${branch_name}" >/dev/null
fi

project_response=$(gitlab_api_get "${GITLAB_BASE_URL}/api/v4/projects?search=${GITLAB_PROJECT_NAME}")
project_id=$(jq -r ".[] | select(.path_with_namespace == \"${GITLAB_PROJECT_PATH}\") | .id" <<<"${project_response}" 2>/dev/null | head -n 1 || true)

if [[ -z "${project_id}" ]]; then
  printf 'failed to provision gitlab project %s\n' "${GITLAB_PROJECT_PATH}" >&2
  exit 1
fi
