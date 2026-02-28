#!/usr/bin/env bash

# shellcheck disable=SC2034
E2E_LIB_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
E2E_ROOT_DIR=$(cd -- "${E2E_LIB_DIR}/../../.." && pwd)
E2E_DIR="${E2E_ROOT_DIR}/test/e2e"
E2E_RUNS_DIR="${E2E_DIR}/.runs"

: "${E2E_RUN_ID:=}"
: "${E2E_RUN_DIR:=}"
: "${E2E_STATE_DIR:=}"
: "${E2E_LOG_DIR:=}"
: "${E2E_CONTEXT_DIR:=}"
: "${E2E_CONTEXT_FILE:=}"
: "${E2E_BIN:=}"
: "${E2E_START_EPOCH:=0}"
: "${E2E_METADATA_DIR:=}"
: "${E2E_METADATA_BUNDLE:=}"

: "${E2E_VERBOSE:=0}"
: "${E2E_KEEP_RUNTIME:=0}"
: "${E2E_PROFILE:=basic}"
: "${E2E_CONTAINER_ENGINE:=${DECLAREST_E2E_CONTAINER_ENGINE:-podman}}"
: "${E2E_EXECUTION_LOG:=${DECLAREST_E2E_EXECUTION_LOG:-}}"

: "${E2E_STEP_SKIP:=42}"

if ! declare -p E2E_TEMP_FILES >/dev/null 2>&1; then
  E2E_TEMP_FILES=()
fi
if ! declare -p E2E_DEPRECATED_ENV_WARNED >/dev/null 2>&1; then
  declare -gA E2E_DEPRECATED_ENV_WARNED=()
fi

e2e_info() {
  printf '[INFO] %s\n' "$*"
}

e2e_warn() {
  printf '[WARN] %s\n' "$*" >&2
}

e2e_error() {
  printf '[ERROR] %s\n' "$*" >&2
}

e2e_die() {
  e2e_error "$*"
  return 1
}

e2e_require_command() {
  local command_name=$1
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    e2e_die "required command not found: ${command_name}"
    return 1
  fi
}

e2e_validate_container_engine() {
  case "${E2E_CONTAINER_ENGINE}" in
    podman|docker)
      return 0
      ;;
    *)
      e2e_die "invalid DECLAREST_E2E_CONTAINER_ENGINE value: ${E2E_CONTAINER_ENGINE} (allowed: podman, docker)"
      return 1
      ;;
  esac
}

e2e_compose_cmd() {
  e2e_run_cmd "${E2E_CONTAINER_ENGINE}" compose "$@"
}

e2e_warn_deprecated_env() {
  local legacy_name=$1
  local canonical_name=$2

  if [[ "${E2E_DEPRECATED_ENV_WARNED[${legacy_name}]:-0}" == '1' ]]; then
    return 0
  fi

  E2E_DEPRECATED_ENV_WARNED["${legacy_name}"]=1
  e2e_warn "env ${legacy_name} is deprecated; use ${canonical_name}"
}

e2e_env_optional() {
  local canonical_name=$1
  local legacy_name=${2:-}

  if [[ -n "${!canonical_name:-}" ]]; then
    printf '%s\n' "${!canonical_name}"
    return 0
  fi

  if [[ -n "${legacy_name}" && -n "${!legacy_name:-}" ]]; then
    e2e_warn_deprecated_env "${legacy_name}" "${canonical_name}"
    printf '%s\n' "${!legacy_name}"
    return 0
  fi

  return 1
}

e2e_require_env() {
  local canonical_name=$1
  local legacy_name=${2:-}
  local value

  value=$(e2e_env_optional "${canonical_name}" "${legacy_name}") || {
    if [[ -n "${legacy_name}" ]]; then
      e2e_die "missing env ${canonical_name} (legacy fallback: ${legacy_name})"
    else
      e2e_die "missing env ${canonical_name}"
    fi
    return 1
  }

  printf '%s\n' "${value}"
}

e2e_epoch_now() {
  date +%s
}

e2e_now_utc() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

e2e_format_duration() {
  local total_seconds=$1
  local hours=$((total_seconds / 3600))
  local minutes=$(((total_seconds % 3600) / 60))
  local seconds=$((total_seconds % 60))

  if ((hours > 0)); then
    printf '%dh%02dm%02ds' "${hours}" "${minutes}" "${seconds}"
    return
  fi

  if ((minutes > 0)); then
    printf '%dm%02ds' "${minutes}" "${seconds}"
    return
  fi

  printf '%ds' "${seconds}"
}

e2e_register_temp_file() {
  local file_path=$1
  E2E_TEMP_FILES+=("${file_path}")
}

e2e_cleanup_temp_files() {
  local file_path
  for file_path in "${E2E_TEMP_FILES[@]:-}"; do
    [[ -n "${file_path}" && -f "${file_path}" ]] && rm -f "${file_path}"
  done
}

e2e_pick_free_port() {
  local port
  local used

  for port in $(shuf -i 18080-28999 -n 120); do
    used=$(ss -ltn 2>/dev/null | awk '{print $4}' | grep -E ":${port}$" || true)
    if [[ -z "${used}" ]]; then
      printf '%s\n' "${port}"
      return 0
    fi
  done

  e2e_die "failed to allocate free TCP port"
}

e2e_quote_cmd() {
  local arg
  local -a quoted=()
  for arg in "$@"; do
    quoted+=("$(printf '%q' "${arg}")")
  done
  printf '%s\n' "${quoted[*]}"
}

e2e_run_cmd() {
  local cmd_string
  local rc
  local had_errexit=0

  cmd_string=$(e2e_quote_cmd "$@")
  e2e_info "cmd: ${cmd_string}"

  if [[ $- == *e* ]]; then
    had_errexit=1
  fi

  set +e
  "$@"
  rc=$?
  if ((had_errexit == 1)); then
    set -e
  fi

  if ((rc != 0)); then
    e2e_error "cmd failed rc=${rc}: ${cmd_string}"
    return "${rc}"
  fi

  return 0
}

e2e_write_state_value() {
  local state_file=$1
  local key=$2
  local value=$3
  local encoded_value

  encoded_value=$(printf '%s' "${value}" | base64 | tr -d '\n') || return 1
  printf '%s=%q\n' "${key}" "${value}" >>"${state_file}"
  printf '__DECLAREST_B64_%s=%s\n' "${key}" "${encoded_value}" >>"${state_file}"
}

e2e_state_get() {
  local state_file=$1
  local key=$2
  local b64_key
  local encoded_value
  local value

  if [[ ! -f "${state_file}" ]]; then
    return 1
  fi

  b64_key="__DECLAREST_B64_${key}"
  encoded_value=$(awk -v k="${b64_key}" 'index($0, k "=") == 1 { print substr($0, length(k) + 2) }' "${state_file}" | tail -n 1 || true)
  if [[ -n "${encoded_value}" ]]; then
    printf '%s' "${encoded_value}" | base64 -d
    return $?
  fi

  value=$(awk -v k="${key}" 'index($0, k "=") == 1 { print substr($0, length(k) + 2) }' "${state_file}" | tail -n 1 || true)
  if [[ -z "${value}" ]]; then
    return 1
  fi

  # Backward compatibility for pre-base64 state files.
  # shellcheck disable=SC2086
  eval "printf '%s\n' ${value}"
}

e2e_has_tty() {
  [[ -t 1 ]]
}
