#!/usr/bin/env bash

# Lightweight logging helpers shared by secret-provider scripts.

timestamp() {
    date +"%Y-%m-%dT%H:%M:%S%z"
}

log_target() {
    local target="${RUN_LOG:-}"
    [[ -z "$target" ]] && return 0
    local dir
    dir="$(dirname "$target")"
    if [[ ! -d "$dir" ]]; then
        return 0
    fi
    printf "%s" "$target"
}

log_line() {
    local target
    target="$(log_target)"
    [[ -z "$target" ]] && return 0
    printf "[%s] %s\n" "$(timestamp)" "$*" >>"$target"
}

log_block() {
    local heading="$1"
    local body="$2"
    local target
    target="$(log_target)"
    [[ -z "$target" ]] && return 0
    {
        printf "[%s] %s\n" "$(timestamp)" "$heading"
        if [[ -n "$body" ]]; then
            printf "%s\n" "$body" | sed 's/^/    /'
        fi
    } >>"$target"
}

run_logged() {
    local label="$1"
    shift
    local cmd=("$@")
    local target
    target="$(log_target)"
    [[ -z "$target" ]] && target="/dev/null"

    log_line "START ${label} :: ${cmd[*]}"
    if "${cmd[@]}" >>"$target" 2>&1; then
        log_line "DONE  ${label}"
        return 0
    else
        local status=$?
        log_line "FAIL  ${label} (exit ${status})"
        return $status
    fi
}
