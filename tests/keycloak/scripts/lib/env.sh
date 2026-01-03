#!/usr/bin/env bash

# Common environment variables for the Keycloak test harness.
export DECLAREST_TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export DECLAREST_RUN_ID="${DECLAREST_RUN_ID:-$(date +%Y%m%dT%H%M%S)}"
if [[ -z "${DECLAREST_WORK_DIR:-}" ]]; then
    export DECLAREST_WORK_BASE_DIR="${DECLAREST_WORK_BASE_DIR:-/tmp}"
    export DECLAREST_WORK_DIR="${DECLAREST_WORK_BASE_DIR%/}/declarest-keycloak-${DECLAREST_RUN_ID}"
fi
export DECLAREST_BIN_DIR="$DECLAREST_WORK_DIR/bin"
if [[ -z "${DECLAREST_OUTPUT_GIT_CLONE_DIR:-}" ]]; then
    export DECLAREST_OUTPUT_GIT_CLONE_DIR="$DECLAREST_WORK_DIR/output-git-clone"
else
    export DECLAREST_OUTPUT_GIT_CLONE_DIR
fi
if [[ -z "${DECLAREST_REPO_DIR:-}" ]]; then
    export DECLAREST_REPO_DIR="$DECLAREST_WORK_DIR/repo"
else
    export DECLAREST_REPO_DIR
fi
if [[ -z "${DECLAREST_CONTEXT_FILE:-}" ]]; then
    export DECLAREST_CONTEXT_FILE="$DECLAREST_WORK_DIR/context.yaml"
else
    export DECLAREST_CONTEXT_FILE
fi
export DECLAREST_LOG_DIR="$DECLAREST_WORK_DIR/logs"
export DECLAREST_COMPOSE_DIR="$DECLAREST_WORK_DIR/compose"
export DECLAREST_HOME_DIR="$DECLAREST_WORK_DIR/home"
export DECLAREST_SECRETS_FILE="$DECLAREST_WORK_DIR/secrets.json"
export DECLAREST_TEMPLATE_REPO_DIR="$(cd "$DECLAREST_TEST_DIR/templates/repo" && pwd)"
if [[ -z "${DECLAREST_REPO_REMOTE_URL+x}" ]]; then
    export DECLAREST_REPO_REMOTE_URL="https://github.com/crmarques/declarest-keycloak.git"
else
    export DECLAREST_REPO_REMOTE_URL
