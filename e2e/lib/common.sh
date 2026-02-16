#!/usr/bin/env bash

# shellcheck disable=SC2034
E2E_LIB_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
E2E_ROOT_DIR=$(cd -- "${E2E_LIB_DIR}/../.." && pwd)
E2E_DIR="${E2E_ROOT_DIR}/e2e"
E2E_RUNS_DIR="${E2E_DIR}/.runs"

: "${E2E_RUN_ID:=}"
: "${E2E_RUN_DIR:=}"
: "${E2E_STATE_DIR:=}"
: "${E2E_LOG_DIR:=}"
: "${E2E_CONTEXT_DIR:=}"
: "${E2E_CONTEXT_FILE:=}"
: "${E2E_BIN:=}"
: "${E2E_START_EPOCH:=0}"

: "${E2E_VERBOSE:=0}"
: "${E2E_KEEP_RUNTIME:=0}"
: "${E2E_PROFILE:=basic}"

: "${E2E_STEP_SKIP:=42}"

if ! declare -p E2E_TEMP_FILES >/dev/null 2>&1; then
  E2E_TEMP_FILES=()
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
  fi
}

e2e_epoch_now() {
  date +%s
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

e2e_run_cmd() {
  if ((E2E_VERBOSE == 1)); then
    e2e_info "running: $*"
  fi
  "$@"
}

e2e_write_state_value() {
  local state_file=$1
  local key=$2
  local value=$3
  printf '%s=%q\n' "${key}" "${value}" >>"${state_file}"
}

e2e_state_get() {
  local state_file=$1
  local key=$2
  local value

  if [[ ! -f "${state_file}" ]]; then
    return 1
  fi

  value=$(grep -E "^${key}=" "${state_file}" | tail -n 1 | cut -d= -f2- || true)
  if [[ -z "${value}" ]]; then
    return 1
  fi

  # shellcheck disable=SC2086
  eval "printf '%s\n' ${value}"
}

e2e_has_tty() {
  [[ -t 1 ]]
}
