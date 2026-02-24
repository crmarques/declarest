#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

if [[ "${E2E_COMPONENT_CONNECTION}" != 'local' ]]; then
  exit 0
fi

enable_basic_auth=${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}
enable_oauth2=${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}
enable_mtls=${SIMPLE_API_SERVER_ENABLE_MTLS:-false}

if [[ "${enable_basic_auth}" == 'true' && "${enable_oauth2}" == 'true' ]]; then
  printf 'simple-api-server context supports only one auth mode; both basic-auth and oauth2 are enabled\n' >&2
  exit 1
fi

run_curl() {
  local -a args=(curl -fsS)

  if [[ "${enable_mtls}" == 'true' ]]; then
    args+=(
      --cacert "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST}"
      --cert "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST}"
      --key "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST}"
    )
  fi

  "${args[@]}" "$@"
}

wait_for_health() {
  local bearer_token=${1:-}
  local basic_user=${2:-}
  local basic_password=${3:-}
  local attempts=${4:-45}
  local delay=${5:-2}
  local i

  for ((i = 1; i <= attempts; i++)); do
    if [[ -n "${bearer_token}" ]]; then
      if run_curl -H "Authorization: Bearer ${bearer_token}" "${SIMPLE_API_SERVER_BASE_URL}/health" >/dev/null 2>&1; then
        return 0
      fi
    elif [[ -n "${basic_user}" || -n "${basic_password}" ]]; then
      if run_curl -u "${basic_user}:${basic_password}" "${SIMPLE_API_SERVER_BASE_URL}/health" >/dev/null 2>&1; then
        return 0
      fi
    else
      if run_curl "${SIMPLE_API_SERVER_BASE_URL}/health" >/dev/null 2>&1; then
        return 0
      fi
    fi
    sleep "${delay}"
  done

  return 1
}

wait_for_token() {
  local attempts=${1:-90}
  local delay=${2:-2}
  local response
  local token
  local i

  for ((i = 1; i <= attempts; i++)); do
    response=$(
      run_curl \
        -X POST "${SIMPLE_API_SERVER_TOKEN_URL}" \
        -H 'Content-Type: application/x-www-form-urlencoded' \
        --data-urlencode 'grant_type=client_credentials' \
        --data-urlencode "client_id=${SIMPLE_API_SERVER_CLIENT_ID}" \
        --data-urlencode "client_secret=${SIMPLE_API_SERVER_CLIENT_SECRET}" 2>/dev/null || true
    )
    token=$(jq -r '.access_token // empty' <<<"${response}" 2>/dev/null || true)
    if [[ -n "${token}" ]]; then
      printf '%s\n' "${token}"
      return 0
    fi
    sleep "${delay}"
  done

  return 1
}

if [[ "${enable_oauth2}" == 'true' ]]; then
  access_token=$(wait_for_token 90 2) || {
    printf 'simple-api-server did not issue oauth2 token: %s\n' "${SIMPLE_API_SERVER_TOKEN_URL}" >&2
    if [[ -n "${E2E_COMPONENT_PROJECT_NAME:-}" ]]; then
      compose_file="${E2E_COMPONENT_DIR}/compose.yaml"
      if [[ -f "${compose_file}" ]]; then
        e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" ps >&2 || true
        e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" logs simple-api-server >&2 || true
      fi
    fi
    exit 1
  }

  wait_for_health "${access_token}" '' '' 45 2 || {
    printf 'simple-api-server health endpoint check failed: %s/health\n' "${SIMPLE_API_SERVER_BASE_URL}" >&2
    if [[ -n "${E2E_COMPONENT_PROJECT_NAME:-}" ]]; then
      compose_file="${E2E_COMPONENT_DIR}/compose.yaml"
      if [[ -f "${compose_file}" ]]; then
        e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" ps >&2 || true
        e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" logs simple-api-server >&2 || true
      fi
    fi
    exit 1
  }
  exit 0
fi

if [[ "${enable_basic_auth}" == 'true' ]]; then
  wait_for_health '' "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}" "${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}" 45 2 || {
    printf 'simple-api-server health endpoint check failed: %s/health\n' "${SIMPLE_API_SERVER_BASE_URL}" >&2
    if [[ -n "${E2E_COMPONENT_PROJECT_NAME:-}" ]]; then
      compose_file="${E2E_COMPONENT_DIR}/compose.yaml"
      if [[ -f "${compose_file}" ]]; then
        e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" ps >&2 || true
        e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" logs simple-api-server >&2 || true
      fi
    fi
    exit 1
  }
  exit 0
fi

wait_for_health '' '' '' 45 2 || {
  printf 'simple-api-server health endpoint check failed: %s/health\n' "${SIMPLE_API_SERVER_BASE_URL}" >&2
  if [[ -n "${E2E_COMPONENT_PROJECT_NAME:-}" ]]; then
    compose_file="${E2E_COMPONENT_DIR}/compose.yaml"
    if [[ -f "${compose_file}" ]]; then
      e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" ps >&2 || true
      e2e_compose_cmd -f "${compose_file}" -p "${E2E_COMPONENT_PROJECT_NAME}" logs simple-api-server >&2 || true
    fi
  fi
  exit 1
}