fi
if [[ -z "${DECLAREST_REPO_TYPE:-}" ]]; then
    if [[ -n "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
        export DECLAREST_REPO_TYPE="git-remote"
    else
        export DECLAREST_REPO_TYPE="git-local"
    fi
else
    export DECLAREST_REPO_TYPE
fi
export DECLAREST_REPO_PROVIDER="${DECLAREST_REPO_PROVIDER:-}"
export DECLAREST_REPO_AUTH_TYPE="${DECLAREST_REPO_AUTH_TYPE:-}"
export DECLAREST_REPO_AUTH="${DECLAREST_REPO_AUTH:-}"
export DECLAREST_REPO_SSH_USER="${DECLAREST_REPO_SSH_USER:-}"
export DECLAREST_REPO_SSH_KEY_FILE="${DECLAREST_REPO_SSH_KEY_FILE:-}"
export DECLAREST_REPO_SSH_PASSPHRASE="${DECLAREST_REPO_SSH_PASSPHRASE:-}"
export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}"
export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY="${DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY:-}"
export DECLAREST_GITLAB_ENABLE="${DECLAREST_GITLAB_ENABLE:-0}"
export DECLAREST_GITEA_ENABLE="${DECLAREST_GITEA_ENABLE:-0}"
export DECLAREST_REMOTE_REPO_PROVIDER="${DECLAREST_REMOTE_REPO_PROVIDER:-}"
export DECLAREST_SERVER_AUTH_TYPE="${DECLAREST_SERVER_AUTH_TYPE:-oauth2}"
export DECLAREST_KEEP_WORK="${DECLAREST_KEEP_WORK:-0}"
export DECLAREST_SECRETS_PASSPHRASE="${DECLAREST_SECRETS_PASSPHRASE:-declarest-e2e-secret}"
export DECLAREST_TEST_CLIENT_SECRET="${DECLAREST_TEST_CLIENT_SECRET:-declarest-client-secret}"
export DECLAREST_TEST_LDAP_BIND_CREDENTIAL="${DECLAREST_TEST_LDAP_BIND_CREDENTIAL:-declarest-ldap-bind}"

# Container runtime (docker or podman)
export CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
if [[ -z "${COMPOSE_PROJECT_NAME:-}" ]]; then
    export COMPOSE_PROJECT_NAME="$(basename "$DECLAREST_WORK_DIR")"
fi

# Keycloak container settings
if [[ -z "${KEYCLOAK_CONTAINER_NAME:-}" ]]; then
    if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
        export KEYCLOAK_CONTAINER_NAME="${COMPOSE_PROJECT_NAME}_keycloak-declarest-test_1"
    else
        export KEYCLOAK_CONTAINER_NAME="keycloak-declarest-test"
    fi
else
    export KEYCLOAK_CONTAINER_NAME
fi
export KEYCLOAK_IMAGE="${KEYCLOAK_IMAGE:-quay.io/keycloak/keycloak:26.2.5}"
export KEYCLOAK_ADMIN_USER="${KEYCLOAK_ADMIN_USER:-admin}"
export KEYCLOAK_ADMIN_PASS="${KEYCLOAK_ADMIN_PASS:-admin}"
if [[ -z "${KEYCLOAK_HTTP_PORT:-}" ]]; then
    port=""
    if [[ -n "${DECLAREST_WORK_DIR:-}" && -f "$DECLAREST_WORK_DIR/keycloak-port" ]]; then
        port=$(tr -d ' \t\r\n' < "$DECLAREST_WORK_DIR/keycloak-port")
    fi
    if [[ -z "$port" ]]; then
        port="18080"
    fi
    export KEYCLOAK_HTTP_PORT="$port"
else
    export KEYCLOAK_HTTP_PORT
fi

# GitLab container settings (used for git-remote tests when enabled)
export GITLAB_IMAGE="${GITLAB_IMAGE:-docker.io/gitlab/gitlab-ce:16.9.1-ce.0}"
export GITLAB_HOSTNAME="${GITLAB_HOSTNAME:-gitlab}"
export GITLAB_ROOT_PASSWORD="${GITLAB_ROOT_PASSWORD:-Dcl9T7pR2X5mZ${DECLAREST_RUN_ID}}"
export GITLAB_ROOT_EMAIL="${GITLAB_ROOT_EMAIL:-root@example.com}"
export GITLAB_USER="${GITLAB_USER:-declarest}"
export GITLAB_USER_PASSWORD="${GITLAB_USER_PASSWORD:-KeycloakE2e1!}"
export GITLAB_USER_EMAIL="${GITLAB_USER_EMAIL:-declarest@example.com}"
export GITLAB_PROJECT_PREFIX="${GITLAB_PROJECT_PREFIX:-declarest-${DECLAREST_RUN_ID}}"
if [[ -z "${GITLAB_HTTP_PORT:-}" ]]; then
    gitlab_port=""
    if [[ -n "${DECLAREST_WORK_DIR:-}" && -f "$DECLAREST_WORK_DIR/gitlab-http-port" ]]; then
        gitlab_port=$(tr -d ' \t\r\n' < "$DECLAREST_WORK_DIR/gitlab-http-port")
    fi
    if [[ -z "$gitlab_port" ]]; then
        gitlab_port="18081"
    fi
    export GITLAB_HTTP_PORT="$gitlab_port"
else
    export GITLAB_HTTP_PORT
fi
if [[ -z "${GITLAB_SSH_PORT:-}" ]]; then
    gitlab_ssh_port=""
    if [[ -n "${DECLAREST_WORK_DIR:-}" && -f "$DECLAREST_WORK_DIR/gitlab-ssh-port" ]]; then
        gitlab_ssh_port=$(tr -d ' \t\r\n' < "$DECLAREST_WORK_DIR/gitlab-ssh-port")
    fi
    if [[ -z "$gitlab_ssh_port" ]]; then
        gitlab_ssh_port="2222"
    fi
    export GITLAB_SSH_PORT="$gitlab_ssh_port"
else
    export GITLAB_SSH_PORT
fi

# Gitea container settings (used for git-remote tests when enabled)
export GITEA_IMAGE="${GITEA_IMAGE:-docker.io/gitea/gitea:1.25.3-rootless}"
export GITEA_HOSTNAME="${GITEA_HOSTNAME:-gitea}"
export GITEA_USER="${GITEA_USER:-declarest}"
export GITEA_USER_PASSWORD="${GITEA_USER_PASSWORD:-declarest-pass}"
export GITEA_USER_EMAIL="${GITEA_USER_EMAIL:-declarest@example.com}"
export GITEA_PROJECT_PREFIX="${GITEA_PROJECT_PREFIX:-declarest-${DECLAREST_RUN_ID}}"
if [[ -z "${GITEA_HTTP_PORT:-}" ]]; then
    gitea_port=""
    if [[ -n "${DECLAREST_WORK_DIR:-}" && -f "$DECLAREST_WORK_DIR/gitea-http-port" ]]; then
        gitea_port=$(tr -d ' \t\r\n' < "$DECLAREST_WORK_DIR/gitea-http-port")
    fi
    if [[ -z "$gitea_port" ]]; then
        gitea_port="18082"
    fi
    export GITEA_HTTP_PORT="$gitea_port"
else
    export GITEA_HTTP_PORT
fi
if [[ -z "${GITEA_SSH_PORT:-}" ]]; then
    gitea_ssh_port=""
    if [[ -n "${DECLAREST_WORK_DIR:-}" && -f "$DECLAREST_WORK_DIR/gitea-ssh-port" ]]; then
        gitea_ssh_port=$(tr -d ' \t\r\n' < "$DECLAREST_WORK_DIR/gitea-ssh-port")
    fi
    if [[ -z "$gitea_ssh_port" ]]; then
        gitea_ssh_port="2223"
    fi
    export GITEA_SSH_PORT="$gitea_ssh_port"
else
    export GITEA_SSH_PORT
fi
