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

  printf 'healthcheck failed for %s: %s\n' "${name}" "${url}" >&2
  return 1
}

wait_for 'keycloak' "${KEYCLOAK_BASE_URL}/realms/master"
wait_for 'resource-api' "${RESOURCE_API_BASE_URL}/healthz"
