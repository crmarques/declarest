#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

printf 'Proxy URL: %s\n' "${PROXY_HTTP_URL:-n/a}"
printf 'Proxy auth type: %s\n' "${PROXY_AUTH_TYPE:-none}"
printf 'Proxy access log: %s\n' "${PROXY_ACCESS_LOG:-n/a}"

if [[ "${PROXY_AUTH_TYPE:-none}" == 'prompt' ]]; then
  if [[ -n "${PROXY_AUTH_USERNAME:-}" ]]; then
    printf 'Proxy auth username: %s\n' "${PROXY_AUTH_USERNAME}"
  fi
  if [[ -n "${PROXY_AUTH_PASSWORD:-}" ]]; then
    printf 'Proxy auth password: %s\n' "${PROXY_AUTH_PASSWORD}"
  fi
fi
