#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"
# shellcheck source=../lib/repo.sh
source "$SCRIPTS_DIR/lib/repo.sh"

mkdir -p "$DECLAREST_HOME_DIR"
mkdir -p "$DECLAREST_LOG_DIR"

tpl_config="$DECLAREST_TEST_DIR/templates/config.yaml"

yaml_quote() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    printf "\"%s\"" "$value"
}

sed_escape() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//&/\\&}"
    value="${value//#/\\#}"
    printf "%s" "$value"
}

repo_type="$(resolve_repo_type)"
if ! valid_repo_type "$repo_type"; then
    die "Unsupported repo type: ${repo_type} (expected fs, git-local, or git-remote)"
fi

server_auth_type="${DECLAREST_SERVER_AUTH_TYPE:-oauth2}"
server_auth_type="${server_auth_type,,}"
if [[ -z "$server_auth_type" ]]; then
    server_auth_type="oauth2"
fi

case "$server_auth_type" in
    oauth2|basic|bearer|bearer-token|bearer_token)
        ;;
    *)
        die "Unsupported server auth type: ${server_auth_type} (expected oauth2, basic, or bearer)"
        ;;
esac

repo_provider="${DECLAREST_REPO_PROVIDER:-}"
repo_provider="${repo_provider,,}"
if [[ -n "$repo_provider" ]]; then
    case "$repo_provider" in
        gitlab|github|gitea)
            ;;
        *)
            die "Unsupported git provider: ${repo_provider} (expected gitlab, github, or gitea)"
            ;;
    esac
fi

repo_auth_type="${DECLAREST_REPO_AUTH_TYPE:-}"
repo_auth_type="${repo_auth_type,,}"
repo_auth_value="${DECLAREST_REPO_AUTH:-}"
if [[ "$repo_auth_type" == "token" ]]; then
    repo_auth_type="pat"
fi
repo_ssh_user="${DECLAREST_REPO_SSH_USER:-}"
repo_ssh_key_file="${DECLAREST_REPO_SSH_KEY_FILE:-}"
repo_ssh_passphrase="${DECLAREST_REPO_SSH_PASSPHRASE:-}"
repo_ssh_known_hosts="${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}"
repo_ssh_insecure="${DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY:-}"
auth_block=""

