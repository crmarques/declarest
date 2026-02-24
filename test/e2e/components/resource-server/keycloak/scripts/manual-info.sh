#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

base_url=${KEYCLOAK_BASE_URL:-}
[[ -n "${base_url}" ]] || exit 0

printf 'Admin Console: %s/admin/\n' "${base_url}"

if [[ -n "${KEYCLOAK_ADMIN_USER:-}" ]]; then
  printf 'Admin User: %s\n' "${KEYCLOAK_ADMIN_USER}"
fi

if [[ -n "${KEYCLOAK_ADMIN_PASSWORD:-}" ]]; then
  printf 'Admin Password: %s\n' "${KEYCLOAK_ADMIN_PASSWORD}"
fi

if [[ -n "${KEYCLOAK_REALM:-}" ]]; then
  printf 'Configured Realm: %s\n' "${KEYCLOAK_REALM}"
fi
