#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"

state_file=${E2E_COMPONENT_STATE_FILE}
: >"${state_file}"

normalize_bool() {
  local name=$1
  local raw_value=$2
  local default_value=$3
  local value=${raw_value:-${default_value}}

  case "${value,,}" in
    true|1|yes|on)
      printf 'true\n'
      ;;
    false|0|no|off)
      printf 'false\n'
      ;;
    *)
      printf 'invalid boolean value for %s: %s (allowed: true|false)\n' "${name}" "${value}" >&2
      return 1
      ;;
  esac
}

simple_api_auth_defaults_for_selected_type() {
  local auth_type=${E2E_RESOURCE_SERVER_AUTH_TYPE:-oauth2}

  case "${auth_type}" in
    none)
      printf 'false false\n'
      ;;
    basic)
      printf 'true false\n'
      ;;
    oauth2)
      printf 'false true\n'
      ;;
    custom-header)
      printf 'simple-api-server does not support resource-server auth-type custom-header\n' >&2
      return 1
      ;;
    *)
      printf 'invalid resource-server auth-type for simple-api-server: %s\n' "${auth_type}" >&2
      return 1
      ;;
  esac
}

generate_server_certificate() {
  local cert_file=$1
  local key_file=$2
  local config_file

  if openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -sha256 \
    -days 365 \
    -keyout "${key_file}" \
    -out "${cert_file}" \
    -subj '/CN=simple-api-server' \
    -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' \
    -addext 'extendedKeyUsage=serverAuth' >/dev/null 2>&1; then
    return 0
  fi

  config_file=$(mktemp /tmp/declarest-simple-api-server-server-cert.XXXXXX.cnf)
  e2e_register_temp_file "${config_file}"
  cat >"${config_file}" <<'EOF'
[req]
distinguished_name = req_distinguished_name
prompt = no
x509_extensions = v3_req

[req_distinguished_name]
CN = simple-api-server

[v3_req]
subjectAltName = DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth
EOF

  openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -sha256 \
    -days 365 \
    -keyout "${key_file}" \
    -out "${cert_file}" \
    -config "${config_file}" >/dev/null
}

generate_client_certificate() {
  local cert_file=$1
  local key_file=$2
  local config_file

  if openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -sha256 \
    -days 365 \
    -keyout "${key_file}" \
    -out "${cert_file}" \
    -subj '/CN=declarest-e2e-client' \
    -addext 'extendedKeyUsage=clientAuth' >/dev/null 2>&1; then
    return 0
  fi

  config_file=$(mktemp /tmp/declarest-simple-api-server-client-cert.XXXXXX.cnf)
  e2e_register_temp_file "${config_file}"
  cat >"${config_file}" <<'EOF'
[req]
distinguished_name = req_distinguished_name
prompt = no
x509_extensions = v3_req

[req_distinguished_name]
CN = declarest-e2e-client

[v3_req]
extendedKeyUsage = clientAuth
EOF

  openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -sha256 \
    -days 365 \
    -keyout "${key_file}" \
    -out "${cert_file}" \
    -config "${config_file}" >/dev/null
}

ensure_local_mtls_material() {
  local certs_host_dir=$1

  if ! command -v openssl >/dev/null 2>&1; then
    printf 'openssl is required when DECLAREST_E2E_SIMPLE_API_ENABLE_MTLS=true\n' >&2
    return 1
  fi

  local server_dir="${certs_host_dir}/server"
  local client_dir="${certs_host_dir}/clients/client"
  local allowed_dir="${certs_host_dir}/clients/allowed"
  local server_cert="${server_dir}/server.crt"
  local server_key="${server_dir}/server.key"
  local client_cert="${client_dir}/client.crt"
  local client_key="${client_dir}/client.key"
  local allowed_cert="${allowed_dir}/declarest-client.crt"

  mkdir -p "${server_dir}" "${client_dir}" "${allowed_dir}"
  chmod 0755 "${certs_host_dir}" "${certs_host_dir}/server" "${certs_host_dir}/clients" "${client_dir}" "${allowed_dir}"

  if [[ ! -s "${server_cert}" || ! -s "${server_key}" ]]; then
    generate_server_certificate "${server_cert}" "${server_key}" || return 1
  fi

  if [[ ! -s "${client_cert}" || ! -s "${client_key}" ]]; then
    generate_client_certificate "${client_cert}" "${client_key}" || return 1
  fi

  cp "${client_cert}" "${allowed_cert}"
  chmod 0644 "${server_cert}" "${server_key}" "${client_cert}" "${client_key}" "${allowed_cert}"
  return 0
}