if [[ "$repo_type" == "git-remote" ]]; then
    if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
        die "git-remote repo type requires DECLAREST_REPO_REMOTE_URL"
    fi

    case "$repo_auth_type" in
        "")
            if [[ -n "$repo_auth_value" || -n "$repo_ssh_user" || -n "$repo_ssh_key_file" || -n "$repo_ssh_passphrase" || -n "$repo_ssh_known_hosts" || -n "$repo_ssh_insecure" ]]; then
                die "Auth value provided without auth type"
            fi
            ;;
        basic)
            if [[ -z "$repo_auth_value" ]]; then
                die "Auth type basic requires DECLAREST_REPO_AUTH"
            fi
            if [[ "$repo_auth_value" != *:* ]]; then
                die "Basic auth must be in '<user>:<password>' format"
            fi
            repo_user="${repo_auth_value%%:*}"
            repo_pass="${repo_auth_value#*:}"
            if [[ -z "$repo_user" || -z "$repo_pass" ]]; then
                die "Basic auth requires both username and password"
            fi
            auth_block=$'      auth:\n        basic_auth:\n          username: '"$(yaml_quote "$repo_user")"$'\n          password: '"$(yaml_quote "$repo_pass")"
            ;;
        pat)
            if [[ -z "$repo_auth_value" ]]; then
                die "Auth type pat requires DECLAREST_REPO_AUTH"
            fi
            if [[ "$repo_provider" == "gitea" && -n "${DECLAREST_GITEA_USER:-}" ]]; then
                auth_block=$'      auth:\n        basic_auth:\n          username: '"$(yaml_quote "$DECLAREST_GITEA_USER")"$'\n          password: '"$(yaml_quote "$repo_auth_value")"
            else
                auth_block=$'      auth:\n        access_key:\n          token: '"$(yaml_quote "$repo_auth_value")"
            fi
            ;;
        ssh)
            if [[ -n "$repo_auth_value" ]]; then
                die "SSH auth does not use DECLAREST_REPO_AUTH"
            fi
            if [[ -z "$repo_ssh_key_file" ]]; then
                die "Auth type ssh requires DECLAREST_REPO_SSH_KEY_FILE"
            fi
            auth_block=$'      auth:\n        ssh:\n'
            if [[ -n "$repo_ssh_user" ]]; then
                auth_block+=$'          user: '"$(yaml_quote "$repo_ssh_user")"$'\n'
            fi
            auth_block+=$'          private_key_file: '"$(yaml_quote "$repo_ssh_key_file")"$'\n'
            if [[ -n "$repo_ssh_passphrase" ]]; then
                auth_block+=$'          passphrase: '"$(yaml_quote "$repo_ssh_passphrase")"$'\n'
            fi
            if [[ -n "$repo_ssh_known_hosts" ]]; then
                auth_block+=$'          known_hosts_file: '"$(yaml_quote "$repo_ssh_known_hosts")"$'\n'
            fi
            if [[ -n "$repo_ssh_insecure" ]]; then
                case "${repo_ssh_insecure,,}" in
                    1|true|yes|y)
                        auth_block+=$'          insecure_ignore_host_key: true\n'
                        ;;
                    0|false|no|n)
                        auth_block+=$'          insecure_ignore_host_key: false\n'
                        ;;
                    *)
                        die "Unsupported DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY value: ${repo_ssh_insecure}"
                        ;;
                esac
            fi
            auth_block="${auth_block%$'\n'}"
            ;;
        *)
            die "Unsupported auth type: ${repo_auth_type} (expected basic, pat, or ssh)"
            ;;
    esac
fi

repo_block=""
case "$repo_type" in
    fs)
        repo_block=$'repository:\n  filesystem:\n    base_dir: '"$(yaml_quote "$DECLAREST_REPO_DIR")"
        ;;
    git-local)
        repo_block=$'repository:\n  git:\n    local:\n      base_dir: '"$(yaml_quote "$DECLAREST_REPO_DIR")"
        ;;
    git-remote)
        repo_block=$'repository:\n  git:\n    local:\n      base_dir: '"$(yaml_quote "$DECLAREST_REPO_DIR")"$'\n    remote:\n      url: '"$(yaml_quote "$DECLAREST_REPO_REMOTE_URL")"
        if [[ -n "$repo_provider" ]]; then
            repo_block+=$'\n      provider: '"$(yaml_quote "$repo_provider")"
        fi
        if [[ -n "$auth_block" ]]; then
            repo_block+=$'\n'"$auth_block"
        fi
        ;;
esac

keycloak_url="http://localhost:${KEYCLOAK_HTTP_PORT}"
keycloak_url_escaped="$(sed_escape "$keycloak_url")"
server_auth_block=""

case "$server_auth_type" in
    oauth2)
        token_url="${keycloak_url}/realms/master/protocol/openid-connect/token"
        server_auth_block=$'    auth:\n      oauth2:\n        token_url: '"$(yaml_quote "$token_url")"$'\n        grant_type: password\n        client_id: admin-cli\n        username: '"$(yaml_quote "$KEYCLOAK_ADMIN_USER")"$'\n        password: '"$(yaml_quote "$KEYCLOAK_ADMIN_PASS")"
        ;;
    basic)
        server_auth_block=$'    auth:\n      basic_auth:\n        username: '"$(yaml_quote "$KEYCLOAK_ADMIN_USER")"$'\n        password: '"$(yaml_quote "$KEYCLOAK_ADMIN_PASS")"
        ;;
    bearer|bearer-token|bearer_token)
        if [[ -z "${DECLAREST_SERVER_BEARER_TOKEN:-}" ]]; then
            die "Server bearer auth requires DECLAREST_SERVER_BEARER_TOKEN"
        fi
        server_auth_block=$'    auth:\n      bearer_token:\n        token: '"$(yaml_quote "$DECLAREST_SERVER_BEARER_TOKEN")"
        ;;
