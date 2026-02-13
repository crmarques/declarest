#!/usr/bin/env bash

is_interactive() {
    [[ -t 0 ]]
}

prompt_required() {
    local prompt="$1"
    local value=""
    while [[ -z "$value" ]]; do
        read -r -p "$prompt" value
    done
    printf "%s" "$value"
}

prompt_optional() {
    local prompt="$1"
    local value=""
    read -r -p "$prompt" value || true
    printf "%s" "$value"
}

prompt_secret_required() {
    local prompt="$1"
    local value=""
    while [[ -z "$value" ]]; do
        read -r -s -p "$prompt" value
        printf "\n"
    done
    printf "%s" "$value"
}
