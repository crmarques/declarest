#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"
source "$SCRIPTS_DIR/lib/shell.sh"
source "$SCRIPTS_DIR/lib/git-auth.sh"

require_cmd git

remote_url="${DECLAREST_REPO_REMOTE_URL:-}"
if [[ -z "$remote_url" ]]; then
    die "Remote repository URL is not configured"
fi

auth_type="${DECLAREST_REPO_AUTH_TYPE:-}"
auth_type="${auth_type,,}"
if [[ -z "$auth_type" ]]; then
    die "Repo auth type is required for auth smoke test"
fi

case "$auth_type" in
    pat)
        if [[ -z "${DECLAREST_REPO_AUTH:-}" ]]; then
            die "PAT credentials are required for repo auth smoke test"
        fi
        provider="${DECLAREST_REPO_PROVIDER:-}"
        provider="${provider,,}"
        pat_user="$(pat_username_for_provider "$provider")"
        askpass="$(ensure_askpass "${DECLAREST_GIT_ASKPASS:-${DECLAREST_GITLAB_GIT_ASKPASS:-}}")"
        export GIT_ASKPASS="$askpass"
        export GIT_USERNAME="$pat_user"
        export GIT_PASSWORD="$DECLAREST_REPO_AUTH"
        export GIT_TERMINAL_PROMPT=0
        ;;
    basic)
        if [[ -z "${DECLAREST_REPO_AUTH:-}" ]]; then
            die "Basic credentials are required for repo auth smoke test"
        fi
        user="${DECLAREST_REPO_AUTH%%:*}"
        pass="${DECLAREST_REPO_AUTH#*:}"
        if [[ -z "$user" || -z "$pass" ]]; then
            die "Basic auth must be in '<user>:<password>' format"
        fi
        askpass="$(ensure_askpass "${DECLAREST_GIT_ASKPASS:-${DECLAREST_GITLAB_GIT_ASKPASS:-}}")"
        export GIT_ASKPASS="$askpass"
        export GIT_USERNAME="$user"
        export GIT_PASSWORD="$pass"
        export GIT_TERMINAL_PROMPT=0
        ;;
    ssh)
        key_file="${DECLAREST_REPO_SSH_KEY_FILE:-}"
        if [[ -z "$key_file" || ! -f "$key_file" ]]; then
            die "SSH key file not found for repo auth smoke test"
        fi
        case "${DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY:-}" in
            1|true|yes|y)
                export GIT_SSH_COMMAND="ssh -i ${key_file} -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
                ;;
            *)
                known_hosts="${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}"
                if [[ -z "$known_hosts" || ! -f "$known_hosts" ]]; then
                    die "SSH known hosts file not found for repo auth smoke test"
                fi
                export GIT_SSH_COMMAND="ssh -i ${key_file} -o UserKnownHostsFile=${known_hosts} -o StrictHostKeyChecking=yes"
                ;;
        esac
        export GIT_TERMINAL_PROMPT=0
        ;;
    *)
        die "Unsupported repo auth type: ${auth_type}"
        ;;
esac

branch="declarest-auth-${auth_type}-${DECLAREST_RUN_ID}"
work_dir="$DECLAREST_WORK_DIR/auth-smoke-${auth_type}-${DECLAREST_RUN_ID}"

cleanup() {
    rm -rf "$work_dir"
}
trap cleanup EXIT INT TERM

rm -rf "$work_dir"

run_logged "repo auth read" git ls-remote "$remote_url" "HEAD"
run_logged "repo auth clone" git clone "$remote_url" "$work_dir"

git -C "$work_dir" config user.name "Declarest E2E"
git -C "$work_dir" config user.email "declarest-e2e@example.com"

git -C "$work_dir" checkout -b "$branch" >/dev/null 2>&1
printf "auth smoke %s\n" "$(date -Iseconds)" > "$work_dir/auth-smoke-${auth_type}.txt"
git -C "$work_dir" add "auth-smoke-${auth_type}.txt"
git -C "$work_dir" commit -m "Auth smoke ${auth_type}" >/dev/null 2>&1

run_logged "repo auth push" git -C "$work_dir" push origin "$branch"
if ! git -C "$work_dir" push origin ":$branch" >/dev/null 2>&1; then
    log_line "Repo auth smoke: unable to delete remote branch ${branch}"
fi

log_line "Repo auth smoke completed (${auth_type})"
