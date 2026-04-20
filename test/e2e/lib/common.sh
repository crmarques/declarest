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
: "${E2E_BUILD_CACHE_DIR:=${E2E_ROOT_DIR}/.e2e-build}"
: "${E2E_LOCKS_DIR:=${E2E_RUNS_DIR}/.locks}"
: "${E2E_PORT_RESERVATIONS_DIR:=${E2E_RUNS_DIR}/.port-reservations}"
: "${E2E_BIN:=}"
: "${E2E_OPERATOR_BIN:=}"
: "${E2E_OPERATOR_IMAGE:=}"
: "${E2E_OPERATOR_MANAGER_PID:=}"
: "${E2E_OPERATOR_MANAGER_LOG_FILE:=}"
: "${E2E_OPERATOR_MANAGER_DEPLOYMENT:=}"
: "${E2E_OPERATOR_MANAGER_POD:=}"
: "${E2E_OPERATOR_NAMESPACE:=}"
: "${E2E_OPERATOR_RESOURCE_REPOSITORY_NAME:=}"
: "${E2E_OPERATOR_MANAGED_SERVICE_NAME:=}"
: "${E2E_OPERATOR_SECRET_STORE_NAME:=}"
: "${E2E_OPERATOR_SYNC_POLICY_NAME:=}"
: "${E2E_KUBECONFIG:=}"
: "${E2E_KIND_CLUSTER_NAME:=}"
: "${E2E_KIND_CLUSTER_REUSED:=0}"
: "${E2E_KIND_ACTIVE_CLUSTER_SLOT:=}"
: "${E2E_KIND_ACTIVE_CLUSTER_LOCK_PATH:=}"
: "${E2E_K8S_NAMESPACE:=}"
: "${E2E_START_EPOCH:=0}"
: "${E2E_METADATA:=bundle}"
: "${E2E_METADATA_DIR:=}"
: "${E2E_METADATA_BUNDLE:=}"

: "${E2E_VERBOSE:=0}"
: "${E2E_KEEP_RUNTIME:=0}"
: "${E2E_PROFILE:=cli-basic}"
: "${E2E_PLATFORM:=kubernetes}"
: "${E2E_CONTAINER_ENGINE:=${DECLAREST_E2E_CONTAINER_ENGINE:-podman}}"
: "${E2E_EXECUTION_LOG:=${DECLAREST_E2E_EXECUTION_LOG:-}}"
: "${E2E_KIND_NODE_ROOT:=/workspace/declarest}"
: "${E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS:=${DECLAREST_E2E_K8S_COMPONENT_READY_TIMEOUT_SECONDS:-600}}"
: "${E2E_OPERATOR_READY_TIMEOUT_SECONDS:=${DECLAREST_E2E_OPERATOR_READY_TIMEOUT_SECONDS:-300}}"

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

e2e_validate_platform() {
  case "${E2E_PLATFORM}" in
    compose|kubernetes)
      return 0
      ;;
    *)
      e2e_die "invalid e2e platform: ${E2E_PLATFORM} (allowed: compose, kubernetes)"
      return 1
      ;;
  esac
}

e2e_resolve_go_version() {
  local go_mod="${E2E_ROOT_DIR}/go.mod"
  local version

  [[ -f "${go_mod}" ]] || {
    e2e_die "go.mod not found: ${go_mod}"
    return 1
  }

  version=$(awk '/^go /{print $2; exit}' "${go_mod}")
  [[ -n "${version}" ]] || {
    e2e_die "unable to resolve Go version from ${go_mod}"
    return 1
  }

  printf '%s\n' "${version}"
}

e2e_resolve_go_arch() {
  local arch

  arch=$(go env GOARCH 2>/dev/null) || {
    e2e_die 'unable to resolve Go architecture via go env GOARCH'
    return 1
  }

  [[ -n "${arch}" ]] || {
    e2e_die 'go env GOARCH returned an empty architecture'
    return 1
  }

  printf '%s\n' "${arch}"
}

e2e_compose_cmd() {
  e2e_run_cmd "${E2E_CONTAINER_ENGINE}" compose "$@"
}

e2e_kind_run_raw() {
  if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
    KIND_EXPERIMENTAL_PROVIDER=podman kind "$@"
    return $?
  fi

  kind "$@"
}

e2e_kind_cmd() {
  if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
    e2e_run_cmd env KIND_EXPERIMENTAL_PROVIDER=podman kind "$@"
    return $?
  fi

  e2e_run_cmd kind "$@"
}

