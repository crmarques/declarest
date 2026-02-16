#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

wait_for() {
  local name=$1
  local url=$2
  local attempts=${3:-90}
  local delay=${4:-2}
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay}"
  done

  printf 'failed waiting for %s: %s\n' "${name}" "${url}" >&2
  return 1
}

wait_for 'keycloak' "${KEYCLOAK_BASE_URL}/realms/master"

admin_token=$(
  curl -fsS \
    -X POST "${KEYCLOAK_BASE_URL}/realms/master/protocol/openid-connect/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "grant_type=password" \
    --data-urlencode "client_id=admin-cli" \
    --data-urlencode "username=${KEYCLOAK_ADMIN_USER}" \
    --data-urlencode "password=${KEYCLOAK_ADMIN_PASSWORD}" \
    | jq -r '.access_token'
)

[[ -n "${admin_token}" && "${admin_token}" != 'null' ]] || {
  printf 'failed to obtain keycloak admin token\n' >&2
  exit 1
}

realm_status=$(curl -s -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${admin_token}" \
  "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}")

if [[ "${realm_status}" == '404' ]]; then
  curl -fsS \
    -X POST "${KEYCLOAK_BASE_URL}/admin/realms" \
    -H "Authorization: Bearer ${admin_token}" \
    -H 'Content-Type: application/json' \
    -d "{\"realm\":\"${KEYCLOAK_REALM}\",\"enabled\":true}" >/dev/null
fi

existing_client_id=$(
  curl -fsS \
    -H "Authorization: Bearer ${admin_token}" \
    "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients?clientId=${KEYCLOAK_CLIENT_ID}" \
    | jq -r '.[0].id // empty'
)

if [[ -n "${existing_client_id}" ]]; then
  curl -fsS \
    -X DELETE "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients/${existing_client_id}" \
    -H "Authorization: Bearer ${admin_token}" >/dev/null
fi

curl -fsS \
  -X POST "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients" \
  -H "Authorization: Bearer ${admin_token}" \
  -H 'Content-Type: application/json' \
  -d "{\"clientId\":\"${KEYCLOAK_CLIENT_ID}\",\"enabled\":true,\"serviceAccountsEnabled\":true,\"publicClient\":false,\"protocol\":\"openid-connect\",\"secret\":\"${KEYCLOAK_CLIENT_SECRET}\",\"directAccessGrantsEnabled\":true}" >/dev/null
