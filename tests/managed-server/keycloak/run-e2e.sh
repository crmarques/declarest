#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$KEYCLOAK_DIR/scripts"

usage() {
    cat <<EOF
Usage: ./tests/managed-server/keycloak/run-e2e.sh [--managed-server NAME] [--repo-provider NAME] [--secret-provider NAME]

Options:
  --managed-server NAME   Managed server to target (default: keycloak)
                            Available: keycloak
  --repo-provider NAME    Repository provider (default: git)
                            Available: file, git, gitlab, gitea and github
  --secret-provider NAME  Secret store provider (defaults: file)
                            Available: none, file and vault
  -h, --help              Show this help message.
EOF
}

# shellcheck source=scripts/lib/args.sh
source "$SCRIPTS_DIR/lib/args.sh"
# shellcheck source=scripts/lib/github-auth.sh
source "$SCRIPTS_DIR/lib/github-auth.sh"

managed_server="keycloak"
repo_provider="git"
secret_provider="file"

parse_common_flags "$@"
resolve_repo_provider
apply_repo_provider_env

# shellcheck source=scripts/lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=scripts/lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"

REPO_SCRIPTS_DIR="$DECLAREST_TESTS_ROOT/repo-provider/common"
PROVIDER_SCRIPTS_DIR="$DECLAREST_TESTS_ROOT/repo-provider"

resolve_container_runtime() {
    if [[ "${CONTAINER_RUNTIME:-}" == "podman" ]]; then
        if ! podman info >/dev/null 2>&1; then
            if command -v docker >/dev/null 2>&1; then
                log_line "Podman unavailable; falling back to docker"
                export CONTAINER_RUNTIME="docker"
            else
                die "Podman is unavailable and docker is not installed"
            fi
        fi
    fi
}

resolve_container_runtime

if [[ "$repo_type" != "git-remote" ]]; then
    clear_remote_repo_env
fi

if [[ -n "${COMPOSE_PROJECT_NAME:-}" && ( -z "${KEYCLOAK_CONTAINER_NAME:-}" || "$KEYCLOAK_CONTAINER_NAME" == "keycloak-declarest-test" ) ]]; then
    export KEYCLOAK_CONTAINER_NAME="${COMPOSE_PROJECT_NAME}_keycloak-declarest-test_1"
fi

mkdir -p "$DECLAREST_LOG_DIR"
export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-e2e_$(date -Iseconds | tr ':' '-').log}"
touch "$RUN_LOG"

is_tty() {
    [[ -t 1 && "${NO_SPINNER:-0}" != "1" ]]
}

print_step_start() {
    local label="$1"
    local title="$2"

    if is_tty; then
        printf "\r[RUN ] %s | %s..." "$label" "$title"
    else
        printf "[RUN ] %s | %s...\n" "$label" "$title"
    fi
}

print_step_result() {
    local state="$1"
    local label="$2"
    local title="$3"
    local duration="$4"

    if is_tty; then
        printf "\r\033[K"
    fi
    printf "[%s] %s | %s" "$state" "$label" "$title"
    if [[ -n "$duration" ]]; then
        printf " %ss" "$duration"
    fi
    printf "\n"
}

cleanup() {
    local status="$1"

    if [[ "${KEEP_KEYCLOAK:-0}" != "1" ]]; then
        log_line "Stopping Keycloak stack"
        "$SCRIPTS_DIR/stack/stop.sh" >>"$RUN_LOG" 2>&1 || true
    else
        log_line "KEEP_KEYCLOAK=1; skipping Keycloak shutdown"
    fi

    log_line "Cleaning up work directory"
    "$SCRIPTS_DIR/workspace/cleanup.sh" >>"$RUN_LOG" 2>&1 || true

    if [[ $status -ne 0 ]]; then
        printf "\nRun failed (exit %s). See log: %s\n" "$status" "$RUN_LOG"
    fi
}

trap 'cleanup "$?"' EXIT INT TERM

