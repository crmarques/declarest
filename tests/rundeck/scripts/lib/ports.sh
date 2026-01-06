#!/usr/bin/env bash

port_in_use() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        ss -lnt "sport = :$port" 2>/dev/null | awk 'NR>1 {found=1} END{exit !found}'
        return $?
    fi
    if command -v netstat >/dev/null 2>&1; then
        netstat -lnt 2>/dev/null | awk -v p=":$port" '$4 ~ p {found=1} END{exit !found}'
        return $?
    fi
    if command -v lsof >/dev/null 2>&1; then
        lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
        return $?
    fi
    return 1
}

select_port() {
    local requested="$1"
    local start="$2"
    local end="$3"

    if [[ -n "$requested" ]] && ! port_in_use "$requested"; then
        printf "%s" "$requested"
        return 0
    fi
    local port
    for ((port=start; port<=end; port++)); do
        if ! port_in_use "$port"; then
            printf "%s" "$port"
            return 0
        fi
    done
    printf "%s" "$requested"
    return 0
}
