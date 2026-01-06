#!/usr/bin/env bash

DEFAULT_CLI_RETRY_PATTERN="unexpected status 5[0-9][0-9]|502 Bad Gateway|503 Service Unavailable|504 Gateway Timeout|connection refused|connect: connection refused|EOF|i/o timeout"

declarest_bin() {
    printf "%s/declarest" "$DECLAREST_BIN_DIR"
}

declarest_cli() {
    local bin
    bin="$(declarest_bin)"
    if [[ -n "${DECLAREST_CONTEXT_FILE:-}" ]]; then
        HOME="$DECLAREST_HOME_DIR" DECLAREST_CONTEXT_FILE="$DECLAREST_CONTEXT_FILE" "$bin" "$@"
    else
        HOME="$DECLAREST_HOME_DIR" "$bin" "$@"
    fi
}

run_cli() {
    local label="$1"
    shift
    run_logged "$label" declarest_cli "$@"
}

capture_cli() {
    local label="$1"
    shift
    capture_logged "$label" declarest_cli "$@"
}

capture_cli_all() {
    local label="$1"
    shift
    local output status

    log_line "START ${label} :: cli $*"
    output=$(declarest_cli "$@" 2>&1)
    status=$?
    if [[ $status -eq 0 ]]; then
        log_block "${label} output" "$output"
        log_line "DONE  ${label}"
        printf "%s" "$output"
        return 0
    fi

    log_block "${label} output (partial)" "$output"
    log_line "FAIL  ${label} (exit ${status})"
    printf "%s" "$output"
    return $status
}

run_cli_retry_transient() {
    local label="$1"
    local max_attempts="${2:-10}"
    local delay="${3:-2}"
    shift 3

    local output status
    local pattern="${CLI_RETRY_PATTERN:-$DEFAULT_CLI_RETRY_PATTERN}"
    CLI_LAST_OUTPUT=""
    CLI_LAST_STATUS=0
    for ((attempt=1; attempt<=max_attempts; attempt++)); do
        log_line "START ${label} (attempt ${attempt}/${max_attempts}) :: cli $*"
        if output="$(declarest_cli "$@" 2>&1)"; then
            status=0
        else
            status=$?
        fi
        CLI_LAST_OUTPUT="$output"
        CLI_LAST_STATUS=$status

        if [[ -n "$output" ]]; then
            log_block "${label} output (attempt ${attempt})" "$output"
        fi

        if [[ $status -eq 0 ]]; then
            log_line "DONE  ${label} (attempt ${attempt}/${max_attempts})"
            return 0
        fi

        log_line "FAIL  ${label} (attempt ${attempt}/${max_attempts}, exit ${status})"

        if [[ -n "$pattern" ]] && grep -Eq "$pattern" <<<"$output"; then
            if declare -F cli_retry_on_failure >/dev/null 2>&1; then
                cli_retry_on_failure || true
            fi
            sleep "$delay"
            continue
        fi

        return $status
    done

    return 1
}
