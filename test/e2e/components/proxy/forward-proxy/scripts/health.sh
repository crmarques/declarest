#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

wait_for_proxy() {
  local attempts=${1:-30}
  local delay=${2:-1}
  local i

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "http://127.0.0.1:${PROXY_HOST_PORT}/__health" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay}"
  done

  return 1
}

wait_for_proxy 30 1 >/dev/null