if [[ "${E2E_COMPONENT_CONNECTION}" == 'local' ]]; then
  simple_api_auth_defaults=$(simple_api_auth_defaults_for_selected_type) || exit 1
  read -r simple_api_default_basic_auth simple_api_default_oauth2 <<<"${simple_api_auth_defaults}"
  simple_api_port=$(e2e_pick_free_port)
  simple_api_enable_basic_auth_raw=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_ENABLE_BASIC_AUTH' 'E2E_SIMPLE_API_ENABLE_BASIC_AUTH' || true)
  simple_api_enable_oauth2_raw=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_ENABLE_OAUTH2' 'E2E_SIMPLE_API_ENABLE_OAUTH2' || true)
  simple_api_enable_mtls_raw=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_ENABLE_MTLS' 'E2E_SIMPLE_API_ENABLE_MTLS' || true)
  simple_api_enable_basic_auth=$(normalize_bool 'DECLAREST_E2E_SIMPLE_API_ENABLE_BASIC_AUTH' "${simple_api_enable_basic_auth_raw}" "${simple_api_default_basic_auth}") || exit 1
  simple_api_enable_oauth2=$(normalize_bool 'DECLAREST_E2E_SIMPLE_API_ENABLE_OAUTH2' "${simple_api_enable_oauth2_raw}" "${simple_api_default_oauth2}") || exit 1
  simple_api_enable_mtls=$(normalize_bool 'DECLAREST_E2E_SIMPLE_API_ENABLE_MTLS' "${simple_api_enable_mtls_raw}" "${E2E_RESOURCE_SERVER_MTLS:-false}") || exit 1
  if [[ "${simple_api_enable_basic_auth}" == 'true' && "${simple_api_enable_oauth2}" == 'true' ]]; then
    printf 'simple-api-server supports basic-auth and oauth2, but e2e context auth is one-of: enable only one\n' >&2
    exit 1
  fi

  simple_api_certs_host_dir=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_CERTS_HOST_DIR' 'E2E_SIMPLE_API_CERTS_HOST_DIR' || true)
  simple_api_certs_dir=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_CERTS_DIR' 'E2E_SIMPLE_API_CERTS_DIR' || true)
  : "${simple_api_certs_host_dir:=${E2E_RUN_DIR}/certs/resource-server-simple-api-server}"
  : "${simple_api_certs_dir:=/etc/simple-api-server/certs}"

  simple_api_tls_cert_file=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_TLS_CERT_FILE' 'E2E_SIMPLE_API_TLS_CERT_FILE' || true)
  simple_api_tls_key_file=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_TLS_KEY_FILE' 'E2E_SIMPLE_API_TLS_KEY_FILE' || true)
  simple_api_mtls_client_cert_dir=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_MTLS_CLIENT_CERT_DIR' 'E2E_SIMPLE_API_MTLS_CLIENT_CERT_DIR' || true)
  simple_api_mtls_client_cert_files=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_MTLS_CLIENT_CERT_FILES' 'E2E_SIMPLE_API_MTLS_CLIENT_CERT_FILES' || true)

  : "${simple_api_tls_cert_file:=${simple_api_certs_dir}/server/server.crt}"
  : "${simple_api_tls_key_file:=${simple_api_certs_dir}/server/server.key}"
  : "${simple_api_mtls_client_cert_dir:=${simple_api_certs_dir}/clients/allowed}"

  simple_api_tls_ca_cert_file_host=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_TLS_CA_CERT_FILE' 'E2E_SIMPLE_API_TLS_CA_CERT_FILE' || true)
  simple_api_tls_client_cert_file_host=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_TLS_CLIENT_CERT_FILE' 'E2E_SIMPLE_API_TLS_CLIENT_CERT_FILE' || true)
  simple_api_tls_client_key_file_host=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_TLS_CLIENT_KEY_FILE' 'E2E_SIMPLE_API_TLS_CLIENT_KEY_FILE' || true)

  : "${simple_api_tls_ca_cert_file_host:=${simple_api_certs_host_dir}/server/server.crt}"
  : "${simple_api_tls_client_cert_file_host:=${simple_api_certs_host_dir}/clients/client/client.crt}"
  : "${simple_api_tls_client_key_file_host:=${simple_api_certs_host_dir}/clients/client/client.key}"

  simple_api_client_id=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_CLIENT_ID' 'E2E_SIMPLE_API_CLIENT_ID' || true)
  simple_api_client_secret=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_CLIENT_SECRET' 'E2E_SIMPLE_API_CLIENT_SECRET' || true)
  simple_api_basic_auth_username=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_BASIC_AUTH_USERNAME' 'E2E_SIMPLE_API_BASIC_AUTH_USERNAME' || true)
  simple_api_basic_auth_password=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_BASIC_AUTH_PASSWORD' 'E2E_SIMPLE_API_BASIC_AUTH_PASSWORD' || true)
  simple_api_scope=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_SCOPE' 'E2E_SIMPLE_API_SCOPE' || true)
  simple_api_audience=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_AUDIENCE' 'E2E_SIMPLE_API_AUDIENCE' || true)
  : "${simple_api_client_id:=declarest-e2e-client}"
  : "${simple_api_client_secret:=simple-api-${RANDOM}${RANDOM}${RANDOM}}"
  : "${simple_api_basic_auth_username:=declarest-e2e-basic-user}"
  : "${simple_api_basic_auth_password:=simple-api-basic-${RANDOM}${RANDOM}${RANDOM}}"

  mkdir -p "${simple_api_certs_host_dir}"

  if [[ "${simple_api_enable_mtls}" == 'true' ]]; then
    ensure_local_mtls_material "${simple_api_certs_host_dir}" || exit 1
    if [[ -z "${simple_api_mtls_client_cert_files}" ]]; then
      simple_api_mtls_client_cert_files="${simple_api_mtls_client_cert_dir}/declarest-client.crt"
    fi
    if [[ ! -f "${simple_api_tls_ca_cert_file_host}" || ! -f "${simple_api_tls_client_cert_file_host}" || ! -f "${simple_api_tls_client_key_file_host}" ]]; then
      printf 'simple-api-server mTLS is enabled but local TLS material is missing under %s\n' "${simple_api_certs_host_dir}" >&2
      exit 1
    fi
    simple_api_scheme='https'
  else
    simple_api_scheme='http'
  fi

  simple_api_base_url="${simple_api_scheme}://127.0.0.1:${simple_api_port}"
  simple_api_token_url="${simple_api_base_url}/token"

  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_HOST_PORT "${simple_api_port}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_BASE_URL "${simple_api_base_url}"
  e2e_write_state_value "${state_file}" RESOURCE_SERVER_BASE_URL "${simple_api_base_url}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_ENABLE_BASIC_AUTH "${simple_api_enable_basic_auth}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_ENABLE_OAUTH2 "${simple_api_enable_oauth2}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_ENABLE_MTLS "${simple_api_enable_mtls}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_CERTS_HOST_DIR "${simple_api_certs_host_dir}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_CERTS_DIR "${simple_api_certs_dir}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_CERT_FILE "${simple_api_tls_cert_file}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_KEY_FILE "${simple_api_tls_key_file}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_MTLS_CLIENT_CERT_DIR "${simple_api_mtls_client_cert_dir}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST "${simple_api_tls_ca_cert_file_host}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST "${simple_api_tls_client_cert_file_host}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST "${simple_api_tls_client_key_file_host}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TOKEN_URL "${simple_api_token_url}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_CLIENT_ID "${simple_api_client_id}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_CLIENT_SECRET "${simple_api_client_secret}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_BASIC_AUTH_USERNAME "${simple_api_basic_auth_username}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD "${simple_api_basic_auth_password}"

  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_MTLS_CLIENT_CERT_FILES "${simple_api_mtls_client_cert_files}"
  if [[ -n "${simple_api_scope}" ]]; then
    e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_SCOPE "${simple_api_scope}"
  fi
  if [[ -n "${simple_api_audience}" ]]; then
    e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_AUDIENCE "${simple_api_audience}"
  fi
  exit 0
