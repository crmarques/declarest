#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd -- "${SCRIPT_DIR}/../.." && pwd)
RUNS_DIR="${SCRIPT_DIR}/.runs"

e2e_parallel_usage() {
  cat <<'USAGE'
Usage: ./test/e2e/run-e2e-parallel.sh [--matrix-file <path>] [--log-dir <path>]

Run multiple e2e commands concurrently and return a non-zero exit code when any
child command fails.

Input:
  Provide one shell command per line through stdin or --matrix-file.
  Empty lines and lines starting with # are ignored.

Example:
  ./test/e2e/run-e2e-parallel.sh <<'EOF'
  ./test/e2e/run-e2e.sh --profile cli-basic --managed-service keycloak --platform compose
  ./test/e2e/run-e2e.sh --profile cli-basic --managed-service rundeck --platform compose
  EOF
USAGE
}

e2e_parallel_load_commands() {
  local input_file=$1
  local line
  local trimmed

  if [[ -n "${input_file}" ]]; then
    [[ -f "${input_file}" ]] || {
      printf '[ERROR] matrix file not found: %s\n' "${input_file}" >&2
      return 1
    }

    while IFS= read -r line || [[ -n "${line}" ]]; do
      line=${line%$'\r'}
      trimmed=${line#"${line%%[![:space:]]*}"}
      [[ -n "${trimmed}" ]] || continue
      [[ "${trimmed}" == \#* ]] && continue
      E2E_PARALLEL_COMMANDS+=("${line}")
    done <"${input_file}"
    return 0
  fi

  if [[ -t 0 ]]; then
    printf '[ERROR] provide commands through stdin or --matrix-file\n' >&2
    return 1
  fi

  while IFS= read -r line || [[ -n "${line}" ]]; do
    line=${line%$'\r'}
    trimmed=${line#"${line%%[![:space:]]*}"}
    [[ -n "${trimmed}" ]] || continue
    [[ "${trimmed}" == \#* ]] && continue
    E2E_PARALLEL_COMMANDS+=("${line}")
  done
}

e2e_parallel_stop_children() {
  local pid

  for pid in "${E2E_PARALLEL_PIDS[@]:-}"; do
    [[ -n "${pid}" ]] || continue
    kill "${pid}" >/dev/null 2>&1 || true
  done

  for pid in "${E2E_PARALLEL_PIDS[@]:-}"; do
    [[ -n "${pid}" ]] || continue
    wait "${pid}" >/dev/null 2>&1 || true
  done
}

e2e_parallel_handle_signal() {
  local signal_name=$1
  printf '\n[WARN] received %s; stopping parallel e2e jobs\n' "${signal_name}" >&2
  e2e_parallel_stop_children
  exit 130
}

main() {
  local matrix_file=''
  local log_dir=''

  while (($# > 0)); do
    case "$1" in
      --matrix-file)
        [[ $# -ge 2 ]] || {
          printf '[ERROR] missing value for --matrix-file\n' >&2
          return 1
        }
        matrix_file=$2
        shift 2
        ;;
      --log-dir)
        [[ $# -ge 2 ]] || {
          printf '[ERROR] missing value for --log-dir\n' >&2
          return 1
        }
        log_dir=$2
        shift 2
        ;;
      -h|--help)
        e2e_parallel_usage
        return 0
        ;;
      *)
        printf '[ERROR] unknown argument: %s\n' "$1" >&2
        return 1
        ;;
    esac
  done

  E2E_PARALLEL_COMMANDS=()
  E2E_PARALLEL_PIDS=()
  E2E_PARALLEL_LOGS=()
  E2E_PARALLEL_LABELS=()
  E2E_PARALLEL_RCS=()

  e2e_parallel_load_commands "${matrix_file}" || return 1
  if ((${#E2E_PARALLEL_COMMANDS[@]} == 0)); then
    printf '[ERROR] no commands were provided\n' >&2
    return 1
  fi

  if [[ -z "${log_dir}" ]]; then
    log_dir="${RUNS_DIR}/parallel-$(date +%Y%m%d-%H%M%S)-$$"
  fi
  mkdir -p "${log_dir}" || {
    printf '[ERROR] failed to create log dir: %s\n' "${log_dir}" >&2
    return 1
  }

  trap 'e2e_parallel_handle_signal INT' INT
  trap 'e2e_parallel_handle_signal TERM' TERM

  printf 'Parallel E2E log dir: %s\n' "${log_dir}"

  local idx
  local label
  local log_file
  local command_line
  for idx in "${!E2E_PARALLEL_COMMANDS[@]}"; do
    label=$(printf 'job-%02d' "$((idx + 1))")
    log_file="${log_dir}/${label}.log"
    command_line=${E2E_PARALLEL_COMMANDS[${idx}]}

    {
      printf '[%s] COMMAND %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${command_line}"
      cd "${REPO_ROOT}"
      bash -lc "${command_line}"
    } >"${log_file}" 2>&1 &

    E2E_PARALLEL_PIDS+=("$!")
    E2E_PARALLEL_LOGS+=("${log_file}")
    E2E_PARALLEL_LABELS+=("${label}")

    printf '[RUN ] %s %s\n' "${label}" "${command_line}"
  done

  local failed=0
  local pid
  local rc
  for idx in "${!E2E_PARALLEL_PIDS[@]}"; do
    pid=${E2E_PARALLEL_PIDS[${idx}]}
    set +e
    wait "${pid}"
    rc=$?
    set -e
    E2E_PARALLEL_RCS[${idx}]="${rc}"
    if ((rc != 0)); then
      failed=1
    fi
  done

  for idx in "${!E2E_PARALLEL_LABELS[@]}"; do
    label=${E2E_PARALLEL_LABELS[${idx}]}
    log_file=${E2E_PARALLEL_LOGS[${idx}]}
    rc=${E2E_PARALLEL_RCS[${idx}]}
    if ((rc == 0)); then
      printf '[PASS] %s rc=%d log=%s\n' "${label}" "${rc}" "${log_file}"
      continue
    fi

    printf '[FAIL] %s rc=%d log=%s\n' "${label}" "${rc}" "${log_file}" >&2
  done

  if ((failed == 1)); then
    return 1
  fi

  return 0
}

main "$@"
