#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

attempts=${DECLAREST_E2E_HAPROXY_HEALTH_ATTEMPTS:-${E2E_HAPROXY_HEALTH_ATTEMPTS:-120}}
interval_seconds=${DECLAREST_E2E_HAPROXY_HEALTH_INTERVAL_SECONDS:-${E2E_HAPROXY_HEALTH_INTERVAL_SECONDS:-1}}

if ! [[ "${attempts}" =~ ^[0-9]+$ ]] || ((attempts <= 0)); then
  printf 'invalid haproxy health attempts value: %s\n' "${attempts}" >&2
  exit 1
fi

if ! [[ "${interval_seconds}" =~ ^[0-9]+$ ]] || ((interval_seconds <= 0)); then
  printf 'invalid haproxy health interval value: %s\n' "${interval_seconds}" >&2
  exit 1
fi

probe_url="${HAPROXY_BASE_URL%/}/v3/info"

for ((i = 1; i <= attempts; i++)); do
  if curl -fsS -u "${HAPROXY_ADMIN_USER}:${HAPROXY_ADMIN_PASSWORD}" "${probe_url}" >/dev/null 2>&1; then
    exit 0
  fi

  if ((i % 15 == 0)); then
    printf 'haproxy healthcheck pending (%d/%d): %s\n' "${i}" "${attempts}" "${probe_url}" >&2
  fi

  sleep "${interval_seconds}"
done

printf 'haproxy healthcheck failed after %d attempts (%ss interval)\n' "${attempts}" "${interval_seconds}" >&2
exit 1
