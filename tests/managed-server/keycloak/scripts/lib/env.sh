#!/usr/bin/env bash

# Common environment variables for the Keycloak test harness.
export DECLAREST_TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export DECLAREST_TESTS_ROOT="$(cd "$DECLAREST_TEST_DIR/../.." && pwd)"
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
export DECLAREST_SECRET_STORE_TYPE="${DECLAREST_SECRET_STORE_TYPE:-file}"
export DECLAREST_TEMPLATE_REPO_DIR="$(cd "$DECLAREST_TEST_DIR/templates/repo" && pwd)"
export DECLAREST_OPENAPI_SPEC="${DECLAREST_OPENAPI_SPEC:-$DECLAREST_TEST_DIR/templates/openapi.yaml}"
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
export DECLAREST_SERVER_BEARER_TOKEN="${DECLAREST_SERVER_BEARER_TOKEN:-}"
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
keycloak_port_file=""
keycloak_port=""
if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
    keycloak_port_file="$DECLAREST_WORK_DIR/keycloak-port"
fi
if [[ -n "$keycloak_port_file" && -f "$keycloak_port_file" ]]; then
    keycloak_port="$(tr -d ' \t\r\n' < "$keycloak_port_file")"
fi
if [[ -n "$keycloak_port" ]]; then
    export KEYCLOAK_HTTP_PORT="$keycloak_port"
elif [[ -n "${KEYCLOAK_HTTP_PORT:-}" ]]; then
    export KEYCLOAK_HTTP_PORT
else
    export KEYCLOAK_HTTP_PORT="18080"
fi

# Vault container settings (used for vault secret store tests when enabled)
export VAULT_IMAGE="${VAULT_IMAGE:-docker.io/hashicorp/vault:1.17.2}"
if [[ -z "${VAULT_CONTAINER_NAME:-}" ]]; then
    if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
        export VAULT_CONTAINER_NAME="${COMPOSE_PROJECT_NAME}_vault_1"
    else
        export VAULT_CONTAINER_NAME="vault-declarest-test"
    fi
else
    export VAULT_CONTAINER_NAME
fi
vault_port_file=""
vault_port=""
if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
    vault_port_file="$DECLAREST_WORK_DIR/vault-port"
fi
if [[ -n "$vault_port_file" && -f "$vault_port_file" ]]; then
    vault_port="$(tr -d ' \t\r\n' < "$vault_port_file")"
fi
if [[ -n "$vault_port" ]]; then
    export VAULT_HTTP_PORT="$vault_port"
elif [[ -n "${VAULT_HTTP_PORT:-}" ]]; then
    export VAULT_HTTP_PORT
else
    export VAULT_HTTP_PORT="18200"
fi
export DECLAREST_VAULT_ADDR="${DECLAREST_VAULT_ADDR:-http://localhost:${VAULT_HTTP_PORT}}"
export DECLAREST_VAULT_MOUNT="${DECLAREST_VAULT_MOUNT:-secret}"
export DECLAREST_VAULT_PATH_PREFIX="${DECLAREST_VAULT_PATH_PREFIX:-declarest}"
export DECLAREST_VAULT_KV_VERSION="${DECLAREST_VAULT_KV_VERSION:-2}"
export DECLAREST_VAULT_AUTH_TYPE="${DECLAREST_VAULT_AUTH_TYPE:-token}"
export DECLAREST_VAULT_TOKEN="${DECLAREST_VAULT_TOKEN:-}"
export DECLAREST_VAULT_UNSEAL_KEY="${DECLAREST_VAULT_UNSEAL_KEY:-}"
export DECLAREST_VAULT_USERNAME="${DECLAREST_VAULT_USERNAME:-declarest}"
export DECLAREST_VAULT_PASSWORD="${DECLAREST_VAULT_PASSWORD:-declarest-pass}"
export DECLAREST_VAULT_ROLE_ID="${DECLAREST_VAULT_ROLE_ID:-}"
export DECLAREST_VAULT_SECRET_ID="${DECLAREST_VAULT_SECRET_ID:-}"
export DECLAREST_VAULT_USERPASS_MOUNT="${DECLAREST_VAULT_USERPASS_MOUNT:-}"
export DECLAREST_VAULT_APPROLE_MOUNT="${DECLAREST_VAULT_APPROLE_MOUNT:-}"
export DECLAREST_VAULT_CA_CERT_FILE="${DECLAREST_VAULT_CA_CERT_FILE:-}"
export DECLAREST_VAULT_CLIENT_CERT_FILE="${DECLAREST_VAULT_CLIENT_CERT_FILE:-}"
export DECLAREST_VAULT_CLIENT_KEY_FILE="${DECLAREST_VAULT_CLIENT_KEY_FILE:-}"
export DECLAREST_VAULT_INSECURE_SKIP_VERIFY="${DECLAREST_VAULT_INSECURE_SKIP_VERIFY:-}"

