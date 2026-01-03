#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"

project_name="${COMPOSE_PROJECT_NAME:-$(basename "$DECLAREST_COMPOSE_DIR")}"

log_line "Stopping Keycloak stack for project ${project_name}"

if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
    log_line "Container runtime ${CONTAINER_RUNTIME} not available; skipping container cleanup"
    exit 0
fi

if [ -d "$DECLAREST_COMPOSE_DIR" ] && [ -f "$DECLAREST_COMPOSE_DIR/compose.yml" ]; then
    (cd "$DECLAREST_COMPOSE_DIR" && run_logged "compose down" "$CONTAINER_RUNTIME" compose -p "$project_name" down --remove-orphans) || true
fi

mapfile -t compose_containers < <("$CONTAINER_RUNTIME" ps -aq --filter "label=com.docker.compose.project=${project_name}" || true)
if ((${#compose_containers[@]} > 0)); then
    run_logged "remove compose containers" "$CONTAINER_RUNTIME" rm -f "${compose_containers[@]}" || true
fi

mapfile -t compose_networks < <("$CONTAINER_RUNTIME" network ls -q --filter "label=com.docker.compose.project=${project_name}" || true)
if ((${#compose_networks[@]} > 0)); then
    run_logged "remove compose networks" "$CONTAINER_RUNTIME" network rm "${compose_networks[@]}" || true
fi

run_logged "remove container ${KEYCLOAK_CONTAINER_NAME}" "$CONTAINER_RUNTIME" rm -f "$KEYCLOAK_CONTAINER_NAME" || true
run_logged "remove container nginx" "$CONTAINER_RUNTIME" rm -f "nginx" || true
if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
    gitlab_container="${COMPOSE_PROJECT_NAME}_gitlab_1"
    run_logged "remove container ${gitlab_container}" "$CONTAINER_RUNTIME" rm -f "$gitlab_container" || true
    gitea_container="${COMPOSE_PROJECT_NAME}_gitea_1"
    run_logged "remove container ${gitea_container}" "$CONTAINER_RUNTIME" rm -f "$gitea_container" || true
fi

log_line "Keycloak stack stopped"
