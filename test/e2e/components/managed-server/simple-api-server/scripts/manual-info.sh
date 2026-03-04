#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

printf 'Base URL: %s\n' "${SIMPLE_API_SERVER_BASE_URL}"

if [[ "${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}" == 'true' ]]; then
  printf 'Auth Mode: basic\n'
else
  printf 'Auth Mode: oauth2\n'
fi

printf 'mTLS: %s\n' "${SIMPLE_API_SERVER_ENABLE_MTLS:-false}"

if [[ "${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}" == 'true' ]]; then
  printf 'Username: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}"
  printf 'Password: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}"
fi

if [[ "${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}" == 'true' ]]; then
  printf 'Token URL: %s\n' "${SIMPLE_API_SERVER_TOKEN_URL}"
  printf 'Client ID: %s\n' "${SIMPLE_API_SERVER_CLIENT_ID}"
fi