fi

simple_api_base_url=$(e2e_require_env 'DECLAREST_E2E_RESOURCE_SERVER_BASE_URL' 'E2E_RESOURCE_SERVER_BASE_URL') || exit 1
simple_api_auth_defaults=$(simple_api_auth_defaults_for_selected_type) || exit 1
read -r simple_api_default_basic_auth simple_api_default_oauth2 <<<"${simple_api_auth_defaults}"
simple_api_enable_basic_auth_raw=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_ENABLE_BASIC_AUTH' 'E2E_SIMPLE_API_ENABLE_BASIC_AUTH' || true)
simple_api_enable_oauth2_raw=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_ENABLE_OAUTH2' 'E2E_SIMPLE_API_ENABLE_OAUTH2' || true)
simple_api_enable_mtls_raw=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_ENABLE_MTLS' 'E2E_SIMPLE_API_ENABLE_MTLS' || true)
simple_api_enable_basic_auth=$(normalize_bool 'DECLAREST_E2E_SIMPLE_API_ENABLE_BASIC_AUTH' "${simple_api_enable_basic_auth_raw}" "${simple_api_default_basic_auth}") || exit 1
simple_api_enable_oauth2=$(normalize_bool 'DECLAREST_E2E_SIMPLE_API_ENABLE_OAUTH2' "${simple_api_enable_oauth2_raw}" "${simple_api_default_oauth2}") || exit 1
simple_api_enable_mtls=$(normalize_bool 'DECLAREST_E2E_SIMPLE_API_ENABLE_MTLS' "${simple_api_enable_mtls_raw}" "${E2E_RESOURCE_SERVER_MTLS:-false}") || exit 1
if [[ "${simple_api_enable_basic_auth}" == 'true' && "${simple_api_enable_oauth2}" == 'true' ]]; then
  printf 'simple-api-server supports basic-auth and oauth2, but e2e context auth is one-of: enable only one\n' >&2
  exit 1
