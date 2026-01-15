#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"

usage() {
    cat <<EOF
Usage: cleanup.sh [--all]

Options:
  --all    Remove all declarest Keycloak work directories and container artifacts.
  -h, --help Show this help message.
EOF
}

clean_all=0
while [[ $# -gt 0 ]]; do
    case "$1" in
        --all)
            clean_all=1
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

runtime_available() {
    command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1
}

remove_project_artifacts() {
    local project="$1"
    if [[ -z "$project" ]]; then
        return 0
    fi
    if ! runtime_available; then
        log_line "Container runtime ${CONTAINER_RUNTIME} not available; skipping container cleanup"
        return 0
    fi

    mapfile -t compose_containers < <("$CONTAINER_RUNTIME" ps -aq --filter "label=com.docker.compose.project=${project}" || true)
    if ((${#compose_containers[@]} > 0)); then
        run_logged "remove compose containers" "$CONTAINER_RUNTIME" rm -f "${compose_containers[@]}" || true
    fi

    mapfile -t compose_networks < <("$CONTAINER_RUNTIME" network ls -q --filter "label=com.docker.compose.project=${project}" || true)
    if ((${#compose_networks[@]} > 0)); then
        run_logged "remove compose networks" "$CONTAINER_RUNTIME" network rm "${compose_networks[@]}" || true
    fi

    mapfile -t compose_volumes < <("$CONTAINER_RUNTIME" volume ls -q --filter "label=com.docker.compose.project=${project}" || true)
    if ((${#compose_volumes[@]} > 0)); then
        run_logged "remove compose volumes" "$CONTAINER_RUNTIME" volume rm "${compose_volumes[@]}" || true
    fi
}

remove_work_dir() {
    local work_dir="$1"
    local home_dir="$2"
    if [[ -z "$work_dir" ]]; then
        return 0
    fi

    if [[ -n "$home_dir" && -d "$home_dir" && "$home_dir" == "$work_dir"* ]]; then
        storage_dir="$home_dir/.local/share/containers"
        if [[ -d "$storage_dir" ]] && runtime_available; then
            log_line "Removing container storage at $storage_dir"
            "$CONTAINER_RUNTIME" unshare rm -rf "$storage_dir" >/dev/null 2>&1 || true
        fi
    fi

    chmod -R u+w "$work_dir" >/dev/null 2>&1 || true

    set +e
    rm -rf "$work_dir"
    rm_status=$?
    set -e
    if [[ $rm_status -ne 0 && runtime_available && "$CONTAINER_RUNTIME" == "podman" ]]; then
        log_line "Retrying work directory removal via podman unshare: $work_dir"
        "$CONTAINER_RUNTIME" unshare rm -rf "$work_dir" || true
        set +e
        rm -rf "$work_dir"
        rm_status=$?
        set -e
    fi
    if [[ $rm_status -ne 0 ]]; then
        printf "Failed to remove work directory: %s\n" "$work_dir" >&2
        exit 1
    fi
    log_line "Work directory removed: $work_dir"
}

if [[ $clean_all -eq 0 && "${DECLAREST_KEEP_WORK:-0}" == "1" ]]; then
    log_line "DECLAREST_KEEP_WORK=1; preserving work directory at $DECLAREST_WORK_DIR"
    exit 0
fi

if [[ $clean_all -eq 1 ]]; then
    base_dir="${DECLAREST_WORK_BASE_DIR:-/tmp}"
    if [[ -z "$base_dir" ]]; then
        base_dir="/tmp"
    fi

    mapfile -t work_dirs < <(find "$base_dir" -maxdepth 1 -type d -name 'declarest-keycloak-*' 2>/dev/null || true)
    project_names=()
    for work_dir in "${work_dirs[@]}"; do
        project_names+=("$(basename "$work_dir")")
    done
    if runtime_available; then
        mapfile -t runtime_projects < <("$CONTAINER_RUNTIME" ps -a --format '{{.Names}}' | awk -F_ '/^declarest-keycloak-/{print $1}' | sort -u || true)
        project_names+=("${runtime_projects[@]}")
    fi
    if ((${#project_names[@]} > 0)); then
        mapfile -t unique_projects < <(printf "%s\n" "${project_names[@]}" | sort -u)
        for project_name in "${unique_projects[@]}"; do
            remove_project_artifacts "$project_name"
        done
    fi
    for work_dir in "${work_dirs[@]}"; do
        remove_work_dir "$work_dir" "$work_dir/home"
    done
    exit 0
fi

work_dir="${DECLAREST_WORK_DIR:-}"
home_dir="${DECLAREST_HOME_DIR:-}"

if [[ -z "$work_dir" ]]; then
    exit 0
fi

remove_project_artifacts "${COMPOSE_PROJECT_NAME:-}"
remove_work_dir "$work_dir" "$home_dir"
