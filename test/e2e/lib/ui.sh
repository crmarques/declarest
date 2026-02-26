#!/usr/bin/env bash

E2E_UI_TTY=0
E2E_UI_COLOR=0
E2E_UI_SPINNER='|/-\\'
E2E_UI_SPINNER_PID=''

E2E_STEPS_TOTAL=0
E2E_STEP_FAILED=0
E2E_STEP_LAST_LOG=''
E2E_STEP_STATUSES=()
E2E_STEP_TITLES=()
E2E_STEP_DURATIONS=()
E2E_STEP_LABEL_WIDTH=5
E2E_STEP_TITLE_WIDTH=24
E2E_STEP_DURATION_WIDTH=6
E2E_STEP_STATUS_WIDTH=12
E2E_STEP_TABLE_HEADER_PRINTED=0

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
  local cell_width=${2:-0}
  local plain_label
  local color=''

  case "${state}" in
    PASS|OK)
      plain_label='[OK]'
      color="${E2E_CLR_GREEN}"
      ;;
    FAIL)
      plain_label='[FAILED]'
      color="${E2E_CLR_RED}"
      ;;
    SKIP)
      plain_label='[SKIP]'
      color="${E2E_CLR_YELLOW}"
      ;;
    RUNNING)
      plain_label='[RUNNING]'
      color="${E2E_CLR_BLUE}"
      ;;
    *)
      plain_label="[${state}]"
      ;;
  esac

  local rendered_label="${plain_label}"
  if ((cell_width > 0)); then
    rendered_label=$(ui_center_text "${cell_width}" "${plain_label}")
  fi

  if [[ -n "${color}" ]]; then
    ui_colorize "${color}" "${rendered_label}"
  else
    printf '%s' "${rendered_label}"
  fi
}

ui_step_table_total_width() {
  printf '%d\n' $((E2E_STEP_LABEL_WIDTH + E2E_STEP_TITLE_WIDTH + E2E_STEP_DURATION_WIDTH + E2E_STEP_STATUS_WIDTH + 13))
}

ui_step_table_border_line() {
  local column_widths=(
    "${E2E_STEP_LABEL_WIDTH}"
    "${E2E_STEP_TITLE_WIDTH}"
    "${E2E_STEP_DURATION_WIDTH}"
    "${E2E_STEP_STATUS_WIDTH}"
  )
  local line='+'
  local width

  for width in "${column_widths[@]}"; do
    local segment
    segment=$(printf '%*s' $((width + 2)) '' | tr ' ' '-')
    line+="${segment}+"
  done

  printf '%s\n' "${line}"
}

ui_print_step_table_footer() {
  ui_step_table_border_line
}

