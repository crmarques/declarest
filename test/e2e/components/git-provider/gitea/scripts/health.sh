#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

attempts=${DECLAREST_E2E_GITEA_HEALTH_ATTEMPTS:-${E2E_GITEA_HEALTH_ATTEMPTS:-90}}
interval_seconds=${DECLAREST_E2E_GITEA_HEALTH_INTERVAL_SECONDS:-${E2E_GITEA_HEALTH_INTERVAL_SECONDS:-2}}

if ! [[ "${attempts}" =~ ^[0-9]+$ ]] || ((attempts <= 0)); then
  printf 'invalid gitea health attempts value: %s\n' "${attempts}" >&2
  exit 1
fi

if ! [[ "${interval_seconds}" =~ ^[0-9]+$ ]] || ((interval_seconds <= 0)); then
  printf 'invalid gitea health interval value: %s\n' "${interval_seconds}" >&2
  exit 1
fi

for ((i = 1; i <= attempts; i++)); do
  if curl -fsS "${GITEA_BASE_URL}/api/healthz" >/dev/null 2>&1 || curl -fsS "${GITEA_BASE_URL}/user/login" >/dev/null 2>&1; then
    exit 0
  fi

  if ((i % 10 == 0)); then
    printf 'gitea healthcheck pending (%d/%d): %s\n' "${i}" "${attempts}" "${GITEA_BASE_URL}" >&2
  fi

  sleep "${interval_seconds}"
done

printf 'gitea healthcheck failed after %d attempts (%ss interval)\n' "${attempts}" "${interval_seconds}" >&2
exit 1
