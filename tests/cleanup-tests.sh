#!/usr/bin/env bash

set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RUNTIME="${CONTAINER_RUNTIME:-podman}"

usage() {
    cat <<EOF
Usage: ./tests/cleanup-tests.sh [--all]

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
safe_remove() {
    local path="$1"
    if rm -rf "$path"; then
        return 0
    fi
    if command -v podman >/dev/null 2>&1; then
        podman unshare chown -R "$(id -u):$(id -g)" "$path" >/dev/null 2>&1 || true
        podman unshare rm -rf "$path" >/dev/null 2>&1 && return 0
    fi
    return 1
}

remove_containers_by_prefix() {
    local pattern
    if ! command -v "$RUNTIME" >/dev/null 2>&1; then
        return
    fi
    for pattern in "$@"; do
        local containers
        mapfile -t containers < <("$RUNTIME" ps -aq --format '{{.Names}}' 2>/dev/null | grep -E "^${pattern}" || true)
        if ((${#containers[@]} > 0)); then
            printf "Removing containers matching %s\n" "$pattern"
            "$RUNTIME" rm -f "${containers[@]}" >/dev/null 2>&1 || true
        fi
    done
}

remove_networks_by_prefix() {
    local pattern
    if ! command -v "$RUNTIME" >/dev/null 2>&1; then
        return
    fi
    for pattern in "$@"; do
        local networks
        mapfile -t networks < <("$RUNTIME" network ls --format '{{.Name}}' | grep -E "^${pattern}" || true)
        if ((${#networks[@]} > 0)); then
            printf "Removing networks matching %s\n" "$pattern"
            "$RUNTIME" network rm "${networks[@]}" >/dev/null 2>&1 || true
        fi
    done
}

remove_volumes_by_prefix() {
    local pattern
    if ! command -v "$RUNTIME" >/dev/null 2>&1; then
        return
    fi
    for pattern in "$@"; do
        local volumes
        mapfile -t volumes < <("$RUNTIME" volume ls --format '{{.Name}}' | grep -E "^${pattern}" || true)
        if ((${#volumes[@]} > 0)); then
            printf "Removing volumes matching %s\n" "$pattern"
            "$RUNTIME" volume rm "${volumes[@]}" >/dev/null 2>&1 || true
        fi
    done
}

stop_stack() {
    local workspace="$1"
    local project
    project="$(basename "$workspace")"
    case "$project" in
        declarest-keycloak-*)
            if [[ -x "$TESTS_DIR/managed-server/keycloak/run.sh" ]]; then
                "$TESTS_DIR/managed-server/keycloak/run.sh" stop --work-dir "$workspace" >/dev/null 2>&1 || true
            fi
            ;;
        declarest-rundeck-*)
            if [[ -x "$TESTS_DIR/managed-server/rundeck/run.sh" ]]; then
                "$TESTS_DIR/managed-server/rundeck/run.sh" stop --work-dir "$workspace" >/dev/null 2>&1 || true
            fi
            ;;
    esac
}

printf "Cleaning workspaces under %s\n" "$base"
count=0
remove_containers_by_prefix declarest-keycloak- declarest-rundeck-
remove_networks_by_prefix declarest-keycloak- declarest-rundeck-
remove_volumes_by_prefix declarest-keycloak- declarest-rundeck-
for prefix in declarest-keycloak- declarest-rundeck-; do
    while IFS= read -r dir; do
        if [[ -d "$dir" ]]; then
            printf "Stopping services in %s\n" "$dir"
            stop_stack "$dir"
            printf "Removing %s\n" "$dir"
            safe_remove "$dir" || printf "  Warning: could not remove %s (permissions)\n" "$dir"
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
            safe_remove "$target" || printf "  Warning: could not remove %s (permissions)\n" "$target"
        fi
    done
fi
