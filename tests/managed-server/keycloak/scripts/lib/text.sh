#!/usr/bin/env bash

split_lines_nonempty() {
    local -n out="$1"
    local input="$2"
    out=()
    while IFS= read -r line; do
        [[ -n "$line" ]] && out+=("$line")
    done <<<"$input"
}

trim_whitespace() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf "%s" "$value"
}

sort_paths_by_depth() {
    local -n in_paths="$1"
    local -n out_paths="$2"
    local order="${3:-asc}"

    local entries=()
    local path slashes depth
    for path in "${in_paths[@]}"; do
        slashes="${path//[^\/]/}"
        depth=${#slashes}
        entries+=("${depth}"$'\t'"${path}")
    done

    if [[ "$order" == "desc" ]]; then
        mapfile -t out_paths < <(printf '%s\n' "${entries[@]}" | sort -rn | cut -f2-)
        return 0
    fi

    mapfile -t out_paths < <(printf '%s\n' "${entries[@]}" | sort -n | cut -f2-)
}
