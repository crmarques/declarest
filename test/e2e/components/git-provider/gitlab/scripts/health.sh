#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

attempts=${DECLAREST_E2E_GITLAB_HEALTH_ATTEMPTS:-${E2E_GITLAB_HEALTH_ATTEMPTS:-180}}
interval_seconds=${DECLAREST_E2E_GITLAB_HEALTH_INTERVAL_SECONDS:-${E2E_GITLAB_HEALTH_INTERVAL_SECONDS:-5}}

if ! [[ "${attempts}" =~ ^[0-9]+$ ]] || ((attempts <= 0)); then
  printf 'invalid gitlab health attempts value: %s\n' "${attempts}" >&2
  exit 1
fi

if ! [[ "${interval_seconds}" =~ ^[0-9]+$ ]] || ((interval_seconds <= 0)); then
  printf 'invalid gitlab health interval value: %s\n' "${interval_seconds}" >&2
  exit 1
fi

for ((i = 1; i <= attempts; i++)); do
  if curl -fsS "${GITLAB_BASE_URL}/users/sign_in" >/dev/null 2>&1; then
    exit 0
  fi

  if ((i % 12 == 0)); then
    printf 'gitlab healthcheck pending (%d/%d): %s/users/sign_in\n' "${i}" "${attempts}" "${GITLAB_BASE_URL}" >&2
  fi

  sleep "${interval_seconds}"
done

printf 'gitlab healthcheck failed after %d attempts (%ss interval)\n' "${attempts}" "${interval_seconds}" >&2
exit 1