esac

secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-file}"
secret_store_type="${secret_store_type,,}"
secret_store_block=""

case "$secret_store_type" in
    none)
        secret_store_block=""
        ;;
    file)
        if [[ -z "${DECLAREST_SECRETS_FILE:-}" || -z "${DECLAREST_SECRETS_PASSPHRASE:-}" ]]; then
            die "File secret store requires DECLAREST_SECRETS_FILE and DECLAREST_SECRETS_PASSPHRASE"
        fi
        secret_store_block=$'secret_store:\n  file:\n    path: '"$(yaml_quote "$DECLAREST_SECRETS_FILE")"$'\n    passphrase: '"$(yaml_quote "$DECLAREST_SECRETS_PASSPHRASE")"
        ;;
    vault)
        vault_addr="${DECLAREST_VAULT_ADDR:-}"
        if [[ -z "$vault_addr" ]]; then
            die "Vault secret store requires DECLAREST_VAULT_ADDR"
        fi
        vault_mount="${DECLAREST_VAULT_MOUNT:-secret}"
        vault_path_prefix="${DECLAREST_VAULT_PATH_PREFIX:-declarest}"
        vault_kv_version="${DECLAREST_VAULT_KV_VERSION:-2}"
        if [[ "$vault_kv_version" != "1" && "$vault_kv_version" != "2" ]]; then
            die "Vault KV version must be 1 or 2 (got ${vault_kv_version})"
        fi

        auth_block=""
        vault_auth_type="${DECLAREST_VAULT_AUTH_TYPE:-token}"
        vault_auth_type="${vault_auth_type,,}"
        case "$vault_auth_type" in
            token)
                if [[ -z "${DECLAREST_VAULT_TOKEN:-}" ]]; then
                    die "Vault token auth requires DECLAREST_VAULT_TOKEN"
                fi
                auth_block=$'    auth:\n      token: '"$(yaml_quote "$DECLAREST_VAULT_TOKEN")"
                ;;
            password)
                if [[ -z "${DECLAREST_VAULT_USERNAME:-}" || -z "${DECLAREST_VAULT_PASSWORD:-}" ]]; then
                    die "Vault password auth requires DECLAREST_VAULT_USERNAME and DECLAREST_VAULT_PASSWORD"
                fi
                auth_block=$'    auth:\n      password:\n        username: '"$(yaml_quote "$DECLAREST_VAULT_USERNAME")"$'\n        password: '"$(yaml_quote "$DECLAREST_VAULT_PASSWORD")"
                if [[ -n "${DECLAREST_VAULT_USERPASS_MOUNT:-}" ]]; then
                    auth_block+=$'\n        mount: '"$(yaml_quote "$DECLAREST_VAULT_USERPASS_MOUNT")"
                fi
                ;;
            approle)
                if [[ -z "${DECLAREST_VAULT_ROLE_ID:-}" || -z "${DECLAREST_VAULT_SECRET_ID:-}" ]]; then
                    die "Vault AppRole auth requires DECLAREST_VAULT_ROLE_ID and DECLAREST_VAULT_SECRET_ID"
                fi
                auth_block=$'    auth:\n      approle:\n        role_id: '"$(yaml_quote "$DECLAREST_VAULT_ROLE_ID")"$'\n        secret_id: '"$(yaml_quote "$DECLAREST_VAULT_SECRET_ID")"
                if [[ -n "${DECLAREST_VAULT_APPROLE_MOUNT:-}" ]]; then
                    auth_block+=$'\n        mount: '"$(yaml_quote "$DECLAREST_VAULT_APPROLE_MOUNT")"
                fi
                ;;
            *)
                die "Unsupported vault auth type: ${vault_auth_type} (expected token, password, or approle)"
                ;;
        esac

        tls_block=""
        tls_has_values=0
        if [[ -n "${DECLAREST_VAULT_CA_CERT_FILE:-}" || -n "${DECLAREST_VAULT_CLIENT_CERT_FILE:-}" || -n "${DECLAREST_VAULT_CLIENT_KEY_FILE:-}" || -n "${DECLAREST_VAULT_INSECURE_SKIP_VERIFY:-}" ]]; then
            tls_has_values=1
            if [[ -n "${DECLAREST_VAULT_CLIENT_CERT_FILE:-}" && -z "${DECLAREST_VAULT_CLIENT_KEY_FILE:-}" ]]; then
                die "Vault TLS requires DECLAREST_VAULT_CLIENT_KEY_FILE when providing a client cert"
            fi
            if [[ -n "${DECLAREST_VAULT_CLIENT_KEY_FILE:-}" && -z "${DECLAREST_VAULT_CLIENT_CERT_FILE:-}" ]]; then
                die "Vault TLS requires DECLAREST_VAULT_CLIENT_CERT_FILE when providing a client key"
            fi
            tls_block=$'    tls:\n'
            if [[ -n "${DECLAREST_VAULT_CA_CERT_FILE:-}" ]]; then
                tls_block+=$'      ca_cert_file: '"$(yaml_quote "$DECLAREST_VAULT_CA_CERT_FILE")"$'\n'
            fi
            if [[ -n "${DECLAREST_VAULT_CLIENT_CERT_FILE:-}" ]]; then
                tls_block+=$'      client_cert_file: '"$(yaml_quote "$DECLAREST_VAULT_CLIENT_CERT_FILE")"$'\n'
            fi
            if [[ -n "${DECLAREST_VAULT_CLIENT_KEY_FILE:-}" ]]; then
                tls_block+=$'      client_key_file: '"$(yaml_quote "$DECLAREST_VAULT_CLIENT_KEY_FILE")"$'\n'
            fi
            if [[ -n "${DECLAREST_VAULT_INSECURE_SKIP_VERIFY:-}" ]]; then
                case "${DECLAREST_VAULT_INSECURE_SKIP_VERIFY,,}" in
                    1|true|yes|y)
                        tls_block+=$'      insecure_skip_verify: true\n'
                        ;;
                    0|false|no|n)
                        tls_block+=$'      insecure_skip_verify: false\n'
                        ;;
                    *)
                        die "Unsupported DECLAREST_VAULT_INSECURE_SKIP_VERIFY value: ${DECLAREST_VAULT_INSECURE_SKIP_VERIFY}"
                        ;;
                esac
            fi
            tls_block="${tls_block%$'\n'}"
        fi

        secret_store_block=$'secret_store:\n  vault:\n    address: '"$(yaml_quote "$vault_addr")"$'\n    mount: '"$(yaml_quote "$vault_mount")"$'\n    path_prefix: '"$(yaml_quote "$vault_path_prefix")"$'\n    kv_version: '"${vault_kv_version}"
        if [[ -n "$auth_block" ]]; then
            secret_store_block+=$'\n'"$auth_block"
        fi
        if [[ $tls_has_values -eq 1 ]]; then
            secret_store_block+=$'\n'"$tls_block"
        fi
        ;;
    *)
        die "Unsupported secret store type: ${secret_store_type} (expected none, file, or vault)"
        ;;
esac

sed \
    -e "s#__KEYCLOAK_URL__#${keycloak_url_escaped}#g" \
    "$tpl_config" | while IFS= read -r line; do
        if [[ "$line" == "__REPOSITORY_BLOCK__" ]]; then
            printf "%s\n" "$repo_block"
        elif [[ "$line" == "__SERVER_AUTH_BLOCK__" ]]; then
            printf "%s\n" "$server_auth_block"
        elif [[ "$line" == "__SECRET_STORE_BLOCK__" ]]; then
            printf "%s\n" "$secret_store_block"
        else
            printf "%s\n" "$line"
        fi
    done > "$DECLAREST_CONTEXT_FILE"
log_line "Context file rendered to $DECLAREST_CONTEXT_FILE"
