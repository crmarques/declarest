#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/ports.sh"

cleanup_existing() {
    local project_name="${COMPOSE_PROJECT_NAME}"

    log_line "Cleaning existing containers for project ${project_name}"
    if [ -d "$DECLAREST_COMPOSE_DIR" ] && [ -f "$DECLAREST_COMPOSE_DIR/compose.yml" ]; then
        (cd "$DECLAREST_COMPOSE_DIR" && run_logged "compose down" "$CONTAINER_RUNTIME" compose -p "$project_name" down --remove-orphans)
    fi

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

cleanup_existing

KEYCLOAK_PORT_MIN=18080
KEYCLOAK_PORT_MAX=18180
VAULT_PORT_MIN=18200
VAULT_PORT_MAX=18299
GITEA_PORT_MIN=18082
GITEA_PORT_MAX=18182

keycloak_port_file="$DECLAREST_WORK_DIR/keycloak-port"
vault_port_file="$DECLAREST_WORK_DIR/vault-port"
gitea_http_file="$DECLAREST_WORK_DIR/gitea-http-port"

requested_keycloak_port="${KEYCLOAK_HTTP_PORT:-}"
requested_vault_port="${VAULT_HTTP_PORT:-}"
requested_gitea_port="${GITEA_HTTP_PORT:-}"

keycloak_port_start="$KEYCLOAK_PORT_MIN"
vault_port_start="$VAULT_PORT_MIN"
gitea_port_start="$GITEA_PORT_MIN"

declare -a keycloak_selected_port_log=()

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

secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-file}"
secret_store_type="${secret_store_type,,}"
vault_enabled=0
case "$secret_store_type" in
    vault)
        vault_enabled=1
        ;;
    none|file|"")
        ;;
    *)
        log_line "Unsupported secret store type: ${secret_store_type}"
        exit 1
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

mkdir -p "$DECLAREST_COMPOSE_DIR"
cp -R "$DECLAREST_TEST_DIR/templates/compose/." "$DECLAREST_COMPOSE_DIR"/
mkdir -p "$DECLAREST_COMPOSE_DIR/nginx-logs"
vault_data_dir="$DECLAREST_COMPOSE_DIR/vault-data"
mkdir -p "$vault_data_dir"
chmod 0777 "$vault_data_dir"

select_keycloak_port() {
    local requested="${requested_keycloak_port:-}"
    KEYCLOAK_HTTP_PORT="$(select_port "$requested" "$keycloak_port_start" "$KEYCLOAK_PORT_MAX")"
    if [[ -n "$requested" && "$KEYCLOAK_HTTP_PORT" != "$requested" ]]; then
        log_line "Keycloak port ${requested} is in use; using ${KEYCLOAK_HTTP_PORT}"
    fi
    export KEYCLOAK_HTTP_PORT
    printf "%s" "$KEYCLOAK_HTTP_PORT" > "$keycloak_port_file"
}

select_vault_port() {
    local requested="${requested_vault_port:-}"
    VAULT_HTTP_PORT="$(select_port "$requested" "$vault_port_start" "$VAULT_PORT_MAX")"
    if [[ -n "$requested" && "$VAULT_HTTP_PORT" != "$requested" ]]; then
        log_line "Vault port ${requested} is in use; using ${VAULT_HTTP_PORT}"
    fi
    export VAULT_HTTP_PORT
    printf "%s" "$VAULT_HTTP_PORT" > "$vault_port_file"
}

select_gitea_port() {
    local requested="${requested_gitea_port:-}"
    GITEA_HTTP_PORT="$(select_port "$requested" "$gitea_port_start" "$GITEA_PORT_MAX")"
    if [[ -n "$requested" && "$GITEA_HTTP_PORT" != "$requested" ]]; then
        log_line "Gitea port ${requested} is in use; using ${GITEA_HTTP_PORT}"
    fi
    export GITEA_HTTP_PORT
    printf "%s" "$GITEA_HTTP_PORT" > "$gitea_http_file"
}

write_compose_env() {
    log_line "Writing Keycloak compose environment to $DECLAREST_COMPOSE_DIR/.env"
    cat <<ENVFILE > "$DECLAREST_COMPOSE_DIR/.env"
KEYCLOAK_IMAGE=$KEYCLOAK_IMAGE
KEYCLOAK_CONTAINER_NAME=$KEYCLOAK_CONTAINER_NAME
KEYCLOAK_ADMIN_USER=$KEYCLOAK_ADMIN_USER
KEYCLOAK_ADMIN_PASS=$KEYCLOAK_ADMIN_PASS
KEYCLOAK_HTTP_PORT=$KEYCLOAK_HTTP_PORT
VAULT_IMAGE=$VAULT_IMAGE
VAULT_CONTAINER_NAME=$VAULT_CONTAINER_NAME
VAULT_HTTP_PORT=$VAULT_HTTP_PORT
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
}

select_keycloak_port
if [[ $vault_enabled -eq 1 ]]; then
    select_vault_port
else
    VAULT_HTTP_PORT="${requested_vault_port:-18200}"
    export VAULT_HTTP_PORT
    printf "%s" "$VAULT_HTTP_PORT" > "$vault_port_file"
fi
if [[ $gitea_enabled -eq 1 ]]; then
    select_gitea_port
else
    GITEA_HTTP_PORT="${requested_gitea_port:-18082}"
    export GITEA_HTTP_PORT
    printf "%s" "$GITEA_HTTP_PORT" > "$gitea_http_file"
fi
write_compose_env

compose_up_with_retry() {
    local attempt=1
    local max_attempts=3
    while true; do
        log_line "Starting Keycloak stack with podman (attempt ${attempt})"
        log_line "START compose up :: cd ${DECLAREST_COMPOSE_DIR} && $CONTAINER_RUNTIME compose -p ${COMPOSE_PROJECT_NAME} up -d"
        if (cd "$DECLAREST_COMPOSE_DIR" && "$CONTAINER_RUNTIME" compose -p "$COMPOSE_PROJECT_NAME" --profile gitea --profile vault up -d) >>"$RUN_LOG" 2>&1; then
            log_line "DONE  compose up"
            return 0
        fi
        log_line "FAIL  compose up (exit $?)"
        if (( attempt >= max_attempts )); then
            return 1
        fi
        if grep -q "bind: address already in use" "$RUN_LOG"; then
            log_line "Port conflict detected; selecting new Keycloak port (attempt ${attempt})"
            requested_keycloak_port=""
            keycloak_port_start=$((KEYCLOAK_HTTP_PORT + 1))
            select_keycloak_port
            write_compose_env
            attempt=$((attempt + 1))
            continue
        fi
        return 1
    done
}

compose_up_with_retry

log_line "Starting Keycloak stack with ${CONTAINER_RUNTIME}"
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
if [[ $vault_enabled -eq 1 ]]; then
    compose_profiles+=(--profile vault)
fi
(cd "$DECLAREST_COMPOSE_DIR" && run_logged "compose up" "$CONTAINER_RUNTIME" compose -p "$COMPOSE_PROJECT_NAME" "${compose_profiles[@]}" up -d)
