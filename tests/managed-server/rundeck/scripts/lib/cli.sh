#!/usr/bin/env bash

declarest_bin() {
    printf "%s/declarest" "$DECLAREST_BIN_DIR"
}

declarest_cli() {
    local bin args=()
    bin="$(declarest_bin)"
    if [[ -n "${DECLAREST_DEBUG_GROUPS:-}" ]]; then
        args+=("--debug=${DECLAREST_DEBUG_GROUPS}")
    fi
    if [[ -n "${DECLAREST_CONTEXT_FILE:-}" ]]; then
        HOME="$DECLAREST_HOME_DIR" DECLAREST_CONTEXT_FILE="$DECLAREST_CONTEXT_FILE" "$bin" "${args[@]}" "$@"
    else
        HOME="$DECLAREST_HOME_DIR" "$bin" "${args[@]}" "$@"
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
