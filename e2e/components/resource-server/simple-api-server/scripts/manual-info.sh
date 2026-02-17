#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

printf 'Base URL: %s\n' "${SIMPLE_API_SERVER_BASE_URL}"
printf 'Basic Auth Enabled: %s\n' "${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}"
printf 'OAuth2 Enabled: %s\n' "${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}"
printf 'mTLS Enabled: %s\n' "${SIMPLE_API_SERVER_ENABLE_MTLS:-false}"

if [[ "${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}" == 'true' ]]; then
  printf 'Basic Auth Username: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}"
  printf 'Basic Auth Password: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}"
fi

if [[ "${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}" == 'true' ]]; then
  printf 'Token URL: %s\n' "${SIMPLE_API_SERVER_TOKEN_URL}"
  printf 'Client ID: %s\n' "${SIMPLE_API_SERVER_CLIENT_ID}"
  printf 'Client Secret: %s\n' "${SIMPLE_API_SERVER_CLIENT_SECRET}"
fi

if [[ "${SIMPLE_API_SERVER_ENABLE_MTLS:-false}" == 'true' ]]; then
  printf 'mTLS Certs Host Dir: %s\n' "${SIMPLE_API_SERVER_CERTS_HOST_DIR:-}"
  printf 'mTLS CA Cert (Host): %s\n' "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST:-}"
  printf 'mTLS Client Cert (Host): %s\n' "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST:-}"
  printf 'mTLS Client Key (Host): %s\n' "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST:-}"
fi
