#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/ports.sh"
source "$SCRIPTS_DIR/lib/shell.sh"

resolve_image() {
    local image="$1"
    if [[ "${CONTAINER_RUNTIME:-}" != "podman" ]]; then
        printf "%s" "$image"
        return 0
    fi
    if [[ "$image" == */* ]]; then
        local registry="${image%%/*}"
        if [[ "$registry" == "localhost" || "$registry" == *.* || "$registry" == *:* ]]; then
            printf "%s" "$image"
            return 0
        fi
        printf "docker.io/%s" "$image"
        return 0
    fi
    printf "docker.io/library/%s" "$image"
}

require_cmd "$CONTAINER_RUNTIME"

if "$CONTAINER_RUNTIME" ps -a --format '{{.Names}}' | grep -q "^${RUNDECK_CONTAINER_NAME}$"; then
    run_logged "remove existing container" "$CONTAINER_RUNTIME" rm -f "$RUNDECK_CONTAINER_NAME"
fi

mkdir -p "$DECLAREST_WORK_DIR"
port_file="$DECLAREST_WORK_DIR/rundeck-port"
requested_port="${RUNDECK_HTTP_PORT:-}"
selected_port="$(select_port "$requested_port" 4440 4499)"
if [[ -z "$selected_port" ]]; then
    selected_port="$requested_port"
fi
if [[ -z "$selected_port" ]]; then
    selected_port="4440"
fi
if [[ -n "$requested_port" && "$selected_port" != "$requested_port" ]]; then
    log_line "Rundeck port ${requested_port} is in use; using ${selected_port}"
fi
RUNDECK_HTTP_PORT="$selected_port"
export RUNDECK_HTTP_PORT
export RUNDECK_PORT="$RUNDECK_HTTP_PORT"
printf "%s" "$RUNDECK_HTTP_PORT" > "$port_file"

base_url="$RUNDECK_BASE_URL"
if [[ -z "$base_url" || "$base_url" == http://localhost* || "$base_url" == http://127.0.0.1* || "$base_url" == http://0.0.0.0* ]]; then
    base_url="http://localhost:${RUNDECK_HTTP_PORT}"
fi
RUNDECK_BASE_URL="$base_url"
export RUNDECK_BASE_URL

resolved_image="$(resolve_image "$RUNDECK_IMAGE")"
if [[ "$resolved_image" != "$RUNDECK_IMAGE" ]]; then
    log_line "Resolved Rundeck image $RUNDECK_IMAGE -> $resolved_image"
fi

log_line "Starting Rundeck container ($resolved_image)"
run_logged "rundeck run" "$CONTAINER_RUNTIME" run -d --rm \
    --name "$RUNDECK_CONTAINER_NAME" \
    -p "${RUNDECK_HTTP_PORT}:4440" \
    -e RUNDECK_GRAILS_URL="$RUNDECK_BASE_URL" \
    -e RUNDECK_ADMIN_USER="$RUNDECK_USER" \
    -e RUNDECK_ADMIN_PASSWORD="$RUNDECK_PASSWORD" \
    -e RUNDECK_ADMIN_ROLES="admin" \
    "$resolved_image"
