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
selected_auth_type=${E2E_MANAGED_SERVER_AUTH_TYPE:-oauth2}

if [[ "${enable_basic_auth}" == 'true' && "${enable_oauth2}" == 'true' ]]; then
  printf 'simple-api-server context supports only one auth mode; both basic-auth and oauth2 are enabled\n' >&2
  exit 1
fi

case "${selected_auth_type}" in
  none)
    if [[ "${enable_basic_auth}" != 'false' || "${enable_oauth2}" != 'false' ]]; then
      printf 'simple-api-server auth-type none requires both basic-auth and oauth2 to be disabled (got basic-auth=%s oauth2=%s)\n' "${enable_basic_auth}" "${enable_oauth2}" >&2
      exit 1
    fi
    ;;
  basic)
    if [[ "${enable_basic_auth}" != 'true' || "${enable_oauth2}" != 'false' ]]; then
      printf 'simple-api-server auth-type basic requires basic-auth=true and oauth2=false (got basic-auth=%s oauth2=%s)\n' "${enable_basic_auth}" "${enable_oauth2}" >&2
      exit 1
    fi
    ;;
  prompt)
    if [[ "${enable_basic_auth}" != 'true' || "${enable_oauth2}" != 'false' ]]; then
      printf 'simple-api-server auth-type prompt requires basic-auth=true and oauth2=false (got basic-auth=%s oauth2=%s)\n' "${enable_basic_auth}" "${enable_oauth2}" >&2
      exit 1
    fi
    ;;
  oauth2)
    if [[ "${enable_basic_auth}" != 'false' || "${enable_oauth2}" != 'true' ]]; then
      printf 'simple-api-server auth-type oauth2 requires basic-auth=false and oauth2=true (got basic-auth=%s oauth2=%s)\n' "${enable_basic_auth}" "${enable_oauth2}" >&2
      exit 1
    fi
    ;;
  custom-header)
    printf 'simple-api-server does not support managed-server auth-type custom-header\n' >&2
    exit 1
    ;;
  *)
    printf 'invalid managed-server auth-type for simple-api-server: %s\n' "${selected_auth_type}" >&2
    exit 1
    ;;
esac

{
  printf 'managedServer:\n'
  printf '  http:\n'
  printf '    url: %s\n' "${SIMPLE_API_SERVER_BASE_URL}"
  if [[ -n "${E2E_COMPONENT_OPENAPI_SPEC:-}" ]]; then
    printf '    openapi: %s\n' "${E2E_COMPONENT_OPENAPI_SPEC}"
  fi

  if [[ "${enable_mtls}" == 'true' ]]; then
    if [[ -z "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST:-}" || -z "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST:-}" || -z "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST:-}" ]]; then
      printf 'simple-api-server mTLS context requires tls ca/client certificate host paths\n' >&2
      exit 1
    fi

    printf '    tls:\n'
    printf '      caCertFile: %s\n' "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST}"
    printf '      clientCertFile: %s\n' "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST}"
    printf '      clientKeyFile: %s\n' "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST}"
  fi

  printf '    auth:\n'
  if [[ "${enable_oauth2}" == 'true' ]]; then
    printf '      oauth2:\n'
    printf '        tokenURL: %s\n' "${SIMPLE_API_SERVER_TOKEN_URL}"
    printf '        grantType: client_credentials\n'
    printf '        clientID: %s\n' "${SIMPLE_API_SERVER_CLIENT_ID}"
    printf '        clientSecret: %s\n' "${SIMPLE_API_SERVER_CLIENT_SECRET}"

    if [[ -n "${SIMPLE_API_SERVER_SCOPE:-}" ]]; then
      printf '        scope: %s\n' "${SIMPLE_API_SERVER_SCOPE}"
    fi
    if [[ -n "${SIMPLE_API_SERVER_AUDIENCE:-}" ]]; then
      printf '        audience: %s\n' "${SIMPLE_API_SERVER_AUDIENCE}"
    fi
  elif [[ "${selected_auth_type}" == 'prompt' ]]; then
    printf '      prompt: {}\n'
  elif [[ "${enable_basic_auth}" == 'true' ]]; then
    printf '      basic:\n'
    printf '        username: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}"
    printf '        password: %s\n' "${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}"
  else
    printf '      customHeaders:\n'
    printf '        - header: Authorization\n'
    printf '          prefix: Bearer\n'
    printf '          value: simple-api-oauth2-disabled\n'
  fi
} >"${fragment_file}"
