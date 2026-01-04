#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$KEYCLOAK_DIR/scripts"
LAST_RUN_FILE="${XDG_CACHE_HOME:-$HOME/.cache}/declarest-keycloak/last"

usage() {
    cat <<EOF
Usage: ./tests/keycloak/run.sh [setup|sync|stop|clean|reset|cli] [options] [-- <declarest args>]

Options:
  --sync           Sync the template repository to Keycloak after setup.
  --seed-secrets   Seed sample secrets in the secret store during setup.
  --work-dir PATH  Use an existing work directory for stop/clean/sync/cli.
  --all            With clean, remove all work directories and container artifacts.
  -h, --help       Show this help message.
EOF
}

command="${1:-setup}"
shift || true

SYNC_RESOURCE=0
WORK_DIR_OVERRIDE=""
CLEAN_ALL=0
SEED_SECRETS=0
CLI_ARGS=()

if [[ "$command" == "cli" ]]; then
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --work-dir)
                WORK_DIR_OVERRIDE="${2:-}"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            --)
                shift
                CLI_ARGS+=("$@")
                break
                ;;
            *)
                CLI_ARGS+=("$1")
                shift
                ;;
        esac
    done
else
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --sync|--sync-resource)
                SYNC_RESOURCE=1
                shift
                ;;
            --seed-secrets)
                if [[ "$command" != "setup" ]]; then
                    printf "Unknown option: %s\n" "$1" >&2
                    usage >&2
                    exit 1
                fi
                SEED_SECRETS=1
                shift
                ;;
            --work-dir)
                WORK_DIR_OVERRIDE="${2:-}"
                shift 2
                ;;
            --all)
                if [[ "$command" != "clean" ]]; then
                    printf "Unknown option: %s\n" "$1" >&2
                    usage >&2
                    exit 1
                fi
                CLEAN_ALL=1
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
fi

if [[ -n "$WORK_DIR_OVERRIDE" ]]; then
    export DECLAREST_WORK_DIR="$WORK_DIR_OVERRIDE"
fi

state_file_path() {
    printf "%s/state.env" "${DECLAREST_WORK_DIR%/}"
}

load_state() {
    local state=""
    if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
        state="$(state_file_path)"
    elif [[ -f "$LAST_RUN_FILE" ]]; then
        state="$(cat "$LAST_RUN_FILE")"
    fi
    if [[ -z "$state" || ! -f "$state" ]]; then
        printf "State file not found. Provide --work-dir or set DECLAREST_WORK_DIR.\n" >&2
        exit 1
    fi
    source "$state"
}

