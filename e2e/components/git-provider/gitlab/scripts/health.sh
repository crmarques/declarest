#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

attempts=120
for ((i = 1; i <= attempts; i++)); do
  if curl -fsS "${GITLAB_BASE_URL}/users/sign_in" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 5
done

printf 'gitlab healthcheck failed after %d attempts\n' "${attempts}" >&2
exit 1
