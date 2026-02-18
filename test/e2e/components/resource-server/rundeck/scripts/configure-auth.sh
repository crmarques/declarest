#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

wait_for() {
  local url=$1
  local attempts=${2:-180}
  local delay=${3:-2}
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay}"
  done

  printf 'rundeck did not become ready: %s\n' "${url}" >&2
  return 1
}

wait_for "${RUNDECK_BASE_URL}/user/login"

api_version="${RUNDECK_API_VERSION:-45}"
auth_header="${RUNDECK_AUTH_HEADER:-X-Rundeck-Auth-Token}"
token=''
for _ in $(seq 1 40); do
  response=$(
    curl -fsS \
      -X POST "${RUNDECK_BASE_URL}/api/${api_version}/tokens/admin" \
      -u "${RUNDECK_ADMIN_USER}:${RUNDECK_ADMIN_PASSWORD}" \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json' \
      -d '{"roles":["admin"],"duration":"24h"}' 2>/dev/null || true
  )
  token=$(jq -r '.token // empty' <<<"${response}" 2>/dev/null || true)
  if [[ -n "${token}" ]]; then
    break
  fi
  sleep 2
done

if [[ -n "${token}" ]]; then
  e2e_write_state_value "${E2E_COMPONENT_STATE_FILE}" RUNDECK_API_TOKEN "${token}"
  e2e_write_state_value "${E2E_COMPONENT_STATE_FILE}" RUNDECK_AUTH_MODE "token"
  e2e_write_state_value "${E2E_COMPONENT_STATE_FILE}" RUNDECK_AUTH_HEADER "${auth_header}"
  exit 0
fi

e2e_warn 'rundeck API token bootstrap unavailable; keeping basic-auth context for local rundeck'