resolve_known_hosts() {
    local candidate="$1"
    if [[ -n "$candidate" ]]; then
        printf "%s" "$candidate"
        return 0
    fi
    local default_known_hosts="$HOME/.ssh/known_hosts"
    if [[ -f "$default_known_hosts" ]]; then
        printf "%s" "$default_known_hosts"
        return 0
    fi
    printf "%s" ""
}

configure_repo_auth() {
    local auth="$1"
    local resolved_hosts

    export DECLAREST_REPO_AUTH_TYPE=""
    export DECLAREST_REPO_AUTH=""
    export DECLAREST_REPO_SSH_USER=""
    export DECLAREST_REPO_SSH_KEY_FILE=""
    export DECLAREST_REPO_SSH_PASSPHRASE=""
    export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE=""
    export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY=""

    if [[ "$repo_type" != "git-remote" ]]; then
        export DECLAREST_REPO_REMOTE_URL=""
        return 0
    fi

    export DECLAREST_REPO_PROVIDER="$repo_provider"
    auth="${auth,,}"

    case "$auth" in
        pat)
            export DECLAREST_REPO_AUTH_TYPE="pat"
            case "$repo_provider" in
                gitlab)
                    export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITLAB_PAT_URL"
                    export DECLAREST_REPO_AUTH="$DECLAREST_GITLAB_PAT"
                    ;;
                gitea)
                    export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITEA_PAT_URL"
                    export DECLAREST_REPO_AUTH="$DECLAREST_GITEA_PAT"
                    ;;
                github)
                    export DECLAREST_REPO_REMOTE_URL="$github_https_url"
                    export DECLAREST_REPO_AUTH="$github_pat"
                    ;;
            esac
            ;;
        basic)
            export DECLAREST_REPO_AUTH_TYPE="basic"
            case "$repo_provider" in
                gitlab)
                    export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITLAB_BASIC_URL"
                    export DECLAREST_REPO_AUTH="${DECLAREST_GITLAB_USER}:${DECLAREST_GITLAB_PASSWORD}"
                    ;;
                gitea)
                    export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITEA_BASIC_URL"
                    export DECLAREST_REPO_AUTH="${DECLAREST_GITEA_USER}:${DECLAREST_GITEA_PASSWORD}"
                    ;;
                github)
                    die "GitHub does not support basic auth in this harness"
                    ;;
            esac
            ;;
        ssh)
            export DECLAREST_REPO_AUTH_TYPE="ssh"
            case "$repo_provider" in
                gitlab)
                    export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITLAB_SSH_URL"
                    export DECLAREST_REPO_SSH_USER="git"
                    export DECLAREST_REPO_SSH_KEY_FILE="$DECLAREST_GITLAB_SSH_KEY_FILE"
                    export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$DECLAREST_GITLAB_KNOWN_HOSTS_FILE"
                    ;;
                gitea)
                    export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITEA_SSH_URL"
                    export DECLAREST_REPO_SSH_USER="git"
                    export DECLAREST_REPO_SSH_KEY_FILE="$DECLAREST_GITEA_SSH_KEY_FILE"
                    export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$DECLAREST_GITEA_KNOWN_HOSTS_FILE"
                    ;;
                github)
                    export DECLAREST_REPO_REMOTE_URL="$github_ssh_url"
                    export DECLAREST_REPO_SSH_KEY_FILE="$github_ssh_key_file"
                    if [[ -n "$github_ssh_insecure" ]]; then
                        export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY="$github_ssh_insecure"
                    fi
                    resolved_hosts="$(resolve_known_hosts "$github_ssh_known_hosts")"
                    if [[ -n "$resolved_hosts" ]]; then
                        export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$resolved_hosts"
                    fi
                    if [[ -z "$DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE" && -z "$DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY" ]]; then
                        die "GitHub SSH host verification requires DECLAREST_GITHUB_SSH_KNOWN_HOSTS_FILE or DECLAREST_GITHUB_SSH_INSECURE_IGNORE_HOST_KEY=1"
                    fi
                    ;;
            esac
            ;;
        *)
            die "Unsupported repo auth type: ${auth}"
            ;;
    esac

    if [[ -z "${DECLAREST_REPO_REMOTE_URL:-}" ]]; then
        die "Remote repository URL is not configured"
    fi
    if [[ "$DECLAREST_REPO_AUTH_TYPE" == "pat" || "$DECLAREST_REPO_AUTH_TYPE" == "basic" ]]; then
        if [[ -z "${DECLAREST_REPO_AUTH:-}" ]]; then
            die "Missing repository credentials for auth type ${DECLAREST_REPO_AUTH_TYPE}"
        fi
    fi
    if [[ "$DECLAREST_REPO_AUTH_TYPE" == "ssh" ]]; then
        if [[ -z "${DECLAREST_REPO_SSH_KEY_FILE:-}" || ! -f "$DECLAREST_REPO_SSH_KEY_FILE" ]]; then
            die "SSH key file not found for repo auth"
        fi
    fi
}

