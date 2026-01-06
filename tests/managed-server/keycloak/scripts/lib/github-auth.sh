#!/usr/bin/env bash

GITHUB_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./prompts.sh
source "$GITHUB_LIB_DIR/prompts.sh"
# shellcheck source=./shell.sh
source "$GITHUB_LIB_DIR/shell.sh"

github_pat_load_defaults() {
    github_https_url="${github_https_url:-${DECLAREST_GITHUB_HTTPS_URL:-${GITHUB_HTTPS_URL:-}}}"
    github_pat="${github_pat:-${DECLAREST_GITHUB_PAT:-${GITHUB_PAT:-}}}"
}

github_pat_ssh_load_defaults() {
    github_pat_load_defaults
    github_ssh_url="${github_ssh_url:-${DECLAREST_GITHUB_SSH_URL:-${GITHUB_SSH_URL:-}}}"
    github_ssh_key_file="${github_ssh_key_file:-${DECLAREST_GITHUB_SSH_KEY_FILE:-${GITHUB_SSH_KEY_FILE:-}}}"
    github_ssh_known_hosts="${github_ssh_known_hosts:-${DECLAREST_GITHUB_SSH_KNOWN_HOSTS_FILE:-${GITHUB_SSH_KNOWN_HOSTS_FILE:-}}}"
    github_ssh_insecure="${github_ssh_insecure:-${DECLAREST_GITHUB_SSH_INSECURE_IGNORE_HOST_KEY:-${GITHUB_SSH_INSECURE_IGNORE_HOST_KEY:-}}}"
}

ensure_github_pat_credentials() {
    if [[ "${repo_provider:-}" != "github" ]]; then
        return 0
    fi

    github_pat_load_defaults
    if ! is_interactive; then
        if [[ -z "$github_https_url" || -z "$github_pat" ]]; then
            die "GitHub credentials missing; set DECLAREST_GITHUB_HTTPS_URL and DECLAREST_GITHUB_PAT"
        fi
        return 0
    fi

    if [[ -z "$github_https_url" ]]; then
        github_https_url="$(prompt_required "GitHub HTTPS repo URL (PAT): ")"
    fi
    if [[ -z "$github_pat" ]]; then
        github_pat="$(prompt_secret_required "GitHub PAT token: ")"
    fi
}

ensure_github_pat_ssh_credentials() {
    if [[ "${repo_provider:-}" != "github" ]]; then
        return 0
    fi

    github_pat_ssh_load_defaults
    if ! is_interactive; then
        if [[ -z "$github_https_url" || -z "$github_pat" || -z "$github_ssh_url" || -z "$github_ssh_key_file" ]]; then
            die "GitHub credentials missing; set DECLAREST_GITHUB_HTTPS_URL, DECLAREST_GITHUB_PAT, DECLAREST_GITHUB_SSH_URL, and DECLAREST_GITHUB_SSH_KEY_FILE"
        fi
        return 0
    fi

    if [[ -z "$github_https_url" ]]; then
        github_https_url="$(prompt_required "GitHub HTTPS repo URL (PAT): ")"
    fi
    if [[ -z "$github_pat" ]]; then
        github_pat="$(prompt_secret_required "GitHub PAT token: ")"
    fi
    if [[ -z "$github_ssh_url" ]]; then
        github_ssh_url="$(prompt_required "GitHub SSH repo URL: ")"
    fi
    if [[ -z "$github_ssh_key_file" ]]; then
        github_ssh_key_file="$(prompt_required "GitHub SSH private key file: ")"
    fi
    if [[ -z "$github_ssh_known_hosts" ]]; then
        github_ssh_known_hosts="$(prompt_optional "GitHub SSH known hosts file (leave blank to use ~/.ssh/known_hosts): ")"
    fi
    if [[ -z "$github_ssh_insecure" ]]; then
        insecure_choice="$(prompt_optional "Ignore SSH host key verification? (y/N): ")"
        case "${insecure_choice,,}" in
            y|yes)
                github_ssh_insecure="true"
                ;;
            n|no|"")
                ;;
            *)
                die "Invalid choice: ${insecure_choice}"
                ;;
        esac
    fi
}