ui_center_text() {
  local width=$1
  local text=$2
  local text_length=${#text}
  if ((text_length >= width)); then
    printf '%s' "${text}"
    return 0
  fi

  local left_padding=$(((width - text_length + 1) / 2))
  local right_padding=$((width - text_length - left_padding))
  printf '%*s%s%*s' "${left_padding}" '' "${text}" "${right_padding}" ''
}

ui_step_title_with_indicator() {
  local step_title=$1
  local indicator=$2
  if [[ -n "${indicator}" ]]; then
    printf '%s %s' "${indicator}" "${step_title}"
    return 0
  fi
  printf '%s' "${step_title}"
}

ui_print_step_table_header() {
  local force=${1:-0}
  if ((force == 0 && E2E_STEP_TABLE_HEADER_PRINTED == 1)); then
    return 0
  fi

  local step_header
  local title_header
  local span_header
  local status_header
  step_header=$(ui_center_text "${E2E_STEP_LABEL_WIDTH}" 'STEP')
  title_header=$(ui_center_text "${E2E_STEP_TITLE_WIDTH}" 'ACTION')
  span_header=$(ui_center_text "${E2E_STEP_DURATION_WIDTH}" 'SPAN')
  status_header=$(ui_center_text "${E2E_STEP_STATUS_WIDTH}" 'STATUS')
  ui_step_table_border_line

  printf '| %s | %s | %s | %s |\n' \
    "${step_header}" \
    "${title_header}" \
    "${span_header}" \
    "${status_header}"

  ui_step_table_border_line

  E2E_STEP_TABLE_HEADER_PRINTED=1
}

ui_step_line_render() {
  local step_number=$1
  local step_total=$2
  local step_title=$3
  local step_state=$4
  local indicator=$5
  local duration=$6

  local step_label
  step_label=$(ui_center_text "${E2E_STEP_LABEL_WIDTH}" "$(printf '%d/%d' "${step_number}" "${step_total}")")
  local title_label
  title_label=$(ui_step_title_with_indicator "${step_title}" "${indicator}")
  local span_label
  span_label=$(ui_center_text "${E2E_STEP_DURATION_WIDTH}" "${duration}")

  local status_label
  status_label=$(ui_step_state_label "${step_state}" "${E2E_STEP_STATUS_WIDTH}")

  printf '| %s | %-*s | %s | %s |' \
    "${step_label}" \
    "${E2E_STEP_TITLE_WIDTH}" \
    "${title_label}" \
    "${span_label}" \
    "${status_label}"
}

ui_print_step_line() {
  local step_number=$1
  local step_total=$2
  local step_title=$3
  local step_state=$4
  local elapsed=$5
  local should_print_footer=${6:-0}

  local duration=''
  if [[ "${step_state}" != 'RUNNING' ]]; then
    duration=$(e2e_format_duration "${elapsed}")
  fi

  if ((E2E_UI_TTY == 1)); then
    local width
    width=$(ui_step_table_total_width)
    printf '\r%-*s\r' "${width}" ''
  fi

  local line
  line=$(ui_step_line_render "${step_number}" "${step_total}" "${step_title}" "${step_state}" '' "${duration}")
  printf '%s\n' "${line}"

  if ((should_print_footer == 1)); then
    ui_print_step_table_footer
  fi
}

ui_selected_components_summary() {
  if ! declare -p E2E_SELECTED_COMPONENT_KEYS >/dev/null 2>&1 || ((${#E2E_SELECTED_COMPONENT_KEYS[@]} == 0)); then
    printf 'none\n'
    return
  fi

  local component_key
  local connection
  local -a labels=()
  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    connection=$(e2e_component_connection_for_key "${component_key}")
    labels+=("${component_key}@${connection}")
  done

  printf '%s\n' "${labels[*]}"
}

ui_write_step_log_header() {
  local step_log=$1
  local step_number=$2
  local step_total=$3
  local step_title=$4

  {
    printf '[%s] STEP %d/%d START %s\n' "$(e2e_now_utc)" "${step_number}" "${step_total}" "${step_title}"
    printf '[%s] run-id=%s profile=%s keep-runtime=%s verbose=%s\n' \
      "$(e2e_now_utc)" \
      "${E2E_RUN_ID:-n/a}" \
      "${E2E_PROFILE:-n/a}" \
      "${E2E_KEEP_RUNTIME:-0}" \
      "${E2E_VERBOSE:-0}"
    printf '[%s] stack repo-type=%s resource-server=%s(%s) resource-server-security=auth-type:%s mtls:%s git-provider=%s(%s) secret-provider=%s(%s)\n' \
      "$(e2e_now_utc)" \
      "${E2E_REPO_TYPE:-n/a}" \
      "${E2E_RESOURCE_SERVER:-n/a}" \
      "${E2E_RESOURCE_SERVER_CONNECTION:-n/a}" \
      "${E2E_RESOURCE_SERVER_AUTH_TYPE:-auto}" \
      "${E2E_RESOURCE_SERVER_MTLS:-false}" \
      "${E2E_GIT_PROVIDER:-none}" \
      "${E2E_GIT_PROVIDER_CONNECTION:-n/a}" \
      "${E2E_SECRET_PROVIDER:-n/a}" \
      "${E2E_SECRET_PROVIDER_CONNECTION:-n/a}"
    printf '[%s] selected-components: %s\n' "$(e2e_now_utc)" "$(ui_selected_components_summary)"
    printf '[%s] log-path=%s\n' "$(e2e_now_utc)" "${step_log}"
  } >>"${step_log}"
}

ui_write_step_log_footer() {
  local step_log=$1
  local step_number=$2
  local step_total=$3
  local step_title=$4
  local step_state=$5
  local step_rc=$6
  local step_elapsed=$7

  {
    printf '[%s] STEP %d/%d END %s state=%s rc=%d elapsed=%s\n' \
      "$(e2e_now_utc)" \
      "${step_number}" \
      "${step_total}" \
      "${step_title}" \
      "${step_state}" \
      "${step_rc}" \
      "${step_elapsed}"
  } >>"${step_log}"
}

ui_run_step_body() {
  local step_log=$1
  local step_fn=$2
  shift 2

  if [[ -n "${E2E_EXECUTION_LOG:-}" ]]; then
    "${step_fn}" "$@" > >(tee -a "${step_log}" "${E2E_EXECUTION_LOG}" >/dev/null) 2> >(tee -a "${step_log}" "${E2E_EXECUTION_LOG}" >/dev/null)
    return
  fi

  "${step_fn}" "$@" >"${step_log}" 2>&1
}

ui_spinner_start() {
  local step_number=$1
  local step_total=$2
  local step_title=$3

  ui_spinner_stop

  (
    local spin_index=0
    while true; do
      local spinner_char=${E2E_UI_SPINNER:spin_index:1}
      local line
      line=$(ui_step_line_render "${step_number}" "${step_total}" "${step_title}" "RUNNING" "${spinner_char}" '')
      printf '\r%s' "${line}"
      spin_index=$(((spin_index + 1) % 4))
      sleep 0.1
    done
  ) &
  E2E_UI_SPINNER_PID=$!
}

ui_spinner_stop() {
  local spinner_pid=${E2E_UI_SPINNER_PID:-}

  if [[ -n "${spinner_pid}" && "${spinner_pid}" =~ ^[0-9]+$ ]]; then
    kill "${spinner_pid}" >/dev/null 2>&1 || true
    wait "${spinner_pid}" 2>/dev/null || true
  fi

  E2E_UI_SPINNER_PID=''
  if ((E2E_UI_TTY == 1)); then
    local width
    width=$(ui_step_table_total_width)
    printf '\r%-*s\r' "${width}" ''
  fi
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
  ui_write_step_log_header "${step_log}" "${step_number}" "${step_total}" "${step_title}"

  if [[ -n "${E2E_EXECUTION_LOG:-}" ]]; then
    printf '\n[%s] STEP %d/%d START %s\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "${step_number}" \
      "${step_total}" \
      "${step_title}" >>"${E2E_EXECUTION_LOG}"
  fi

  if ((E2E_UI_TTY == 1)); then
    ui_print_step_table_header
    ui_spinner_start "${step_number}" "${step_total}" "${step_title}"
    set +e
    ui_run_step_body "${step_log}" "${step_fn}" "$@"
    rc=$?
    set -e

    ui_spinner_stop
  else
    ui_print_step_table_header
    ui_print_step_line "${step_number}" "${step_total}" "${step_title}" "RUNNING" 0
    set +e
    ui_run_step_body "${step_log}" "${step_fn}" "$@"
    rc=$?
    set -e
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
  ui_write_step_log_footer "${step_log}" "${step_number}" "${step_total}" "${step_title}" "${state}" "${rc}" "$(e2e_format_duration "${elapsed}")"

  if [[ -n "${E2E_EXECUTION_LOG:-}" ]]; then
    printf '[%s] STEP %d/%d END %s state=%s rc=%d elapsed=%s\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "${step_number}" \
      "${step_total}" \
      "${step_title}" \
      "${state}" \
      "${rc}" \
      "$(e2e_format_duration "${elapsed}")" >>"${E2E_EXECUTION_LOG}"
  fi

  local should_print_footer=0
  if ((step_number == step_total || rc != 0)); then
    should_print_footer=1
  fi

  ui_print_step_line "${step_number}" "${step_total}" "${step_title}" "${state}" "${elapsed}" "${should_print_footer}"

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
  printf '  execution-log: %s\n' "${E2E_EXECUTION_LOG:-n/a}"

  if [[ -n "${E2E_STEP_LAST_LOG}" ]]; then
    printf '  last-fail-log: %s\n' "${E2E_STEP_LAST_LOG}"
  fi
}
