#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

wait_attempts=${DECLAREST_E2E_GITEA_HEALTH_ATTEMPTS:-${E2E_GITEA_HEALTH_ATTEMPTS:-90}}
wait_interval_seconds=${DECLAREST_E2E_GITEA_HEALTH_INTERVAL_SECONDS:-${E2E_GITEA_HEALTH_INTERVAL_SECONDS:-2}}

if ! [[ "${wait_attempts}" =~ ^[0-9]+$ ]] || ((wait_attempts <= 0)); then
  printf 'invalid gitea health attempts value: %s\n' "${wait_attempts}" >&2
  exit 1
fi

if ! [[ "${wait_interval_seconds}" =~ ^[0-9]+$ ]] || ((wait_interval_seconds <= 0)); then
  printf 'invalid gitea health interval value: %s\n' "${wait_interval_seconds}" >&2
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

    if ((i % 10 == 0)); then
      printf 'gitea readiness pending (%d/%d): %s\n' "${i}" "${attempts}" "${url}" >&2
    fi

    sleep "${interval_seconds}"
  done

  printf 'gitea did not become ready after %d attempts (%ss interval): %s\n' "${attempts}" "${interval_seconds}" "${url}" >&2
  return 1
}

gitea_compose_exec() {
  e2e_compose_cmd -f "${E2E_COMPONENT_DIR}/compose.yaml" -p "${E2E_COMPONENT_PROJECT_NAME}" exec -T --user git gitea "$@"
}

wait_for "${GITEA_BASE_URL}/user/login"

if ! curl -fsS "${GITEA_BASE_URL}/api/v1/users/${GITEA_ADMIN_USERNAME}" >/dev/null 2>&1; then
  gitea_compose_exec /usr/local/bin/gitea admin user create \
    --username "${GITEA_ADMIN_USERNAME}" \
    --password "${GITEA_ADMIN_PASSWORD}" \
    --email "${GITEA_ADMIN_EMAIL}" \
    --admin \
    --must-change-password=false >/dev/null
fi

repo_url="${GITEA_BASE_URL}/api/v1/repos/${GITEA_REPO_OWNER}/${GITEA_REPO_NAME}"
if ! curl -fsS -u "${GITEA_ADMIN_USERNAME}:${GITEA_ADMIN_PASSWORD}" "${repo_url}" >/dev/null 2>&1; then
  create_payload=$(printf '{"name":"%s","private":true,"auto_init":true,"default_branch":"%s"}' "${GITEA_REPO_NAME}" "${GIT_REMOTE_BRANCH:-main}")

  curl -fsS \
    -X POST "${GITEA_BASE_URL}/api/v1/user/repos" \
    -H 'Content-Type: application/json' \
    -u "${GITEA_ADMIN_USERNAME}:${GITEA_ADMIN_PASSWORD}" \
    -d "${create_payload}" >/dev/null
fi

if ! curl -fsS -u "${GITEA_ADMIN_USERNAME}:${GITEA_ADMIN_PASSWORD}" "${repo_url}" >/dev/null 2>&1; then
  printf 'failed to provision gitea repository %s\n' "${GITEA_REPO_PATH}" >&2
  exit 1
fi
