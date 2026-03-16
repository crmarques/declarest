#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

attempts=${DECLAREST_E2E_RUNDECK_HEALTH_ATTEMPTS:-${E2E_RUNDECK_HEALTH_ATTEMPTS:-150}}
interval_seconds=${DECLAREST_E2E_RUNDECK_HEALTH_INTERVAL_SECONDS:-${E2E_RUNDECK_HEALTH_INTERVAL_SECONDS:-1}}

if ! [[ "${attempts}" =~ ^[0-9]+$ ]] || ((attempts <= 0)); then
  printf 'invalid rundeck health attempts value: %s\n' "${attempts}" >&2
  exit 1
fi

if ! [[ "${interval_seconds}" =~ ^[0-9]+$ ]] || ((interval_seconds <= 0)); then
  printf 'invalid rundeck health interval value: %s\n' "${interval_seconds}" >&2
  exit 1
fi

for ((i = 1; i <= attempts; i++)); do
  if curl -fsS "${RUNDECK_BASE_URL}/user/login" >/dev/null 2>&1; then
    exit 0
  fi

  if ((i % 15 == 0)); then
    printf 'rundeck healthcheck pending (%d/%d): %s/user/login\n' "${i}" "${attempts}" "${RUNDECK_BASE_URL}" >&2
  fi

  sleep "${interval_seconds}"
done

printf 'rundeck healthcheck failed after %d attempts (%ss interval)\n' "${attempts}" "${interval_seconds}" >&2
exit 1