configure_secret_auth() {
    local auth="$1"
    if [[ "$secret_provider" == "vault" ]]; then
        export DECLAREST_VAULT_AUTH_TYPE="$auth"
    else
        export DECLAREST_VAULT_AUTH_TYPE=""
    fi
}

resolve_keycloak_port() {
    local port_file port
    if [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
        port_file="${DECLAREST_WORK_DIR%/}/keycloak-port"
    fi
    if [[ -n "${port_file:-}" && -f "$port_file" ]]; then
        port="$(tr -d ' \t\r\n' < "$port_file")"
        if [[ -n "$port" ]]; then
            printf "%s" "$port"
            return 0
        fi
    fi
    printf "%s" "${KEYCLOAK_HTTP_PORT:-18080}"
}

fetch_keycloak_bearer_token() {
    require_cmd curl
    require_cmd jq

    local port token_url response status token
    port="$(resolve_keycloak_port)"
    token_url="http://localhost:${port}/realms/master/protocol/openid-connect/token"

    set +e
    response="$(curl -sS --fail \
        -H "Content-Type: application/x-www-form-urlencoded" \
        --data-urlencode "grant_type=password" \
        --data-urlencode "client_id=admin-cli" \
        --data-urlencode "username=${KEYCLOAK_ADMIN_USER}" \
        --data-urlencode "password=${KEYCLOAK_ADMIN_PASS}" \
        "$token_url" 2>&1)"
    status=$?
    set -e

    if [[ $status -ne 0 ]]; then
        log_block "Keycloak token request failed" "$response"
        die "Failed to fetch Keycloak access token"
    fi

    token="$(jq -r '.access_token // empty' <<<"$response")"
    if [[ -z "$token" ]]; then
        log_block "Keycloak token response" "$response"
        die "Keycloak access token missing in response"
    fi

    printf "%s" "$token"
}

configure_server_auth() {
    local auth="$1"
    export DECLAREST_SERVER_AUTH_TYPE="$auth"
    export DECLAREST_SERVER_BEARER_TOKEN=""
    case "${auth,,}" in
        bearer|bearer-token|bearer_token)
            export DECLAREST_SERVER_BEARER_TOKEN="$(fetch_keycloak_bearer_token)"
            ;;
    esac
}

set_context() {
    local suffix="$1"
    if [[ -n "$suffix" ]]; then
        export DECLAREST_CONTEXT_NAME="keycloak-e2e-${suffix}"
        export DECLAREST_CONTEXT_FILE="$DECLAREST_WORK_DIR/context-${suffix}.yaml"
    else
        export DECLAREST_CONTEXT_NAME="keycloak-e2e"
        export DECLAREST_CONTEXT_FILE="$DECLAREST_WORK_DIR/context.yaml"
    fi
}

TOTAL_STEPS=4
if [[ "$secret_provider" == "none" ]]; then
    TOTAL_STEPS=$((TOTAL_STEPS + 1))