fi
simple_api_token_url=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_TOKEN_URL' 'E2E_SIMPLE_API_TOKEN_URL' || true)
simple_api_scope=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_SCOPE' 'E2E_SIMPLE_API_SCOPE' || true)
simple_api_audience=$(e2e_env_optional 'DECLAREST_E2E_SIMPLE_API_AUDIENCE' 'E2E_SIMPLE_API_AUDIENCE' || true)

simple_api_client_id=''
simple_api_client_secret=''
simple_api_basic_auth_username=''
simple_api_basic_auth_password=''
if [[ "${simple_api_enable_oauth2}" == 'true' ]]; then
  simple_api_client_id=$(e2e_require_env 'DECLAREST_E2E_SIMPLE_API_CLIENT_ID' 'E2E_SIMPLE_API_CLIENT_ID') || exit 1
  simple_api_client_secret=$(e2e_require_env 'DECLAREST_E2E_SIMPLE_API_CLIENT_SECRET' 'E2E_SIMPLE_API_CLIENT_SECRET') || exit 1
fi
if [[ "${simple_api_enable_basic_auth}" == 'true' ]]; then
  simple_api_basic_auth_username=$(e2e_require_env 'DECLAREST_E2E_SIMPLE_API_BASIC_AUTH_USERNAME' 'E2E_SIMPLE_API_BASIC_AUTH_USERNAME') || exit 1
  simple_api_basic_auth_password=$(e2e_require_env 'DECLAREST_E2E_SIMPLE_API_BASIC_AUTH_PASSWORD' 'E2E_SIMPLE_API_BASIC_AUTH_PASSWORD') || exit 1
fi

simple_api_tls_ca_cert_file_host=''
simple_api_tls_client_cert_file_host=''
simple_api_tls_client_key_file_host=''
if [[ "${simple_api_enable_mtls}" == 'true' ]]; then
  simple_api_tls_ca_cert_file_host=$(e2e_require_env 'DECLAREST_E2E_SIMPLE_API_TLS_CA_CERT_FILE' 'E2E_SIMPLE_API_TLS_CA_CERT_FILE') || exit 1
  simple_api_tls_client_cert_file_host=$(e2e_require_env 'DECLAREST_E2E_SIMPLE_API_TLS_CLIENT_CERT_FILE' 'E2E_SIMPLE_API_TLS_CLIENT_CERT_FILE') || exit 1
  simple_api_tls_client_key_file_host=$(e2e_require_env 'DECLAREST_E2E_SIMPLE_API_TLS_CLIENT_KEY_FILE' 'E2E_SIMPLE_API_TLS_CLIENT_KEY_FILE') || exit 1
fi

if [[ -z "${simple_api_token_url}" ]]; then
  simple_api_token_url="${simple_api_base_url%/}/token"
fi

e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_BASE_URL "${simple_api_base_url}"
e2e_write_state_value "${state_file}" RESOURCE_SERVER_BASE_URL "${simple_api_base_url}"
e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_ENABLE_BASIC_AUTH "${simple_api_enable_basic_auth}"
e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_ENABLE_OAUTH2 "${simple_api_enable_oauth2}"
e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_ENABLE_MTLS "${simple_api_enable_mtls}"
if [[ "${simple_api_enable_oauth2}" == 'true' ]]; then
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TOKEN_URL "${simple_api_token_url}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_CLIENT_ID "${simple_api_client_id}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_CLIENT_SECRET "${simple_api_client_secret}"
fi
if [[ "${simple_api_enable_basic_auth}" == 'true' ]]; then
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_BASIC_AUTH_USERNAME "${simple_api_basic_auth_username}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD "${simple_api_basic_auth_password}"
fi
if [[ "${simple_api_enable_mtls}" == 'true' ]]; then
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST "${simple_api_tls_ca_cert_file_host}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST "${simple_api_tls_client_cert_file_host}"
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST "${simple_api_tls_client_key_file_host}"
fi

if [[ -n "${simple_api_scope}" ]]; then
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_SCOPE "${simple_api_scope}"
fi
if [[ -n "${simple_api_audience}" ]]; then
  e2e_write_state_value "${state_file}" SIMPLE_API_SERVER_AUDIENCE "${simple_api_audience}"
fi
