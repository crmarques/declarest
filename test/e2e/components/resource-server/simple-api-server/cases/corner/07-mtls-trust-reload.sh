#!/usr/bin/env bash

CASE_ID='simple-api-server-mtls-trust-reload'
CASE_SCOPE='corner'
CASE_REQUIRES='resource-server=simple-api-server has-resource-server-mtls'

CASE_ALLOWED_DIR=''
CASE_BACKUP_DIR=''

run_mtls_curl() {
  local -a args=(
    curl -fsS
    --cacert "${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST}"
    --cert "${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST}"
    --key "${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST}"
  )
  "${args[@]}" "$@"
}

health_request() {
  local -a args=()
  if [[ "${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}" == 'true' ]]; then
    args+=(-H "Authorization: Bearer ${HEALTH_ACCESS_TOKEN}")
  elif [[ "${SIMPLE_API_SERVER_ENABLE_BASIC_AUTH:-false}" == 'true' ]]; then
    args+=(-u "${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME}:${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD}")
  fi

  run_mtls_curl "${args[@]}" "${SIMPLE_API_SERVER_BASE_URL}/health"
}

expect_health_success() {
  if ! health_request >/dev/null 2>&1; then
    printf 'expected health request to succeed\n' >&2
    return 1
  fi
}

health_request_succeeds() {
  health_request >/dev/null 2>&1
}

health_request_fails() {
  ! health_request >/dev/null 2>&1
}

expect_health_failure() {
  if health_request_succeeds; then
    printf 'expected health request to fail\n' >&2
    return 1
  fi
}

trim() {
  local value=$1
  # shellcheck disable=SC2001
  sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' <<<"${value}"
}

resolve_host_trust_file() {
  local cert_file
  cert_file=$(trim "$1")
  if [[ -z "${cert_file}" ]]; then
    return 0
  fi

  if [[ -n "${SIMPLE_API_SERVER_CERTS_DIR:-}" && -n "${SIMPLE_API_SERVER_CERTS_HOST_DIR:-}" && "${cert_file}" == "${SIMPLE_API_SERVER_CERTS_DIR}"* ]]; then
    printf '%s%s\n' "${SIMPLE_API_SERVER_CERTS_HOST_DIR}" "${cert_file#${SIMPLE_API_SERVER_CERTS_DIR}}"
    return 0
  fi

  printf '%s\n' "${cert_file}"
}

resolve_mtls_trust_files() {
  local allowed_dir=$1
  local entry
  local -a entries=()
  local file

  if [[ -n "${SIMPLE_API_SERVER_MTLS_CLIENT_CERT_FILES:-}" ]]; then
    IFS=',' read -r -a entries <<<"${SIMPLE_API_SERVER_MTLS_CLIENT_CERT_FILES}"
    for entry in "${entries[@]}"; do
      resolve_host_trust_file "${entry}"
    done
    return 0
  fi

  shopt -s nullglob
  for file in "${allowed_dir}"/*.crt "${allowed_dir}"/*.pem "${allowed_dir}"/*.cer; do
    printf '%s\n' "${file}"
  done
  shopt -u nullglob
}

cleanup_restore_allowed_certs() {
  local allowed_dir=${CASE_ALLOWED_DIR:-}
  local backup_dir=${CASE_BACKUP_DIR:-}
  local file

  if [[ -z "${allowed_dir}" || ! -d "${backup_dir}" ]]; then
    return 0
  fi

  shopt -s nullglob
  for file in "${allowed_dir}"/*.crt "${allowed_dir}"/*.pem "${allowed_dir}"/*.cer; do
    rm -f "${file}"
  done
  shopt -u nullglob

  shopt -s nullglob
  for file in "${backup_dir}"/*; do
    mv "${file}" "${allowed_dir}/"
  done
  shopt -u nullglob
}

case_run() {
  local state_file
  local allowed_dir
  local backup_dir
  local token_response
  local token_value
  local file
  local trust_seed_file
  local trust_file
  local -a trust_files=()

  state_file=$(e2e_component_state_file 'resource-server:simple-api-server')
  if [[ ! -f "${state_file}" ]]; then
    printf 'simple-api-server state file not found: %s\n' "${state_file}" >&2
    return 1
  fi

  # shellcheck disable=SC1090
  source "${state_file}"

  allowed_dir="${SIMPLE_API_SERVER_CERTS_HOST_DIR}/clients/allowed"
  backup_dir="${E2E_CASE_TMP_DIR}/allowed-backup"
  mkdir -p "${allowed_dir}" "${backup_dir}"

  CASE_ALLOWED_DIR="${allowed_dir}"
  CASE_BACKUP_DIR="${backup_dir}"
  trap cleanup_restore_allowed_certs EXIT

  shopt -s nullglob
  for file in "${allowed_dir}"/*.crt "${allowed_dir}"/*.pem "${allowed_dir}"/*.cer; do
    mv "${file}" "${backup_dir}/"
  done
  shopt -u nullglob

  mapfile -t trust_files < <(resolve_mtls_trust_files "${allowed_dir}")
  if [[ ${#trust_files[@]} -eq 0 ]]; then
    trust_files=("${allowed_dir}/declarest-client.crt")
  fi

  trust_seed_file=''
  for trust_file in "${trust_files[@]}"; do
    if [[ -f "${trust_file}" ]]; then
      trust_seed_file="${trust_file}"
      break
    fi
  done
  if [[ -z "${trust_seed_file}" ]]; then
    trust_seed_file="${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST}"
  fi
  if [[ ! -f "${trust_seed_file}" ]]; then
    printf 'trusted certificate seed file not found: %s\n' "${trust_seed_file}" >&2
    return 1
  fi

  for trust_file in "${trust_files[@]}"; do
    mkdir -p "$(dirname "${trust_file}")"
    cp "${trust_seed_file}" "${trust_file}"
  done

  if [[ "${SIMPLE_API_SERVER_ENABLE_OAUTH2:-true}" == 'true' ]]; then
    token_response=$(
      run_mtls_curl \
        -X POST "${SIMPLE_API_SERVER_TOKEN_URL}" \
        -H 'Content-Type: application/x-www-form-urlencoded' \
        --data-urlencode 'grant_type=client_credentials' \
        --data-urlencode "client_id=${SIMPLE_API_SERVER_CLIENT_ID}" \
        --data-urlencode "client_secret=${SIMPLE_API_SERVER_CLIENT_SECRET}"
    ) || {
      printf 'failed to obtain oauth2 token from %s\n' "${SIMPLE_API_SERVER_TOKEN_URL}" >&2
      return 1
    }
    token_value=$(jq -r '.access_token // empty' <<<"${token_response}" 2>/dev/null || true)
    if [[ -z "${token_value}" ]]; then
      printf 'oauth2 token response missing access_token: %s\n' "${token_response}" >&2
      return 1
    fi
    HEALTH_ACCESS_TOKEN="${token_value}"
  else
    HEALTH_ACCESS_TOKEN=''
  fi

  expect_health_success || return 1

  for trust_file in "${trust_files[@]}"; do
    rm -f "${trust_file}"
  done
  case_wait_until 15 1 'simple-api-server mTLS trust removal to block client health checks' health_request_fails || return 1

  for trust_file in "${trust_files[@]}"; do
    cp "${trust_seed_file}" "${trust_file}"
  done
  case_wait_until 15 1 'simple-api-server mTLS trust restore to allow client health checks' health_request_succeeds || return 1
}
