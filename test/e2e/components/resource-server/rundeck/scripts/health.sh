#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

attempts=180
for ((i = 1; i <= attempts; i++)); do
  if curl -fsS "${RUNDECK_BASE_URL}/user/login" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 2
done

printf 'rundeck healthcheck failed\n' >&2
exit 1