fi
if [[ "$secret_provider" == "none" && "$repo_type" == "git-remote" ]]; then
    TOTAL_STEPS=$((TOTAL_STEPS + 1))
fi

server_auth_primary="oauth2"
server_auth_secondary=(bearer)
secret_auth_primary=""
secret_auth_secondary=()
case "$secret_provider" in
    vault)
        secret_auth_primary="token"
        secret_auth_secondary=(password approle)
        ;;
    file)
        secret_auth_primary="file"
        ;;
    none)
        secret_auth_primary="none"
        ;;
esac

repo_auth_primary=""
repo_auth_secondary=()
if [[ "$repo_type" == "git-remote" ]]; then
    repo_auth_primary="pat"
    if [[ "$repo_provider" == "github" ]]; then
        repo_auth_secondary=(ssh)
    else
        repo_auth_secondary=(basic ssh)
    fi
fi

heavy_steps=4
heavy_steps=$((heavy_steps + 1))
if [[ "$repo_type" == "git-remote" ]]; then
    heavy_steps=$((heavy_steps + 1))
fi
if [[ "$repo_type" == "git-local" ]]; then
    heavy_steps=$((heavy_steps + 1))
fi
TOTAL_STEPS=$((TOTAL_STEPS + heavy_steps))
TOTAL_STEPS=$((TOTAL_STEPS + ${#server_auth_secondary[@]}))
TOTAL_STEPS=$((TOTAL_STEPS + ${#secret_auth_secondary[@]}))
TOTAL_STEPS=$((TOTAL_STEPS + ${#repo_auth_secondary[@]}))

STEP_NUM_WIDTH=${#TOTAL_STEPS}

format_step_label() {
    local step="$1"
    printf "%*d/%s" "$STEP_NUM_WIDTH" "$step" "$TOTAL_STEPS"
}

run_step() {
    local title="$1"
    shift
    local cmd=("$@")

    current_step=$((current_step + 1))
    local label
    label="$(format_step_label "$current_step")"
    log_line "STEP START (${label}) ${title}"
    print_step_start "$label" "$title"
    local started_at
    started_at=$(date +%s)

    set +e
    (
        set -euo pipefail
        "${cmd[@]}"
    ) >>"$RUN_LOG" 2>&1
    local status=$?
    set -e

    local elapsed=$(( $(date +%s) - started_at ))
    if [[ $status -eq 0 ]]; then
        print_step_result "DONE" "$label" "$title" "$elapsed"
        log_line "STEP DONE (${label}) ${title} (${elapsed}s)"
        return 0
    fi

    print_step_result "FAIL" "$label" "$title" "$elapsed"
    log_line "STEP FAILED (${label}) ${title} (exit ${status}, ${elapsed}s)"
    printf "See detailed log: %s\n" "$RUN_LOG"
    exit $status
}

current_step=0

ensure_github_pat_ssh_credentials

echo "Starting Keycloak E2E run"
echo "Detailed log: $RUN_LOG"
log_line "Keycloak E2E run started"
log_line "Container runtime: $CONTAINER_RUNTIME"

run_step "Preparing workspace" "$SCRIPTS_DIR/workspace/prepare.sh"
if [[ "$secret_provider" == "none" ]]; then
    template_dest="$DECLAREST_WORK_DIR/templates/repo-no-secrets"
    run_step "Preparing template (no secrets)" "$REPO_SCRIPTS_DIR/strip-secrets.sh" "$DECLAREST_TEST_DIR/templates/repo" "$template_dest"
    export DECLAREST_TEMPLATE_REPO_DIR="$template_dest"
fi
run_step "Building declarest CLI" "$SCRIPTS_DIR/declarest/build.sh"
run_step "Starting stack" "$SCRIPTS_DIR/stack/start-compose.sh"

provider_label=""
provider_env=""
provider_setup=""
if [[ "$repo_type" == "git-remote" ]]; then
    case "$repo_provider" in
        gitlab)
            provider_label="GitLab"
            provider_env="$DECLAREST_WORK_DIR/gitlab.env"
            provider_setup="$PROVIDER_SCRIPTS_DIR/gitlab/setup.sh"
            ;;
        gitea)
            provider_label="Gitea"
            provider_env="$DECLAREST_WORK_DIR/gitea.env"
            provider_setup="$PROVIDER_SCRIPTS_DIR/gitea/setup.sh"
            ;;
        github)
            provider_label="GitHub"
            ;;
    esac
fi

run_step "Preparing services" "$SCRIPTS_DIR/stack/prepare-services-e2e.sh" "$provider_setup"

if [[ -n "$provider_setup" ]]; then
    if [[ ! -f "$provider_env" ]]; then
        die "${provider_label} env file missing: $provider_env"
    fi
    # shellcheck source=/dev/null
    source "$provider_env"
fi

configure_server_auth "$server_auth_primary"
configure_secret_auth "$secret_auth_primary"
if [[ "$repo_type" == "git-remote" ]]; then
    configure_repo_auth "$repo_auth_primary"
fi
set_context "primary"

run_step "Preparing repo (primary)" "$REPO_SCRIPTS_DIR/prepare.sh"
run_step "Configuring declarest context (primary)" "$SCRIPTS_DIR/context/render.sh"
run_step "Registering declarest context (primary)" "$SCRIPTS_DIR/context/register.sh"
run_step "Validating OpenAPI defaults (primary)" "$SCRIPTS_DIR/declarest/openapi-smoke.sh"
run_step "Validating metadata inference (primary)" "$SCRIPTS_DIR/declarest/metadata-infer-smoke.sh"
run_step "Validating metadata edit (primary)" "$SCRIPTS_DIR/declarest/metadata-edit-smoke.sh"
if [[ "$secret_provider" == "none" && "$repo_type" == "git-remote" ]]; then
    run_step "Sanitizing repository (primary)" "$REPO_SCRIPTS_DIR/strip-secrets.sh" "$DECLAREST_REPO_DIR"
fi
run_step "Running declarest workflow (primary)" "$SCRIPTS_DIR/declarest/run.sh"
if [[ "$repo_type" == "git-remote" ]]; then
    run_step "Verifying remote repo (primary)" "$REPO_SCRIPTS_DIR/verify.sh"
elif [[ "$repo_type" == "git-local" ]]; then
    run_step "Printing git log (primary)" "$REPO_SCRIPTS_DIR/print-log.sh"
fi

for server_auth in "${server_auth_secondary[@]}"; do
    configure_server_auth "$server_auth"
    configure_secret_auth "$secret_auth_primary"
    if [[ "$repo_type" == "git-remote" ]]; then
        configure_repo_auth "$repo_auth_primary"
    fi
    set_context "server-${server_auth}"
    run_step "Validating server auth (${server_auth})" "$SCRIPTS_DIR/declarest/server-auth-smoke.sh"
done

if [[ "$secret_provider" == "vault" ]]; then
    for vault_auth in "${secret_auth_secondary[@]}"; do
        configure_server_auth "$server_auth_primary"
        configure_secret_auth "$vault_auth"
        if [[ "$repo_type" == "git-remote" ]]; then
            configure_repo_auth "$repo_auth_primary"
        fi
        set_context "vault-${vault_auth}"
        run_step "Validating vault auth (${vault_auth})" "$SCRIPTS_DIR/declarest/secret-auth-smoke.sh"
    done
fi

if [[ "$repo_type" == "git-remote" ]]; then
    for repo_auth in "${repo_auth_secondary[@]}"; do
        configure_repo_auth "$repo_auth"
        run_step "Validating repo auth (${repo_auth})" "$REPO_SCRIPTS_DIR/auth-smoke.sh"
    done
fi

print_step_result "DONE" "$TOTAL_STEPS/$TOTAL_STEPS" "Completing E2E flow" ""
log_line "E2E test completed successfully"
echo "E2E test completed successfully. Log: $RUN_LOG"
