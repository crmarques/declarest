#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

attempts=60
for ((i = 1; i <= attempts; i++)); do
  if curl -fsS "${VAULT_ADDRESS}/v1/sys/health" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

curl -fsS \
  -X POST \
  -H "X-Vault-Token: ${VAULT_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"type":"kv-v2"}' \
  "${VAULT_ADDRESS}/v1/sys/mounts/${VAULT_MOUNT}" >/dev/null 2>&1 || true
