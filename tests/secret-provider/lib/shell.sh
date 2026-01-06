#!/usr/bin/env bash

die() {
    printf "Error: %s\n" "$1" >&2
    exit 1
}

require_cmd() {
    local cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        printf "Missing required command: %s\n" "$cmd" >&2
        exit 1
    fi
}
