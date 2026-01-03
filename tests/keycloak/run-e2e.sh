#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$KEYCLOAK_DIR/scripts"

usage() {
    cat <<EOF
Usage: ./tests/keycloak/run-e2e.sh [--repo-type TYPE] [--server-auth-type TYPE]
                               [--remote-repo-provider PROVIDER]
                               [--output-git-clone-dir DIR]

Options:
  --repo-type TYPE        Repository type for the context file:
                            fs: filesystem repository (default)
                            git-local: git local-only repository
                            git-remote: git local + remote repository
  --server-auth-type TYPE Managed server auth type:
                          oauth2 (default) or basic
  --remote-repo-provider PROVIDER
                          Remote git provider for git-remote tests:
                          gitea (default) or gitlab
  --output-git-clone-dir DIR
                          Clone remote git repositories into DIR when using
                          --repo-type git-remote. Defaults to
                          <working-dir>/output-git-clone
  -h, --help              Show this help message.
EOF
}

die() {
    printf "Error: %s\n" "$1" >&2
    exit 1
}

require_arg() {
    local opt="$1"
    local value="${2:-}"
    if [[ -z "$value" ]]; then
        die "Missing value for ${opt}"
    fi
}

repo_type="fs"
server_auth_type="oauth2"
remote_repo_provider="gitea"
output_git_clone_dir=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo-type)
            require_arg "$1" "${2:-}"
            repo_type="${2:-}"
            shift 2
            ;;
        --server-auth-type)
            require_arg "$1" "${2:-}"
            server_auth_type="${2:-}"
            shift 2
            ;;
        --remote-repo-provider)
            require_arg "$1" "${2:-}"
            remote_repo_provider="${2:-}"
            shift 2
            ;;
        --output-git-clone-dir)
            require_arg "$1" "${2:-}"
            output_git_clone_dir="${2:-}"
            shift 2
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

repo_type="${repo_type,,}"
case "$repo_type" in
    fs|git-local|git-remote)
        ;;
    *)
        die "Invalid --repo-type: ${repo_type} (expected fs, git-local, or git-remote)"
        ;;
esac

server_auth_type="${server_auth_type,,}"
case "$server_auth_type" in
    oauth2|basic)
        ;;
    *)
        die "Invalid --server-auth-type: ${server_auth_type} (expected oauth2 or basic)"
        ;;
esac

remote_repo_provider="${remote_repo_provider,,}"
case "$remote_repo_provider" in
    gitea|gitlab)
        ;;
    *)
        die "Invalid --remote-repo-provider: ${remote_repo_provider} (expected gitea or gitlab)"
        ;;
esac

export DECLAREST_REPO_TYPE="$repo_type"
export DECLAREST_SERVER_AUTH_TYPE="$server_auth_type"
if [[ "$repo_type" == "git-remote" ]]; then
    export DECLAREST_REMOTE_REPO_PROVIDER="$remote_repo_provider"
    case "$remote_repo_provider" in
        gitlab)
            export DECLAREST_GITLAB_ENABLE="1"
            export DECLAREST_GITEA_ENABLE="0"
            ;;
        gitea)
            export DECLAREST_GITLAB_ENABLE="0"
            export DECLAREST_GITEA_ENABLE="1"
            ;;
    esac
else
    export DECLAREST_GITLAB_ENABLE="0"
    export DECLAREST_GITEA_ENABLE="0"
    export DECLAREST_REMOTE_REPO_PROVIDER=""
fi

# shellcheck source=scripts/lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=scripts/lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"

if [[ -n "$output_git_clone_dir" ]]; then
    export DECLAREST_OUTPUT_GIT_CLONE_DIR="$output_git_clone_dir"
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