# GitLab container settings (used for git-remote tests when enabled)
export GITLAB_IMAGE="${GITLAB_IMAGE:-docker.io/gitlab/gitlab-ce:16.9.1-ce.0}"
export GITLAB_HOSTNAME="${GITLAB_HOSTNAME:-gitlab}"
export GITLAB_ROOT_PASSWORD="${GITLAB_ROOT_PASSWORD:-Dcl9T7pR2X5mZ${DECLAREST_RUN_ID}}"
export GITLAB_ROOT_EMAIL="${GITLAB_ROOT_EMAIL:-root@example.com}"
export GITLAB_USER="${GITLAB_USER:-declarest}"
export GITLAB_USER_PASSWORD="${GITLAB_USER_PASSWORD:-KeycloakE2e1!}"
export GITLAB_USER_EMAIL="${GITLAB_USER_EMAIL:-declarest@example.com}"
export GITLAB_PROJECT_PREFIX="${GITLAB_PROJECT_PREFIX:-declarest-${DECLAREST_RUN_ID}}"
gitlab_http_port_file=""
gitlab_http_port=""
if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
    gitlab_http_port_file="$DECLAREST_WORK_DIR/gitlab-http-port"
fi
if [[ -n "$gitlab_http_port_file" && -f "$gitlab_http_port_file" ]]; then
    gitlab_http_port="$(tr -d ' \t\r\n' < "$gitlab_http_port_file")"
fi
if [[ -n "$gitlab_http_port" ]]; then
    export GITLAB_HTTP_PORT="$gitlab_http_port"
elif [[ -n "${GITLAB_HTTP_PORT:-}" ]]; then
    export GITLAB_HTTP_PORT
else
    export GITLAB_HTTP_PORT="18081"
fi

gitlab_ssh_port_file=""
gitlab_ssh_port=""
if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
    gitlab_ssh_port_file="$DECLAREST_WORK_DIR/gitlab-ssh-port"
fi
if [[ -n "$gitlab_ssh_port_file" && -f "$gitlab_ssh_port_file" ]]; then
    gitlab_ssh_port="$(tr -d ' \t\r\n' < "$gitlab_ssh_port_file")"
fi
if [[ -n "$gitlab_ssh_port" ]]; then
    export GITLAB_SSH_PORT="$gitlab_ssh_port"
elif [[ -n "${GITLAB_SSH_PORT:-}" ]]; then
    export GITLAB_SSH_PORT
else
    export GITLAB_SSH_PORT="2222"
fi

# Gitea container settings (used for git-remote tests when enabled)
export GITEA_IMAGE="${GITEA_IMAGE:-docker.io/gitea/gitea:1.25.3-rootless}"
export GITEA_HOSTNAME="${GITEA_HOSTNAME:-gitea}"
export GITEA_USER="${GITEA_USER:-declarest}"
export GITEA_USER_PASSWORD="${GITEA_USER_PASSWORD:-declarest-pass}"
export GITEA_USER_EMAIL="${GITEA_USER_EMAIL:-declarest@example.com}"
export GITEA_PROJECT_PREFIX="${GITEA_PROJECT_PREFIX:-declarest-${DECLAREST_RUN_ID}}"
gitea_http_port_file=""
gitea_http_port=""
if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
    gitea_http_port_file="$DECLAREST_WORK_DIR/gitea-http-port"
fi
if [[ -n "$gitea_http_port_file" && -f "$gitea_http_port_file" ]]; then
    gitea_http_port="$(tr -d ' \t\r\n' < "$gitea_http_port_file")"
fi
if [[ -n "$gitea_http_port" ]]; then
    export GITEA_HTTP_PORT="$gitea_http_port"
elif [[ -n "${GITEA_HTTP_PORT:-}" ]]; then
    export GITEA_HTTP_PORT
else
    export GITEA_HTTP_PORT="18082"
fi

gitea_ssh_port_file=""
gitea_ssh_port=""
if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
    gitea_ssh_port_file="$DECLAREST_WORK_DIR/gitea-ssh-port"
fi
if [[ -n "$gitea_ssh_port_file" && -f "$gitea_ssh_port_file" ]]; then
    gitea_ssh_port="$(tr -d ' \t\r\n' < "$gitea_ssh_port_file")"
fi
if [[ -n "$gitea_ssh_port" ]]; then
    export GITEA_SSH_PORT="$gitea_ssh_port"
elif [[ -n "${GITEA_SSH_PORT:-}" ]]; then
    export GITEA_SSH_PORT
else
    export GITEA_SSH_PORT="2223"
fi

vault_env_file="${DECLAREST_WORK_DIR:-}/vault.env"
if [[ -n "$vault_env_file" && -f "$vault_env_file" ]]; then
    # shellcheck source=/dev/null
    source "$vault_env_file"
fi
