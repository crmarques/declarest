#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/ports.sh
source "$SCRIPTS_DIR/lib/ports.sh"

cleanup_existing() {
    local project_name="${COMPOSE_PROJECT_NAME}"

    log_line "Cleaning existing containers for project ${project_name}"
    if [ -d "$DECLAREST_COMPOSE_DIR" ] && [ -f "$DECLAREST_COMPOSE_DIR/compose.yml" ]; then
        (cd "$DECLAREST_COMPOSE_DIR" && run_logged "compose down" "$CONTAINER_RUNTIME" compose -p "$project_name" down --remove-orphans)
    fi

    # Fallback cleanup if compose metadata is gone (e.g., work dir wiped).
    mapfile -t compose_containers < <("$CONTAINER_RUNTIME" ps -aq --filter "label=com.docker.compose.project=${project_name}")
    if ((${#compose_containers[@]} > 0)); then
        run_logged "remove compose containers" "$CONTAINER_RUNTIME" rm -f "${compose_containers[@]}"
    fi
    mapfile -t compose_networks < <("$CONTAINER_RUNTIME" network ls -q --filter "label=com.docker.compose.project=${project_name}")
    if ((${#compose_networks[@]} > 0)); then
        run_logged "remove compose networks" "$CONTAINER_RUNTIME" network rm "${compose_networks[@]}"
    fi

    if "$CONTAINER_RUNTIME" ps -a --format '{{.Names}}' | grep -q "^${KEYCLOAK_CONTAINER_NAME}$"; then
        run_logged "remove stray container" "$CONTAINER_RUNTIME" rm -f "$KEYCLOAK_CONTAINER_NAME"
    fi
}

wait_for_keycloak() {
    local url="http://localhost:${KEYCLOAK_HTTP_PORT}/realms/master"
    local attempts=${KEYCLOAK_WAIT_ATTEMPTS:-150}
    local delay=${KEYCLOAK_WAIT_DELAY:-2}

    log_line "Waiting for Keycloak to become ready at $url (${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        if curl -sk --fail "$url" >/dev/null 2>&1; then
            log_line "Keycloak is up after attempt ${i}"
            return 0
        fi
        if (( i % 10 == 0 )); then
            log_line "Still waiting for Keycloak (${i}/${attempts})"
        fi
        sleep "$delay"
    done

    log_line "Keycloak did not become ready in time"
    local status_output recent_logs
    status_output=$("$CONTAINER_RUNTIME" ps --all --filter "name=${KEYCLOAK_CONTAINER_NAME}" || true)
    recent_logs=$("$CONTAINER_RUNTIME" logs "${KEYCLOAK_CONTAINER_NAME}" 2>&1 | tail -n 50 || true)
    log_block "Container status" "$status_output"
    log_block "Recent container logs" "$recent_logs"
    return 1
}

cleanup_existing

mkdir -p "$DECLAREST_WORK_DIR"
port_file="$DECLAREST_WORK_DIR/keycloak-port"
requested_port="${KEYCLOAK_HTTP_PORT:-}"
selected_port="$(select_port "$requested_port" 18080 18180)"
if [[ -z "$selected_port" ]]; then
    selected_port="$requested_port"
fi
if [[ -z "$selected_port" ]]; then
    selected_port="18080"
fi
if [[ -n "$requested_port" && "$selected_port" != "$requested_port" ]]; then
    log_line "Keycloak port ${requested_port} is in use; using ${selected_port}"
fi
KEYCLOAK_HTTP_PORT="$selected_port"
export KEYCLOAK_HTTP_PORT
printf "%s" "$KEYCLOAK_HTTP_PORT" > "$port_file"

gitlab_enabled=0
gitea_enabled=0
case "${DECLAREST_GITLAB_ENABLE:-0}" in
    1|true|yes|y)
        gitlab_enabled=1
        ;;
esac
case "${DECLAREST_GITEA_ENABLE:-0}" in
    1|true|yes|y)
        gitea_enabled=1
        ;;
esac

remote_provider="${DECLAREST_REMOTE_REPO_PROVIDER:-}"
remote_provider="${remote_provider,,}"
case "$remote_provider" in
    gitlab)
        gitlab_enabled=1
        gitea_enabled=0
        ;;
    gitea)
        gitea_enabled=1
        gitlab_enabled=0
        ;;
esac

if [[ $gitlab_enabled -eq 1 ]]; then
    gitlab_http_file="$DECLAREST_WORK_DIR/gitlab-http-port"
    gitlab_ssh_file="$DECLAREST_WORK_DIR/gitlab-ssh-port"

    requested_gitlab_port="${GITLAB_HTTP_PORT:-}"
    selected_gitlab_port="$(select_port "$requested_gitlab_port" 18081 18180)"
    if [[ -z "$selected_gitlab_port" ]]; then
        selected_gitlab_port="$requested_gitlab_port"
    fi
    if [[ -z "$selected_gitlab_port" ]]; then
        selected_gitlab_port="18081"
    fi
    GITLAB_HTTP_PORT="$selected_gitlab_port"
    export GITLAB_HTTP_PORT
    printf "%s" "$GITLAB_HTTP_PORT" > "$gitlab_http_file"

    requested_gitlab_ssh="${GITLAB_SSH_PORT:-}"
    selected_gitlab_ssh="$(select_port "$requested_gitlab_ssh" 2222 2299)"
    if [[ -z "$selected_gitlab_ssh" ]]; then
        selected_gitlab_ssh="$requested_gitlab_ssh"
    fi
    if [[ -z "$selected_gitlab_ssh" ]]; then
        selected_gitlab_ssh="2222"
    fi
    GITLAB_SSH_PORT="$selected_gitlab_ssh"
    export GITLAB_SSH_PORT
    printf "%s" "$GITLAB_SSH_PORT" > "$gitlab_ssh_file"
fi

if [[ $gitea_enabled -eq 1 ]]; then
    gitea_http_file="$DECLAREST_WORK_DIR/gitea-http-port"
    gitea_ssh_file="$DECLAREST_WORK_DIR/gitea-ssh-port"

    requested_gitea_port="${GITEA_HTTP_PORT:-}"
    selected_gitea_port="$(select_port "$requested_gitea_port" 18082 18180)"
    if [[ -z "$selected_gitea_port" ]]; then
        selected_gitea_port="$requested_gitea_port"
    fi
    if [[ -z "$selected_gitea_port" ]]; then
        selected_gitea_port="18082"
    fi
    GITEA_HTTP_PORT="$selected_gitea_port"
    export GITEA_HTTP_PORT
    printf "%s" "$GITEA_HTTP_PORT" > "$gitea_http_file"

    requested_gitea_ssh="${GITEA_SSH_PORT:-}"
    selected_gitea_ssh="$(select_port "$requested_gitea_ssh" 2223 2299)"
    if [[ -z "$selected_gitea_ssh" ]]; then
        selected_gitea_ssh="$requested_gitea_ssh"
    fi
    if [[ -z "$selected_gitea_ssh" ]]; then
        selected_gitea_ssh="2223"
    fi
    GITEA_SSH_PORT="$selected_gitea_ssh"
    export GITEA_SSH_PORT
    printf "%s" "$GITEA_SSH_PORT" > "$gitea_ssh_file"
fi

mkdir -p "$DECLAREST_COMPOSE_DIR"
cp -R "$DECLAREST_TEST_DIR/templates/compose/." "$DECLAREST_COMPOSE_DIR"/
mkdir -p "$DECLAREST_COMPOSE_DIR/nginx-logs"

log_line "Writing Keycloak compose environment to $DECLAREST_COMPOSE_DIR/.env"
cat <<ENVFILE > "$DECLAREST_COMPOSE_DIR/.env"
KEYCLOAK_IMAGE=$KEYCLOAK_IMAGE
KEYCLOAK_CONTAINER_NAME=$KEYCLOAK_CONTAINER_NAME
KEYCLOAK_ADMIN_USER=$KEYCLOAK_ADMIN_USER
KEYCLOAK_ADMIN_PASS=$KEYCLOAK_ADMIN_PASS
KEYCLOAK_HTTP_PORT=$KEYCLOAK_HTTP_PORT
GITLAB_IMAGE=$GITLAB_IMAGE
GITLAB_HOSTNAME=$GITLAB_HOSTNAME
GITLAB_ROOT_PASSWORD=$GITLAB_ROOT_PASSWORD
GITLAB_HTTP_PORT=$GITLAB_HTTP_PORT
GITLAB_SSH_PORT=$GITLAB_SSH_PORT
GITEA_IMAGE=$GITEA_IMAGE
GITEA_HOSTNAME=$GITEA_HOSTNAME
GITEA_HTTP_PORT=$GITEA_HTTP_PORT
GITEA_SSH_PORT=$GITEA_SSH_PORT
ENVFILE

log_line "Starting Keycloak container with ${CONTAINER_RUNTIME}"
if [[ "${CONTAINER_RUNTIME}" == "podman" ]]; then
    runtime_dir="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
    mkdir -p "$runtime_dir/containers/networks" >/dev/null 2>&1 || true
fi
compose_profiles=()
if [[ $gitlab_enabled -eq 1 ]]; then
    compose_profiles+=(--profile gitlab)
fi
if [[ $gitea_enabled -eq 1 ]]; then
    compose_profiles+=(--profile gitea)
fi
(cd "$DECLAREST_COMPOSE_DIR" && run_logged "compose up" "$CONTAINER_RUNTIME" compose -p "$COMPOSE_PROJECT_NAME" "${compose_profiles[@]}" up -d)

wait_for_keycloak