auth_methods=()
steps_per_auth=0
case "$repo_type" in
    git-remote)
        auth_methods=(basic pat ssh)
        steps_per_auth=5
        TOTAL_STEPS=$((4 + steps_per_auth * ${#auth_methods[@]}))
        ;;
    git-local)
        TOTAL_STEPS=8
        ;;
    *)
        TOTAL_STEPS=7
        ;;
esac
current_step=0

run_step() {
    local title="$1"
    shift
    local cmd=("$@")

    current_step=$((current_step + 1))
    local label="${current_step}/${TOTAL_STEPS}"
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

echo "Starting Keycloak E2E run"
echo "Detailed log: $RUN_LOG"
log_line "Keycloak E2E run started"
log_line "Container runtime: $CONTAINER_RUNTIME"

run_step "Preparing workspace" "$SCRIPTS_DIR/workspace/prepare.sh"
run_step "Building declarest CLI" "$SCRIPTS_DIR/declarest/build.sh"
run_step "Starting Keycloak" "$SCRIPTS_DIR/stack/start.sh"

if [[ "$repo_type" == "git-remote" ]]; then
    case "$remote_repo_provider" in
        gitlab)
            provider_label="GitLab"
            provider_env="$DECLAREST_WORK_DIR/gitlab.env"
            provider_setup="$SCRIPTS_DIR/providers/gitlab/setup.sh"
            ;;
        gitea)
            provider_label="Gitea"
            provider_env="$DECLAREST_WORK_DIR/gitea.env"
            provider_setup="$SCRIPTS_DIR/providers/gitea/setup.sh"
            ;;
    esac

    run_step "Preparing ${provider_label}" "$provider_setup"

    if [[ ! -f "$provider_env" ]]; then
        die "${provider_label} env file missing: $provider_env"
    fi
    # shellcheck source=/dev/null
    source "$provider_env"

    for auth in "${auth_methods[@]}"; do
        export DECLAREST_REPO_PROVIDER="$remote_repo_provider"
        export DECLAREST_REPO_AUTH_TYPE=""
        export DECLAREST_REPO_AUTH=""
        export DECLAREST_REPO_SSH_USER=""
        export DECLAREST_REPO_SSH_KEY_FILE=""
        export DECLAREST_REPO_SSH_PASSPHRASE=""
        export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE=""
        export DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY=""

        case "$auth" in
            basic)
                case "$remote_repo_provider" in
                    gitlab)
                        export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITLAB_BASIC_URL"
                        export DECLAREST_REPO_AUTH_TYPE="basic"
                        export DECLAREST_REPO_AUTH="${DECLAREST_GITLAB_USER}:${DECLAREST_GITLAB_PASSWORD}"
                        ;;
                    gitea)
                        export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITEA_BASIC_URL"
                        export DECLAREST_REPO_AUTH_TYPE="basic"
                        export DECLAREST_REPO_AUTH="${DECLAREST_GITEA_USER}:${DECLAREST_GITEA_PASSWORD}"
                        ;;
                esac
                ;;
            pat)
                case "$remote_repo_provider" in
                    gitlab)
                        export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITLAB_PAT_URL"
                        export DECLAREST_REPO_AUTH_TYPE="pat"
                        export DECLAREST_REPO_AUTH="${DECLAREST_GITLAB_PAT}"
                        ;;
                    gitea)
                        export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITEA_PAT_URL"
                        export DECLAREST_REPO_AUTH_TYPE="pat"
                        export DECLAREST_REPO_AUTH="${DECLAREST_GITEA_PAT}"
                        ;;
                esac
                ;;
            ssh)
                case "$remote_repo_provider" in
                    gitlab)
                        export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITLAB_SSH_URL"
                        export DECLAREST_REPO_AUTH_TYPE="ssh"
                        export DECLAREST_REPO_SSH_USER="git"
                        export DECLAREST_REPO_SSH_KEY_FILE="$DECLAREST_GITLAB_SSH_KEY_FILE"
                        export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$DECLAREST_GITLAB_KNOWN_HOSTS_FILE"
                        ;;
                    gitea)
                        export DECLAREST_REPO_REMOTE_URL="$DECLAREST_GITEA_SSH_URL"
                        export DECLAREST_REPO_AUTH_TYPE="ssh"
                        export DECLAREST_REPO_SSH_USER="git"
                        export DECLAREST_REPO_SSH_KEY_FILE="$DECLAREST_GITEA_SSH_KEY_FILE"
                        export DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="$DECLAREST_GITEA_KNOWN_HOSTS_FILE"
                        ;;
                esac
                ;;
        esac

        export DECLAREST_REPO_DIR="$DECLAREST_WORK_DIR/repo-${auth}"
        export DECLAREST_CONTEXT_FILE="$DECLAREST_WORK_DIR/context-${auth}.yaml"
        export DECLAREST_CONTEXT_NAME="keycloak-e2e-${auth}"

        run_step "Preparing repo (${auth})" "$SCRIPTS_DIR/repo/prepare.sh"
        run_step "Configuring declarest context (${auth})" "$SCRIPTS_DIR/context/render.sh"
        run_step "Registering declarest context (${auth})" "$SCRIPTS_DIR/context/register.sh"
        run_step "Running declarest workflow (${auth})" "$SCRIPTS_DIR/declarest/run.sh"
        run_step "Verifying remote repo (${auth})" "$SCRIPTS_DIR/repo/verify.sh"
    done
else
    run_step "Preparing template repo" "$SCRIPTS_DIR/repo/prepare.sh"
    run_step "Configuring declarest context" "$SCRIPTS_DIR/context/render.sh"
    run_step "Registering declarest context" "$SCRIPTS_DIR/context/register.sh"
    run_step "Running declarest workflow" "$SCRIPTS_DIR/declarest/run.sh"
    if [[ "$repo_type" == "git-local" ]]; then
        run_step "Printing git log" "$SCRIPTS_DIR/repo/print-log.sh"
    fi
fi

print_step_result "DONE" "$TOTAL_STEPS/$TOTAL_STEPS" "Completing E2E flow" ""
log_line "E2E test completed successfully"
echo "E2E test completed successfully. Log: $RUN_LOG"
