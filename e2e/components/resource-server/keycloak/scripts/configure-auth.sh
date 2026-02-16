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

keycloak_get_client_uuid() {
  local admin_token=$1
  local realm=$2
  local client_id=$3

  curl -fsS \
    -H "Authorization: Bearer ${admin_token}" \
    "${KEYCLOAK_BASE_URL}/admin/realms/${realm}/clients?clientId=${client_id}" \
    | jq -r '.[0].id // empty'
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
    | jq -r '.access_token // empty'
)

[[ -n "${admin_token}" ]] || {
  printf 'failed to obtain keycloak admin token\n' >&2
  exit 1
}

realm_status=$(curl -s -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${admin_token}" \
  "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}")

if [[ "${realm_status}" == '404' ]]; then
  realm_payload=$(jq -nc --arg realm "${KEYCLOAK_REALM}" '{realm:$realm,enabled:true}')
  curl -fsS \
    -X POST "${KEYCLOAK_BASE_URL}/admin/realms" \
    -H "Authorization: Bearer ${admin_token}" \
    -H 'Content-Type: application/json' \
    -d "${realm_payload}" >/dev/null
elif [[ "${realm_status}" != '200' ]]; then
  printf 'unexpected keycloak realm status for %s: %s\n' "${KEYCLOAK_REALM}" "${realm_status}" >&2
  exit 1
fi

existing_client_id=$(keycloak_get_client_uuid "${admin_token}" "${KEYCLOAK_REALM}" "${KEYCLOAK_CLIENT_ID}")
if [[ -n "${existing_client_id}" ]]; then
  curl -fsS \
    -X DELETE "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients/${existing_client_id}" \
    -H "Authorization: Bearer ${admin_token}" >/dev/null
fi

client_payload=$(jq -nc \
  --arg client_id "${KEYCLOAK_CLIENT_ID}" \
  --arg client_secret "${KEYCLOAK_CLIENT_SECRET}" \
  '{
    clientId:$client_id,
    enabled:true,
    serviceAccountsEnabled:true,
    publicClient:false,
    protocol:"openid-connect",
    secret:$client_secret,
    directAccessGrantsEnabled:true,
    standardFlowEnabled:false
  }')
curl -fsS \
  -X POST "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients" \
  -H "Authorization: Bearer ${admin_token}" \
  -H 'Content-Type: application/json' \
  -d "${client_payload}" >/dev/null

client_uuid=''
for _ in $(seq 1 30); do
  client_uuid=$(keycloak_get_client_uuid "${admin_token}" "${KEYCLOAK_REALM}" "${KEYCLOAK_CLIENT_ID}")
  if [[ -n "${client_uuid}" ]]; then
    break
  fi
  sleep 1
done

[[ -n "${client_uuid}" ]] || {
  printf 'failed to resolve keycloak client id for %s\n' "${KEYCLOAK_CLIENT_ID}" >&2
  exit 1
}

service_account_user_id=$(
  curl -fsS \
    -H "Authorization: Bearer ${admin_token}" \
    "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients/${client_uuid}/service-account-user" \
    | jq -r '.id // empty'
)
[[ -n "${service_account_user_id}" ]] || {
  printf 'failed to resolve service-account user for client %s\n' "${KEYCLOAK_CLIENT_ID}" >&2
  exit 1
}

# Master realm management relies on realm-level admin role.
realm_admin_realm_role=$(
  curl -fsS \
    -H "Authorization: Bearer ${admin_token}" \
    "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/roles/admin" 2>/dev/null || true
)
if [[ "$(jq -r '.name // empty' <<<"${realm_admin_realm_role}" 2>/dev/null || true)" == 'admin' ]]; then
  realm_role_mapping_payload=$(jq -c '[.]' <<<"${realm_admin_realm_role}")
  curl -fsS \
    -X POST "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/users/${service_account_user_id}/role-mappings/realm" \
    -H "Authorization: Bearer ${admin_token}" \
    -H 'Content-Type: application/json' \
    -d "${realm_role_mapping_payload}" >/dev/null
  exit 0
fi

# Fallback for non-master layouts where admin rights are modeled as client roles.
realm_management_client_id=$(keycloak_get_client_uuid "${admin_token}" "${KEYCLOAK_REALM}" 'realm-management')
if [[ -z "${realm_management_client_id}" ]]; then
  realm_management_client_id=$(keycloak_get_client_uuid "${admin_token}" "${KEYCLOAK_REALM}" 'master-realm')
fi
[[ -n "${realm_management_client_id}" ]] || {
  printf 'failed to resolve management client for realm %s\n' "${KEYCLOAK_REALM}" >&2
  exit 1
}

realm_admin_role=$(
  curl -fsS \
    -H "Authorization: Bearer ${admin_token}" \
    "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients/${realm_management_client_id}/roles/realm-admin" 2>/dev/null || true
)
if [[ "$(jq -r '.name // empty' <<<"${realm_admin_role}" 2>/dev/null || true)" != 'realm-admin' ]]; then
  realm_admin_role=$(
    curl -fsS \
      -H "Authorization: Bearer ${admin_token}" \
      "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/clients/${realm_management_client_id}/roles/manage-realm" 2>/dev/null || true
  )
fi
[[ -n "${realm_admin_role}" && "$(jq -r '.id // empty' <<<"${realm_admin_role}" 2>/dev/null || true)" != '' ]] || {
  printf 'failed to resolve management role for management client in realm %s\n' "${KEYCLOAK_REALM}" >&2
  exit 1
}

client_role_mapping_payload=$(jq -c '[.]' <<<"${realm_admin_role}")
curl -fsS \
  -X POST "${KEYCLOAK_BASE_URL}/admin/realms/${KEYCLOAK_REALM}/users/${service_account_user_id}/role-mappings/clients/${realm_management_client_id}" \
  -H "Authorization: Bearer ${admin_token}" \
  -H 'Content-Type: application/json' \
  -d "${client_role_mapping_payload}" >/dev/null