write_state() {
    local state_file
    state_file="$(state_file_path)"
    mkdir -p "$DECLAREST_WORK_DIR"
    {
        printf 'export DECLAREST_RUN_ID=%q\n' "${DECLAREST_RUN_ID:-}"
        printf 'export DECLAREST_WORK_BASE_DIR=%q\n' "${DECLAREST_WORK_BASE_DIR:-}"
        printf 'export DECLAREST_WORK_DIR=%q\n' "${DECLAREST_WORK_DIR:-}"
        printf 'export DECLAREST_BIN_DIR=%q\n' "${DECLAREST_BIN_DIR:-}"
        printf 'export DECLAREST_REPO_DIR=%q\n' "${DECLAREST_REPO_DIR:-}"
        printf 'export DECLAREST_REPO_REMOTE_URL=%q\n' "${DECLAREST_REPO_REMOTE_URL:-}"
        printf 'export DECLAREST_REPO_TYPE=%q\n' "${DECLAREST_REPO_TYPE:-}"
        printf 'export DECLAREST_REPO_PROVIDER=%q\n' "${DECLAREST_REPO_PROVIDER:-}"
        printf 'export DECLAREST_REMOTE_REPO_PROVIDER=%q\n' "${DECLAREST_REMOTE_REPO_PROVIDER:-}"
        printf 'export DECLAREST_GITLAB_ENABLE=%q\n' "${DECLAREST_GITLAB_ENABLE:-}"
        printf 'export DECLAREST_GITEA_ENABLE=%q\n' "${DECLAREST_GITEA_ENABLE:-}"
        printf 'export DECLAREST_REPO_AUTH_TYPE=%q\n' "${DECLAREST_REPO_AUTH_TYPE:-}"
        printf 'export DECLAREST_REPO_AUTH=%q\n' "${DECLAREST_REPO_AUTH:-}"
        printf 'export DECLAREST_REPO_SSH_USER=%q\n' "${DECLAREST_REPO_SSH_USER:-}"
        printf 'export DECLAREST_REPO_SSH_KEY_FILE=%q\n' "${DECLAREST_REPO_SSH_KEY_FILE:-}"
        printf 'export DECLAREST_REPO_SSH_PASSPHRASE=%q\n' "${DECLAREST_REPO_SSH_PASSPHRASE:-}"
        printf 'export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE=%q\n' "${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}"
        printf 'export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY=%q\n' "${DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY:-}"
        printf 'export DECLAREST_SERVER_AUTH_TYPE=%q\n' "${DECLAREST_SERVER_AUTH_TYPE:-}"
        printf 'export DECLAREST_CONTEXT_FILE=%q\n' "${DECLAREST_CONTEXT_FILE:-}"
        printf 'export DECLAREST_LOG_DIR=%q\n' "${DECLAREST_LOG_DIR:-}"
        printf 'export DECLAREST_COMPOSE_DIR=%q\n' "${DECLAREST_COMPOSE_DIR:-}"
        printf 'export DECLAREST_HOME_DIR=%q\n' "${DECLAREST_HOME_DIR:-}"
        printf 'export DECLAREST_SECRETS_FILE=%q\n' "${DECLAREST_SECRETS_FILE:-}"
        printf 'export DECLAREST_SECRET_STORE_TYPE=%q\n' "${DECLAREST_SECRET_STORE_TYPE:-}"
        printf 'export DECLAREST_TEMPLATE_REPO_DIR=%q\n' "${DECLAREST_TEMPLATE_REPO_DIR:-}"
        printf 'export DECLAREST_KEEP_WORK=%q\n' "${DECLAREST_KEEP_WORK:-}"
        printf 'export DECLAREST_SECRETS_PASSPHRASE=%q\n' "${DECLAREST_SECRETS_PASSPHRASE:-}"
        printf 'export DECLAREST_TEST_CLIENT_SECRET=%q\n' "${DECLAREST_TEST_CLIENT_SECRET:-}"
        printf 'export DECLAREST_TEST_LDAP_BIND_CREDENTIAL=%q\n' "${DECLAREST_TEST_LDAP_BIND_CREDENTIAL:-}"
        printf 'export DECLAREST_VAULT_ADDR=%q\n' "${DECLAREST_VAULT_ADDR:-}"
        printf 'export DECLAREST_VAULT_AUTH_TYPE=%q\n' "${DECLAREST_VAULT_AUTH_TYPE:-}"
        printf 'export DECLAREST_VAULT_TOKEN=%q\n' "${DECLAREST_VAULT_TOKEN:-}"
        printf 'export DECLAREST_VAULT_UNSEAL_KEY=%q\n' "${DECLAREST_VAULT_UNSEAL_KEY:-}"
        printf 'export DECLAREST_VAULT_USERNAME=%q\n' "${DECLAREST_VAULT_USERNAME:-}"
        printf 'export DECLAREST_VAULT_PASSWORD=%q\n' "${DECLAREST_VAULT_PASSWORD:-}"
        printf 'export DECLAREST_VAULT_ROLE_ID=%q\n' "${DECLAREST_VAULT_ROLE_ID:-}"
        printf 'export DECLAREST_VAULT_SECRET_ID=%q\n' "${DECLAREST_VAULT_SECRET_ID:-}"
        printf 'export DECLAREST_VAULT_MOUNT=%q\n' "${DECLAREST_VAULT_MOUNT:-}"
        printf 'export DECLAREST_VAULT_PATH_PREFIX=%q\n' "${DECLAREST_VAULT_PATH_PREFIX:-}"
        printf 'export DECLAREST_VAULT_KV_VERSION=%q\n' "${DECLAREST_VAULT_KV_VERSION:-}"
        printf 'export CONTAINER_RUNTIME=%q\n' "${CONTAINER_RUNTIME:-}"
        printf 'export COMPOSE_PROJECT_NAME=%q\n' "${COMPOSE_PROJECT_NAME:-}"
        printf 'export KEYCLOAK_CONTAINER_NAME=%q\n' "${KEYCLOAK_CONTAINER_NAME:-}"
        printf 'export KEYCLOAK_IMAGE=%q\n' "${KEYCLOAK_IMAGE:-}"
        printf 'export KEYCLOAK_ADMIN_USER=%q\n' "${KEYCLOAK_ADMIN_USER:-}"
        printf 'export KEYCLOAK_ADMIN_PASS=%q\n' "${KEYCLOAK_ADMIN_PASS:-}"
        printf 'export KEYCLOAK_HTTP_PORT=%q\n' "${KEYCLOAK_HTTP_PORT:-}"
        printf 'export VAULT_HTTP_PORT=%q\n' "${VAULT_HTTP_PORT:-}"
        printf 'export VAULT_CONTAINER_NAME=%q\n' "${VAULT_CONTAINER_NAME:-}"
        printf 'export VAULT_IMAGE=%q\n' "${VAULT_IMAGE:-}"
    } >"$state_file"
    mkdir -p "$(dirname "$LAST_RUN_FILE")"
    printf "%s\n" "$state_file" >"$LAST_RUN_FILE"
}