e2e_kubectl_cmd() {
  e2e_run_cmd kubectl "$@"
}

e2e_env_optional() {
  local canonical_name=$1

  if [[ -n "${!canonical_name:-}" ]]; then
    printf '%s\n' "${!canonical_name}"
    return 0
  fi

  return 1
}

e2e_require_env() {
  local canonical_name=$1
  local value

  value=$(e2e_env_optional "${canonical_name}") || {
    e2e_die "missing env ${canonical_name}"
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

e2e_lock_path() {
  local name=$1
  local safe_name=${name//[^A-Za-z0-9._-]/_}
  printf '%s/%s.lock\n' "${E2E_LOCKS_DIR}" "${safe_name}"
}

e2e_lock_try_acquire() {
  local name=$1
  local lock_path
  local owner_pid=''

  lock_path=$(e2e_lock_path "${name}")
  mkdir -p "${E2E_LOCKS_DIR}" || return 1

  if mkdir "${lock_path}" 2>/dev/null; then
    printf '%s\n' "$$" >"${lock_path}/pid"
    printf '%s\n' "${lock_path}"
    return 0
  fi

  if [[ -f "${lock_path}/pid" ]]; then
    owner_pid=$(cat "${lock_path}/pid" 2>/dev/null || true)
  fi
  if [[ -n "${owner_pid}" && "${owner_pid}" =~ ^[0-9]+$ ]] && ! kill -0 "${owner_pid}" >/dev/null 2>&1; then
    rm -rf "${lock_path}" >/dev/null 2>&1 || true
    if mkdir "${lock_path}" 2>/dev/null; then
      printf '%s\n' "$$" >"${lock_path}/pid"
      printf '%s\n' "${lock_path}"
      return 0
    fi
  fi

  return 1
}

e2e_lock_acquire_with_timeout() {
  local name=$1
  local timeout_seconds=${2:-60}
  local lock_path
  local owner_pid=''
  local attempts=0
  local max_attempts

  if ! [[ "${timeout_seconds}" =~ ^[0-9]+$ ]] || ((timeout_seconds <= 0)); then
    e2e_die "invalid lock timeout seconds: ${timeout_seconds}"
    return 1
  fi
  max_attempts=$((timeout_seconds * 10))

  lock_path=$(e2e_lock_path "${name}")
  mkdir -p "${E2E_LOCKS_DIR}" || return 1

  while ! mkdir "${lock_path}" 2>/dev/null; do
    owner_pid=''
    if [[ -f "${lock_path}/pid" ]]; then
      owner_pid=$(cat "${lock_path}/pid" 2>/dev/null || true)
    fi
    if [[ -n "${owner_pid}" && "${owner_pid}" =~ ^[0-9]+$ ]] && ! kill -0 "${owner_pid}" >/dev/null 2>&1; then
      rm -rf "${lock_path}" >/dev/null 2>&1 || true
      continue
    fi

    ((attempts += 1))
    if ((attempts >= max_attempts)); then
      e2e_die "timed out waiting for lock: ${name}"
      return 1
    fi
    sleep 0.1
  done

  printf '%s\n' "$$" >"${lock_path}/pid"
  printf '%s\n' "${lock_path}"
}

e2e_lock_acquire() {
  e2e_lock_acquire_with_timeout "$1" 60
}

e2e_lock_release() {
  local lock_path=$1
  [[ -n "${lock_path}" ]] || return 0

  rm -f "${lock_path}/pid" >/dev/null 2>&1 || true
  rmdir "${lock_path}" >/dev/null 2>&1 || rm -rf "${lock_path}" >/dev/null 2>&1 || true
}

e2e_with_lock() {
  local name=$1
  shift

  local lock_path
  local rc
  local had_errexit=0

  lock_path=$(e2e_lock_acquire "${name}") || return 1

  if [[ $- == *e* ]]; then
    had_errexit=1
  fi

  set +e
  "$@"
  rc=$?
  if ((had_errexit == 1)); then
    set -e
  fi

  e2e_lock_release "${lock_path}"
  return "${rc}"
}

e2e_with_lock_timeout() {
  local name=$1
  local timeout_seconds=$2
  shift 2

  local lock_path
  local rc
  local had_errexit=0

  lock_path=$(e2e_lock_acquire_with_timeout "${name}" "${timeout_seconds}") || return 1

  if [[ $- == *e* ]]; then
    had_errexit=1
  fi

  set +e
  "$@"
  rc=$?
  if ((had_errexit == 1)); then
    set -e
  fi

  e2e_lock_release "${lock_path}"
  return "${rc}"
}

e2e_port_reservation_path() {
  local port=$1
  printf '%s/%s\n' "${E2E_PORT_RESERVATIONS_DIR}" "${port}"
}

e2e_is_port_reserved() {
  local port=$1
  local reservation_file

  reservation_file=$(e2e_port_reservation_path "${port}")
  [[ -f "${reservation_file}" ]]
}

e2e_reserve_port() {
  local port=$1
  local reservation_file

  reservation_file=$(e2e_port_reservation_path "${port}")
  mkdir -p "${E2E_PORT_RESERVATIONS_DIR}" || return 1
  if [[ -f "${reservation_file}" ]]; then
    return 1
  fi

  {
    printf 'run_id=%s\n' "${E2E_RUN_ID:-}"
    printf 'pid=%s\n' "$$"
  } >"${reservation_file}" || return 1
}

e2e_pick_free_port_locked() {
  local port
  local used

  for port in $(shuf -i 18080-28999 -n 120); do
    if e2e_is_port_reserved "${port}"; then
      continue
    fi

    used=$(ss -ltn 2>/dev/null | awk '{print $4}' | grep -E ":${port}$" || true)
    if [[ -z "${used}" ]]; then
      e2e_reserve_port "${port}" || continue
      printf '%s\n' "${port}"
      return 0
    fi
  done

  e2e_die "failed to allocate free TCP port"
}

e2e_pick_free_port() {
  e2e_with_lock 'port-allocation' e2e_pick_free_port_locked
}

e2e_release_reserved_ports_for_run() {
  local run_id=$1
  local reservation_file

  [[ -n "${run_id}" ]] || return 0
  [[ -d "${E2E_PORT_RESERVATIONS_DIR}" ]] || return 0

  while IFS= read -r reservation_file; do
    [[ -n "${reservation_file}" ]] || continue
    if grep -Fxq "run_id=${run_id}" "${reservation_file}" 2>/dev/null; then
      rm -f "${reservation_file}" || return 1
    fi
  done < <(find "${E2E_PORT_RESERVATIONS_DIR}" -mindepth 1 -maxdepth 1 -type f | sort)
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

e2e_go_build_target_is_stale() {
  local target=$1
  shift

  if [[ ! -x "${target}" ]]; then
    return 0
  fi

  local source_path
  for source_path in "$@"; do
    [[ -e "${source_path}" ]] || continue

    if [[ -f "${source_path}" ]]; then
      if [[ "${source_path}" -nt "${target}" ]]; then
        return 0
      fi
      continue
    fi

    if find "${source_path}" -type f -name '*.go' -newer "${target}" -print -quit | grep -q .; then
      return 0
    fi
  done

  return 1
}

e2e_stage_cached_binary() {
  local cached_binary=$1
  local target_binary=$2

  [[ -f "${cached_binary}" ]] || {
    e2e_die "cached binary not found: ${cached_binary}"
    return 1
  }

  mkdir -p "$(dirname -- "${target_binary}")" || return 1
  cp -f "${cached_binary}" "${target_binary}" || return 1
  chmod +x "${target_binary}" || return 1
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

  if [[ ! -f "${state_file}" ]]; then
    return 1
  fi

  b64_key="__DECLAREST_B64_${key}"
  encoded_value=$(awk -v k="${b64_key}" 'index($0, k "=") == 1 { print substr($0, length(k) + 2) }' "${state_file}" | tail -n 1 || true)
  if [[ -n "${encoded_value}" ]]; then
    printf '%s' "${encoded_value}" | base64 -d
    return $?
  fi
  return 1
}

e2e_find_collection_metadata_files() {
  local root=$1
  find "${root}" -type f \( -path '*/_/metadata.yaml' -o -path '*/_/metadata.json' \) | sort
}

e2e_metadata_logical_path_from_file() {
  local metadata_root=$1
  local metadata_file=$2
  local rel_path
  local logical_path

  rel_path=${metadata_file#${metadata_root}/}
  case "${rel_path}" in
    */metadata.yaml)
      logical_path=/${rel_path%/metadata.yaml}
      ;;
    */metadata.json)
      logical_path=/${rel_path%/metadata.json}
      ;;
    *)
      printf 'unsupported metadata fixture path: %s\n' "${metadata_file}" >&2
      return 1
      ;;
  esac

  logical_path=${logical_path%/}
  printf '%s\n' "${logical_path}"
}

e2e_has_tty() {
  [[ -t 1 ]]
}
