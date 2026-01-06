#!/usr/bin/env bash

set -euo pipefail

usage() {
    cat <<EOF
Usage: ./tests/clean-tests.sh [--all]

Options:
  --all   Also remove per-harness cache metadata (e.g. ~/.cache/declarest-*)
  -h, --help   Show this help message.
EOF
}

all=0
while [[ $# -gt 0 ]]; do
    case "$1" in
        --all)
            all=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            printf "Unknown option: %s\n" "$1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

base="${DECLAREST_WORK_BASE_DIR:-/tmp}"
printf "Cleaning workspaces under %s\n" "$base"
count=0
for prefix in declarest-keycloak- declarest-rundeck-; do
    while IFS= read -r dir; do
        if [[ -d "$dir" ]]; then
            printf "Removing %s\n" "$dir"
            rm -rf "$dir"
            count=$((count + 1))
        fi
    done < <(find "$base" -maxdepth 1 -type d -name "${prefix}*" 2>/dev/null)
done

if [[ $count -eq 0 ]]; then
    printf "No workspaces found under %s\n" "$base"
fi

if [[ $all -eq 1 ]]; then
    cache_base="${XDG_CACHE_HOME:-$HOME/.cache}"
    printf "Cleaning cache entries under %s\n" "$cache_base"
    for entry in declarest-keycloak declarest-rundeck; do
        target="$cache_base/$entry"
        if [[ -d "$target" ]]; then
            printf "Removing %s\n" "$target"
            rm -rf "$target"
        fi
    done
fi
