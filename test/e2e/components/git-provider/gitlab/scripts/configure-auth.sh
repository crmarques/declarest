#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

wait_for() {
  local url=$1
  local attempts=${2:-120}
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 5
  done

  printf 'gitlab did not become ready: %s\n' "${url}" >&2
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

if [[ -z "${oauth_token}" ]]; then
  # Keep default basic-auth fallback when token bootstrap is unavailable.
  printf '[WARN] gitlab oauth token bootstrap unavailable; continuing with basic auth context only\n' >&2
  exit 0
fi

project_response=$(
  curl -fsS \
    -H "Authorization: Bearer ${oauth_token}" \
    "${GITLAB_BASE_URL}/api/v4/projects?search=${GITLAB_PROJECT_NAME}" 2>/dev/null || true
)
project_id=$(jq -r ".[] | select(.path_with_namespace == \"${GITLAB_PROJECT_PATH}\") | .id" <<<"${project_response}" 2>/dev/null | head -n 1 || true)

if [[ -z "${project_id}" ]]; then
  curl -fsS \
    -X POST "${GITLAB_BASE_URL}/api/v4/projects" \
    -H "Authorization: Bearer ${oauth_token}" \
    --data-urlencode "name=${GITLAB_PROJECT_NAME}" \
    --data-urlencode 'visibility=private' >/dev/null 2>&1 || true
fi
