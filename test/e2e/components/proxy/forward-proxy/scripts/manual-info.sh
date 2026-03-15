#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

printf 'Proxy URL: %s\n' "${PROXY_HTTP_URL:-n/a}"
printf 'Proxy auth type: %s\n' "${PROXY_AUTH_TYPE:-none}"
printf 'Proxy access log: %s\n' "${PROXY_ACCESS_LOG:-n/a}"

if [[ -n "${PROXY_PROMPT_HELPER_FILE:-}" ]]; then
  printf 'Prompt helper: source %s\n' "${PROXY_PROMPT_HELPER_FILE}"
fi
