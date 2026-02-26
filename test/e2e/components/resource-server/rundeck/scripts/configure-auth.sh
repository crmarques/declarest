#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

selected_auth_type=${E2E_RESOURCE_SERVER_AUTH_TYPE:-custom-header}
case "${selected_auth_type}" in
  basic)
    e2e_write_state_value "${E2E_COMPONENT_STATE_FILE}" RUNDECK_AUTH_MODE "basic"
    exit 0
    ;;
  custom-header) ;;
  *)
    e2e_die "resource-server rundeck does not support auth-type ${selected_auth_type} (supported: basic, custom-header)"
    exit 1
    ;;
esac

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

rundeck_create_api_token_with_session() {
  local base_url=$1
  local api_version=$2
  local username=$3
  local password=$4
  local cookie_jar
  local login_headers
  local response
  local rc

  cookie_jar=$(mktemp) || return 1
  login_headers=$(mktemp) || {
    rm -f "${cookie_jar}" || true
    return 1
  }

  if ! curl -fsS -c "${cookie_jar}" "${base_url}/user/login" >/dev/null; then
    rm -f "${cookie_jar}" "${login_headers}" || true
    return 1
  fi

  if ! curl -sS -o /dev/null -D "${login_headers}" \
    -b "${cookie_jar}" -c "${cookie_jar}" \
    -X POST "${base_url}/j_security_check" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "j_username=${username}" \
    --data-urlencode "j_password=${password}"; then
    rm -f "${cookie_jar}" "${login_headers}" || true
    return 1
  fi

  if grep -qiE '^Location: .*/user/error([[:space:]]|$)' "${login_headers}"; then
    rm -f "${cookie_jar}" "${login_headers}" || true
    return 1
  fi

  response=$(
    curl -fsS \
      -b "${cookie_jar}" \
      -X POST "${base_url}/api/${api_version}/tokens/${username}" \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json' \
      -d '{"roles":["admin"],"duration":"24h"}'
  )
  rc=$?
  rm -f "${cookie_jar}" "${login_headers}" || true
  ((rc == 0)) || return "${rc}"

  printf '%s\n' "${response}"
}

wait_for "${RUNDECK_BASE_URL}/user/login"

api_version="${RUNDECK_API_VERSION:-45}"
auth_header="${RUNDECK_AUTH_HEADER:-X-Rundeck-Auth-Token}"
token=''
for _ in $(seq 1 40); do
  response=$(
    rundeck_create_api_token_with_session \
      "${RUNDECK_BASE_URL}" \
      "${api_version}" \
      "${RUNDECK_ADMIN_USER}" \
      "${RUNDECK_ADMIN_PASSWORD}" 2>/dev/null || true
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

e2e_die 'rundeck API token bootstrap unavailable; auth-type custom-header cannot be configured for local rundeck'
exit 1
