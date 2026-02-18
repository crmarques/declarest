#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

required_vars=(
  SIMPLE_API_SERVER_BASE_URL
)

for var_name in "${required_vars[@]}"; do
  if [[ -z "${!var_name:-}" ]]; then
    printf 'missing required simple-api-server state variable: %s\n' "${var_name}" >&2
    exit 1
  fi
done

enable_basic_auth=${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}
enable_oauth2=${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}
enable_mtls=${SIMPLE_API_SERVER_ENABLE_MTLS:-false}

if [[ "${enable_basic_auth}" == 'true' && "${enable_oauth2}" == 'true' ]]; then
  printf 'simple-api-server context supports only one auth mode; both basic-auth and oauth2 are enabled\n' >&2
  exit 1
fi

if [[ "${enable_basic_auth}" == 'true' ]]; then
  basic_required_vars=(
    SIMPLE_API_SERVER_BASIC_AUTH_USERNAME
    SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD
  )

  for var_name in "${basic_required_vars[@]}"; do
    if [[ -z "${!var_name:-}" ]]; then
      printf 'missing required simple-api-server basic-auth state variable: %s\n' "${var_name}" >&2
      exit 1
    fi
  done
fi

if [[ "${enable_oauth2}" == 'true' ]]; then
  oauth_required_vars=(
    SIMPLE_API_SERVER_TOKEN_URL
    SIMPLE_API_SERVER_CLIENT_ID
    SIMPLE_API_SERVER_CLIENT_SECRET
  )

  for var_name in "${oauth_required_vars[@]}"; do
    if [[ -z "${!var_name:-}" ]]; then
      printf 'missing required simple-api-server oauth2 state variable: %s\n' "${var_name}" >&2
      exit 1
    fi
  done
fi

if [[ "${enable_mtls}" == 'true' ]]; then
  mtls_required_vars=(
    SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST
    SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST
    SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST
  )

  for var_name in "${mtls_required_vars[@]}"; do
    if [[ -z "${!var_name:-}" ]]; then
      printf 'missing required simple-api-server mTLS state variable: %s\n' "${var_name}" >&2
      exit 1
    fi
  done
fi