is_tty() {
    [[ -t 1 && "${NO_SPINNER:-0}" != "1" ]]
}

run_step() {
    local title="$1"
    shift
    log_line "STEP START ${title}"
    if is_tty; then
        printf "\r[RUN ] %s..." "$title"
    else
        printf "[RUN ] %s...\n" "$title"
    fi
    if "$@" >>"$RUN_LOG" 2>&1; then
        if is_tty; then
            printf "\r\033[K"
        fi
        printf "[DONE] %s\n" "$title"
        log_line "STEP DONE ${title}"
        return 0
    fi
    local status=$?
    if is_tty; then
        printf "\r\033[K"
    fi
    printf "[FAIL] %s\n" "$title"
    log_line "STEP FAILED ${title} (exit ${status})"
    printf "See detailed log: %s\n" "$RUN_LOG"
    exit $status
}

declarest_cli() {
    HOME="$DECLAREST_HOME_DIR" "$DECLAREST_BIN_DIR/declarest" "$@"
}

run_cli() {
    local label="$1"
    shift
    run_logged "$label" declarest_cli "$@"
}

seed_secrets() {
    run_cli "secret add client secret" secret add --path "/admin/realms/publico/clients/testB" --key "secret" --value "$DECLAREST_TEST_CLIENT_SECRET"
    run_cli "secret add ldap bind credential" secret add --path "/admin/realms/publico/user-registry/ldap-test" --key "config.bindCredential[0]" --value "$DECLAREST_TEST_LDAP_BIND_CREDENTIAL"
}

init_secrets_manager() {
    run_cli "secret init" secret init
}

setup_run_log() {
    mkdir -p "$DECLAREST_LOG_DIR"
    export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-setup_$(date -Iseconds | tr ':' '-').log}"
    touch "$RUN_LOG"
}

cleanup_on_error() {
    local status=$?
    if [[ $status -eq 0 ]]; then
        return 0
    fi
    log_line "Setup failed; stopping Keycloak"
    "$SCRIPTS_DIR/stack/stop.sh" >>"$RUN_LOG" 2>&1 || true
    "$SCRIPTS_DIR/workspace/cleanup.sh" >>"$RUN_LOG" 2>&1 || true
    printf "\nSetup failed (exit %s). See log: %s\n" "$status" "$RUN_LOG"
}

