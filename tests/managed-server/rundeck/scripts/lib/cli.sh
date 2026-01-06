#!/usr/bin/env bash

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
