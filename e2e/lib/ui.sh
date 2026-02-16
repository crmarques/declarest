#!/usr/bin/env bash

E2E_UI_TTY=0
E2E_UI_COLOR=0
E2E_UI_SPINNER='|/-\\'

E2E_STEPS_TOTAL=0
E2E_STEP_FAILED=0
E2E_STEP_LAST_LOG=''
E2E_STEP_STATUSES=()
E2E_STEP_TITLES=()
E2E_STEP_DURATIONS=()

E2E_CASE_TOTAL=0
E2E_CASE_PASSED=0
E2E_CASE_FAILED=0
E2E_CASE_SKIPPED=0

E2E_CLR_RESET=''
E2E_CLR_BLUE=''
E2E_CLR_GREEN=''
E2E_CLR_YELLOW=''
E2E_CLR_RED=''
E2E_CLR_DIM=''

ui_init() {
  if e2e_has_tty; then
    E2E_UI_TTY=1
    if [[ -n "${TERM:-}" && "${TERM}" != "dumb" ]]; then
      E2E_UI_COLOR=1
    fi
  fi

  if ((E2E_UI_COLOR == 1)); then
    E2E_CLR_RESET='\033[0m'
    E2E_CLR_BLUE='\033[34m'
    E2E_CLR_GREEN='\033[32m'
    E2E_CLR_YELLOW='\033[33m'
    E2E_CLR_RED='\033[31m'
    E2E_CLR_DIM='\033[2m'
  fi
}

ui_colorize() {
  local color=$1
  shift
  printf '%b%s%b' "${color}" "$*" "${E2E_CLR_RESET}"
}

ui_step_state_label() {
  local state=$1
  case "${state}" in
    PASS)
      ui_colorize "${E2E_CLR_GREEN}" "PASS"
      ;;
    OK)
      ui_colorize "${E2E_CLR_GREEN}" "OK"
      ;;
    FAIL)
      ui_colorize "${E2E_CLR_RED}" "FAIL"
      ;;
    SKIP)
      ui_colorize "${E2E_CLR_YELLOW}" "SKIP"
      ;;
    RUNNING)
      ui_colorize "${E2E_CLR_BLUE}" "RUNNING"
      ;;
    *)
      printf '%s' "${state}"
      ;;
  esac
}

ui_print_step_line() {
  local step_number=$1
  local step_total=$2
  local step_title=$3
  local step_state=$4
  local elapsed=$5
  local suffix=${6:-}

  if ((E2E_UI_TTY == 1)); then
    printf '\r%-120s\r' ''
  fi

  printf 'Step %d/%d [%s] %s %s' \
    "${step_number}" \
    "${step_total}" \
    "$(ui_step_state_label "${step_state}")" \
    "${step_title}" \
    "$(ui_colorize "${E2E_CLR_DIM}" "(${elapsed})")"

  if [[ -n "${suffix}" ]]; then
    printf ' %s' "${suffix}"
  fi

  printf '\n'
}

ui_run_step() {
  local step_number=$1
  local step_total=$2
  local step_title=$3
  local step_fn=$4

  shift 4

  E2E_STEP_TITLES[step_number]="${step_title}"

  local step_log="${E2E_LOG_DIR}/step-${step_number}.log"
  local step_start
  local step_end
  local elapsed
  local rc=0

  : >"${step_log}"

  step_start=$(e2e_epoch_now)

  if ((E2E_UI_TTY == 1)); then
    "${step_fn}" "$@" >"${step_log}" 2>&1 &
    local pid=$!
    local spin_index=0

    while kill -0 "${pid}" >/dev/null 2>&1; do
      local spinner_char=${E2E_UI_SPINNER:spin_index:1}
      printf '\r%s Step %d/%d [%s] %s %s' \
        "$(ui_colorize "${E2E_CLR_BLUE}" "${spinner_char}")" \
        "${step_number}" \
        "${step_total}" \
        "$(ui_step_state_label "RUNNING")" \
        "${step_title}" \
        "$(ui_colorize "${E2E_CLR_DIM}" "...")"
      spin_index=$(((spin_index + 1) % 4))
      sleep 0.1
    done

    wait "${pid}" || rc=$?
    printf '\r%-120s\r' ''
  else
    ui_print_step_line "${step_number}" "${step_total}" "${step_title}" "RUNNING" "0s"
    "${step_fn}" "$@" >"${step_log}" 2>&1 || rc=$?
  fi

  step_end=$(e2e_epoch_now)
  elapsed=$((step_end - step_start))

  local state='OK'
  if ((rc == E2E_STEP_SKIP)); then
    state='SKIP'
    rc=0
  elif ((rc != 0)); then
    state='FAIL'
    E2E_STEP_FAILED=1
    E2E_STEP_LAST_LOG="${step_log}"
  fi

  E2E_STEP_STATUSES[step_number]="${state}"
  E2E_STEP_DURATIONS[step_number]="${elapsed}"

  ui_print_step_line "${step_number}" "${step_total}" "${step_title}" "${state}" "$(e2e_format_duration "${elapsed}")"

  if ((rc != 0)); then
    printf '  log: %s\n' "${step_log}"
    tail -n 30 "${step_log}" | sed 's/^/  | /'
    return "${rc}"
  fi

  if ((E2E_VERBOSE == 1)); then
    printf '  log: %s\n' "${step_log}"
  fi

  return 0
}

ui_case_result() {
  local case_id=$1
  local state=$2
  local message=${3:-}

  ((E2E_CASE_TOTAL += 1))

  case "${state}" in
    PASS)
      ((E2E_CASE_PASSED += 1))
      ;;
    FAIL)
      ((E2E_CASE_FAILED += 1))
      ;;
    SKIP)
      ((E2E_CASE_SKIPPED += 1))
      ;;
  esac

  printf '  case %-28s %-5s' "${case_id}" "$(ui_step_state_label "${state}")"
  if [[ -n "${message}" ]]; then
    printf ' %s' "${message}"
  fi
  printf '\n'
}

ui_print_summary() {
  local end_epoch
  local total_elapsed
  local step_number

  end_epoch=$(e2e_epoch_now)
  total_elapsed=$((end_epoch - E2E_START_EPOCH))

  printf '\n'
  printf 'E2E Summary\n'
  printf '%s\n' '-----------'

  for ((step_number = 1; step_number <= E2E_STEPS_TOTAL; step_number++)); do
    local title=${E2E_STEP_TITLES[step_number]:-n/a}
    local state=${E2E_STEP_STATUSES[step_number]:-SKIP}
    local duration=${E2E_STEP_DURATIONS[step_number]:-0}

    printf '  step %d: %-18s %-4s (%s)\n' \
      "${step_number}" \
      "${title}" \
      "$(ui_step_state_label "${state}")" \
      "$(e2e_format_duration "${duration}")"
  done

  printf '\n'
  printf '  cases total=%d passed=%d failed=%d skipped=%d\n' \
    "${E2E_CASE_TOTAL}" \
    "${E2E_CASE_PASSED}" \
    "${E2E_CASE_FAILED}" \
    "${E2E_CASE_SKIPPED}"

  printf '  duration: %s\n' "$(e2e_format_duration "${total_elapsed}")"
  printf '  context:  %s\n' "${E2E_CONTEXT_FILE:-n/a}"
  printf '  logs:     %s\n' "${E2E_LOG_DIR:-n/a}"

  if [[ -n "${E2E_STEP_LAST_LOG}" ]]; then
    printf '  last-fail-log: %s\n' "${E2E_STEP_LAST_LOG}"
  fi
}