case "$command" in
    setup)
        source "$SCRIPTS_DIR/lib/env.sh"
        source "$SCRIPTS_DIR/lib/logging.sh"
        source "$SCRIPTS_DIR/lib/shell.sh"
        setup_run_log
        trap 'cleanup_on_error' EXIT INT TERM

        echo "Starting Keycloak manual setup"
        echo "Detailed log: $RUN_LOG"
        log_line "Keycloak manual setup started"
        log_line "Container runtime: $CONTAINER_RUNTIME"
        secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-file}"
        secret_store_type="${secret_store_type,,}"

        run_step "Preparing workspace" "$SCRIPTS_DIR/workspace/prepare.sh"
        if [[ "$secret_store_type" == "none" ]]; then
            template_dest="$DECLAREST_WORK_DIR/templates/repo-no-secrets"
            run_step "Preparing template (no secrets)" "$SCRIPTS_DIR/repo/strip-secrets.sh" "$DECLAREST_TEST_DIR/templates/repo" "$template_dest"
            export DECLAREST_TEMPLATE_REPO_DIR="$template_dest"
        fi
        write_state
        run_step "Building declarest CLI" "$SCRIPTS_DIR/declarest/build.sh"
        run_step "Starting stack" "$SCRIPTS_DIR/stack/start.sh"
        source "$SCRIPTS_DIR/lib/env.sh"
        write_state
        repo_type="${DECLAREST_REPO_TYPE:-}"
        repo_type="${repo_type,,}"
        if [[ "$repo_type" == "git-remote" ]]; then
            provider="${DECLAREST_REMOTE_REPO_PROVIDER:-${DECLAREST_REPO_PROVIDER:-}}"
            provider="${provider,,}"
            provider_env=""
            if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
                case "$provider" in
                    gitlab)
                        run_step "Preparing GitLab" "$SCRIPTS_DIR/providers/gitlab/setup.sh"
                        provider_env="$DECLAREST_WORK_DIR/gitlab.env"
                        ;;
                    gitea)
                        run_step "Preparing Gitea" "$SCRIPTS_DIR/providers/gitea/setup.sh"
                        provider_env="$DECLAREST_WORK_DIR/gitea.env"
                        ;;
                esac
                if [[ -n "$provider_env" ]]; then
                    if [[ ! -f "$provider_env" ]]; then
                        die "Provider env file missing: $provider_env"
                    fi
                    # shellcheck source=/dev/null
                    source "$provider_env"
                fi
            fi

            if [[ "$provider" == "gitlab" || "$provider" == "gitea" ]]; then
                repo_auth_type="${DECLAREST_REPO_AUTH_TYPE:-pat}"
                repo_auth_type="${repo_auth_type,,}"
                export DECLAREST_REPO_AUTH_TYPE="$repo_auth_type"
                export DECLAREST_REPO_PROVIDER="$provider"
                export DECLAREST_REMOTE_REPO_PROVIDER="$provider"

                case "$provider" in
                    gitlab)
                        provider_pat_url="$DECLAREST_GITLAB_PAT_URL"
                        provider_pat="$DECLAREST_GITLAB_PAT"
                        provider_basic_url="$DECLAREST_GITLAB_BASIC_URL"
                        provider_basic_user="$DECLAREST_GITLAB_USER"
                        provider_basic_pass="$DECLAREST_GITLAB_PASSWORD"
                        provider_ssh_url="$DECLAREST_GITLAB_SSH_URL"
                        provider_ssh_key="$DECLAREST_GITLAB_SSH_KEY_FILE"
                        provider_known_hosts="$DECLAREST_GITLAB_KNOWN_HOSTS_FILE"
                        ;;
                    gitea)
                        provider_pat_url="$DECLAREST_GITEA_PAT_URL"
                        provider_pat="$DECLAREST_GITEA_PAT"
                        provider_basic_url="$DECLAREST_GITEA_BASIC_URL"
                        provider_basic_user="$DECLAREST_GITEA_USER"
                        provider_basic_pass="$DECLAREST_GITEA_PASSWORD"
                        provider_ssh_url="$DECLAREST_GITEA_SSH_URL"
                        provider_ssh_key="$DECLAREST_GITEA_SSH_KEY_FILE"
                        provider_known_hosts="$DECLAREST_GITEA_KNOWN_HOSTS_FILE"
                        ;;
                esac

                case "$repo_auth_type" in
                    pat)
                        if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
                            export DECLAREST_REPO_REMOTE_URL="$provider_pat_url"
                        fi
                        if [[ -z "${DECLAREST_REPO_AUTH:-}" ]]; then
                            export DECLAREST_REPO_AUTH="$provider_pat"
                        fi
                        ;;
                    basic)
                        if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
                            export DECLAREST_REPO_REMOTE_URL="$provider_basic_url"
                        fi
                        if [[ -z "${DECLAREST_REPO_AUTH:-}" ]]; then
                            export DECLAREST_REPO_AUTH="${provider_basic_user}:${provider_basic_pass}"
                        fi
                        ;;
                    ssh)
                        if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
                            export DECLAREST_REPO_REMOTE_URL="$provider_ssh_url"
                        fi
                        if [[ -z "${DECLAREST_REPO_SSH_USER:-}" ]]; then
                            export DECLAREST_REPO_SSH_USER="git"
                        fi
                        if [[ -z "${DECLAREST_REPO_SSH_KEY_FILE:-}" ]]; then
                            export DECLAREST_REPO_SSH_KEY_FILE="$provider_ssh_key"
                        fi
                        if [[ -z "${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}" ]]; then
                            export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$provider_known_hosts"
                        fi
                        ;;
                    *)
                        die "Unsupported repo auth type: ${repo_auth_type}"
                        ;;
                esac
            fi
        fi
        write_state
        run_step "Preparing template repo" "$SCRIPTS_DIR/repo/prepare.sh"
        run_step "Configuring declarest context" "$SCRIPTS_DIR/context/render.sh"
        run_step "Registering declarest context" "$SCRIPTS_DIR/context/register.sh"
        if [[ "$secret_store_type" == "none" && "$repo_type" == "git-remote" ]]; then
            run_step "Sanitizing repository (no secrets)" "$SCRIPTS_DIR/repo/strip-secrets.sh" "$DECLAREST_REPO_DIR"
        fi
        if [[ "$secret_store_type" != "none" ]]; then
            run_step "Initializing secret store" init_secrets_manager
            if [[ $SEED_SECRETS -eq 1 ]]; then
                run_step "Seeding secrets" seed_secrets
            fi
        elif [[ $SEED_SECRETS -eq 1 ]]; then
            printf "Secret store disabled; skipping secret seeding.\n"
        fi

        if [[ $SYNC_RESOURCE -eq 1 ]]; then
            run_step "Syncing repository resources" "$SCRIPTS_DIR/declarest/sync.sh"
        fi

        cat <<EOF

