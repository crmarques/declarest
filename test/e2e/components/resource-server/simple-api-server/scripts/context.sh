#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

fragment_file=${1:-${E2E_COMPONENT_CONTEXT_FRAGMENT:-}}
[[ -n "${fragment_file}" ]] || {
  printf 'missing context fragment output path\n' >&2
  exit 1
}

enable_basic_auth=${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}
enable_oauth2=${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}
enable_mtls=${SIMPLE_API_SERVER_ENABLE_MTLS:-false}

if [[ "${enable_basic_auth}" == 'true' && "${enable_oauth2}" == 'true' ]]; then
  printf 'simple-api-server context supports only one auth mode; both basic-auth and oauth2 are enabled\n' >&2
  exit 1
fi

{
  printf 'resource-server:\n'
  printf '  http:\n'
  printf '    base-url: %s\n' "${SIMPLE_API_SERVER_BASE_URL}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi

  if [[ "${enable_mtls}" == 'true' ]]; then
    if [[ -z "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST:-}" || -z "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST:-}" || -z "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST:-}" ]]; then
      printf 'simple-api-server mTLS context requires tls ca/client certificate host paths\n' >&2
      exit 1
    fi

    printf '    tls:\n'
    printf '      ca-cert-file: %s\n' "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST}"
    printf '      client-cert-file: %s\n' "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST}"
    printf '      client-key-file: %s\n' "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST}"
  fi

  printf '    auth:\n'
  if [[ "${enable_oauth2}" == 'true' ]]; then
    printf '      oauth2:\n'
    printf '        token-url: %s\n' "${SIMPLE_API_SERVER_TOKEN_URL}"
    printf '        grant-type: client_credentials\n'
    printf '        client-id: %s\n' "${SIMPLE_API_SERVER_CLIENT_ID}"
    printf '        client-secret: %s\n' "${SIMPLE_API_SERVER_CLIENT_SECRET}"

    if [[ -n "${SIMPLE_API_SERVER_SCOPE:-}" ]]; then
      printf '        scope: %s\n' "${SIMPLE_API_SERVER_SCOPE}"
    fi
    if [[ -n "${SIMPLE_API_SERVER_AUDIENCE:-}" ]]; then
      printf '        audience: %s\n' "${SIMPLE_API_SERVER_AUDIENCE}"
    fi
  elif [[ "${enable_basic_auth}" == 'true' ]]; then
    printf '      basic-auth:\n'
    printf '        username: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}"
    printf '        password: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}"
  else
    printf '      bearer-token:\n'
    printf '        token: simple-api-oauth2-disabled\n'
  fi
} >"${fragment_file}"