Manual setup complete. Keycloak is running at http://localhost:${KEYCLOAK_HTTP_PORT}

Use the declarest CLI with the prepared context:
  ./tests/keycloak/run.sh cli --work-dir "${DECLAREST_WORK_DIR}" resource list --repo
  ./tests/keycloak/run.sh cli --work-dir "${DECLAREST_WORK_DIR}"   # shows declarest help
  ./tests/keycloak/run.sh cli resource list --repo                 # reuses the last run

Logs: ${RUN_LOG}
Repo: ${DECLAREST_REPO_DIR}
Secret store: ${DECLAREST_SECRET_STORE_TYPE}
$(if [[ "${DECLAREST_SECRET_STORE_TYPE:-}" == "vault" ]]; then printf 'Vault address: %s\nVault auth: %s\n' "$DECLAREST_VAULT_ADDR" "$DECLAREST_VAULT_AUTH_TYPE"; elif [[ "${DECLAREST_SECRET_STORE_TYPE:-}" == "none" ]]; then printf 'Secrets file: (disabled)\n'; else printf 'Secrets file: %s\n' "$DECLAREST_SECRETS_FILE"; fi)

Stop Keycloak:
  ./tests/keycloak/run.sh stop --work-dir "${DECLAREST_WORK_DIR}"
  ./tests/keycloak/run.sh stop

Clean the work directory:
  ./tests/keycloak/run.sh clean --work-dir "${DECLAREST_WORK_DIR}"
  ./tests/keycloak/run.sh clean
EOF
        ;;
    sync)
        load_state
        source "$SCRIPTS_DIR/lib/env.sh"
        source "$SCRIPTS_DIR/lib/logging.sh"
        mkdir -p "$DECLAREST_LOG_DIR"
        export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-sync_$(date -Iseconds | tr ':' '-').log}"
        run_step "Syncing repository resources" "$SCRIPTS_DIR/declarest/sync.sh"
        ;;
    stop)
        load_state
        source "$SCRIPTS_DIR/lib/env.sh"
        source "$SCRIPTS_DIR/lib/logging.sh"
        mkdir -p "$DECLAREST_LOG_DIR"
        export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-stop_$(date -Iseconds | tr ':' '-').log}"
        run_step "Stopping Keycloak" "$SCRIPTS_DIR/stack/stop.sh"
        ;;
    clean)
        if [[ $CLEAN_ALL -eq 1 ]]; then
            log_dir="${XDG_CACHE_HOME:-$HOME/.cache}/declarest-keycloak"
            mkdir -p "$log_dir"
            export RUN_LOG="${RUN_LOG:-$log_dir/run-clean-all_$(date -Iseconds | tr ':' '-').log}"
            touch "$RUN_LOG"
            source "$SCRIPTS_DIR/lib/logging.sh"
            run_step "Cleaning all workspaces" "$SCRIPTS_DIR/workspace/cleanup.sh" --all
            rm -f "$LAST_RUN_FILE" 2>/dev/null || true
            exit 0
        fi
        load_state
        source "$SCRIPTS_DIR/lib/env.sh"
        source "$SCRIPTS_DIR/lib/logging.sh"
        mkdir -p "$DECLAREST_LOG_DIR"
        export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-clean_$(date -Iseconds | tr ':' '-').log}"
        run_step "Stopping Keycloak" "$SCRIPTS_DIR/stack/stop.sh"
        run_step "Cleaning workspace" "$SCRIPTS_DIR/workspace/cleanup.sh"
        ;;
    reset)
        if [[ -n "${DECLAREST_WORK_DIR:-}" || -f "$LAST_RUN_FILE" ]]; then
            load_state
            source "$SCRIPTS_DIR/lib/env.sh"
            source "$SCRIPTS_DIR/lib/logging.sh"
            mkdir -p "$DECLAREST_LOG_DIR"
            export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-reset_$(date -Iseconds | tr ':' '-').log}"
            run_step "Stopping Keycloak" "$SCRIPTS_DIR/stack/stop.sh"
            run_step "Cleaning workspace" "$SCRIPTS_DIR/workspace/cleanup.sh"
        fi
        exec "$KEYCLOAK_DIR/run.sh" setup ${SYNC_RESOURCE:+--sync}
        ;;
    cli)
        load_state
        if [[ ! -x "$DECLAREST_BIN_DIR/declarest" ]]; then
            printf "declarest binary not found at %s. Run setup first.\n" "$DECLAREST_BIN_DIR/declarest" >&2
            exit 1
        fi
        if [[ ${#CLI_ARGS[@]} -eq 0 ]]; then
            declarest_cli
            exit $?
        fi
        declarest_cli "${CLI_ARGS[@]}"
        exit $?
        ;;
    *)
        printf "Unknown command: %s\n" "$command" >&2
        usage >&2
        exit 1
        ;;
esac
