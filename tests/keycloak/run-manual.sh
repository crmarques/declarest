#!/usr/bin/env bash

set -euo pipefail

KEYCLOAK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
    cat <<EOF
Usage: ./tests/keycloak/run-manual.sh [--sync-resource] [--work-dir PATH] [--repo-type TYPE] [--server-auth-type TYPE] [git-remote options]

Options:
  --sync-resource        Sync the template repository to Keycloak during startup.
  --work-dir PATH        Use a fixed work directory.
  --repo-type TYPE       Repository type for the context file:
                            fs: filesystem repository (default)
                            git-local: git local-only repository
                            git-remote: git local + remote repository
  --populate-repo        Seed the repository with the default template resources
                         when using --repo-type git-remote.
  --server-auth-type TYPE
                         Managed server auth type:
                         oauth2 (default) or basic

Git-remote options (only with --repo-type git-remote):
  --git-repo URL         Git URL to the remote repository.
  --git-provider NAME    Remote provider: gitlab, gitea, or github.
  --git-auth-type TYPE   Authentication type: basic, pat, or ssh.
  --git-auth STRING      Credentials string for git-auth-type:
                         basic: "<user>:<password>"
                         pat: "<token>"
  --git-ssh-user USER    SSH username (optional).
  --git-ssh-key-file PATH
                         SSH private key file (required for ssh).
  --git-ssh-passphrase PASS
                         SSH passphrase (optional).
  --git-ssh-known-hosts PATH
                         SSH known hosts file (optional).
  --git-ssh-ignore-host-key
                         Disable SSH host key verification.
  -h, --help             Show this help message.

If --repo-type git-remote is selected and any git-remote options are missing,
the script will prompt for them.
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

is_tty() {
    [[ -t 0 ]]
}

prompt_required() {
    local prompt="$1"
    local value=""
    while [[ -z "$value" ]]; do
        read -r -p "$prompt" value
    done
    printf "%s" "$value"
}

prompt_optional() {
    local prompt="$1"
    local value=""
    read -r -p "$prompt" value || true
    printf "%s" "$value"
}

prompt_secret_required() {
    local prompt="$1"
    local value=""
    while [[ -z "$value" ]]; do
        read -r -s -p "$prompt" value
        printf "\n"
    done
    printf "%s" "$value"
}

prompt_choice() {
    local prompt="$1"
    shift
    local choices=("$@")
    local value=""
    while true; do
        read -r -p "$prompt" value
        value="${value,,}"
        for choice in "${choices[@]}"; do
            if [[ "$value" == "$choice" ]]; then
                printf "%s" "$value"
                return 0
            fi
        done
        printf "Invalid choice: %s\n" "$value" >&2
    done
}

prompt_choice_default() {
    local prompt="$1"
    local default="$2"
    shift 2
    local choices=("$@")
    local value=""

    while true; do
        read -r -p "$prompt" value
        value="${value,,}"
        if [[ -z "$value" && -n "$default" ]]; then
            value="${default,,}"
        fi
        for choice in "${choices[@]}"; do
            if [[ "$value" == "$choice" ]]; then
                printf "%s" "$value"
                return 0
            fi
        done
        printf "Invalid choice: %s\n" "$value" >&2
    done
}

infer_git_provider() {
    local repo_url="$1"
    local normalized host

    normalized="${repo_url,,}"
    normalized="${normalized#*://}"
    normalized="${normalized#*git@}"
    host="${normalized%%/*}"
    host="${host%%:*}"
    if [[ "$host" == *github* ]]; then
        printf "github"
        return 0
    fi
    if [[ "$host" == *gitlab* ]]; then
        printf "gitlab"
        return 0
    fi
    if [[ "$host" == *gitea* ]]; then
        printf "gitea"
        return 0
    fi
    printf ""
}

args=()
repo_type="fs"
server_auth_type="oauth2"
work_dir_override=""
git_repo=""
git_provider=""
git_auth_type=""
git_auth_value=""
git_ssh_user=""
git_ssh_key_file=""
git_ssh_passphrase=""
git_ssh_known_hosts=""
git_ssh_insecure=""
populate_repo=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --sync-resource|--sync)
            args+=(--sync)
            shift
            ;;
        --work-dir)
            require_arg "$1" "${2:-}"
            work_dir_override="${2:-}"
            args+=(--work-dir "${2:-}")
            shift 2
            ;;
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
        --populate-repo)
            populate_repo="1"
            shift
            ;;
        --git-repo)
            require_arg "$1" "${2:-}"
            git_repo="${2:-}"
            shift 2
            ;;
        --git-provider)
            require_arg "$1" "${2:-}"
            git_provider="${2:-}"
            shift 2
            ;;
        --git-auth-type)
            require_arg "$1" "${2:-}"
            git_auth_type="${2:-}"
            shift 2
            ;;
        --auth-type)
            require_arg "$1" "${2:-}"
            git_auth_type="${2:-}"
            shift 2
            ;;
        --git-auth)
            require_arg "$1" "${2:-}"
            git_auth_value="${2:-}"
            shift 2
            ;;
        --auth)
            require_arg "$1" "${2:-}"
            git_auth_value="${2:-}"
            shift 2
            ;;
        --git-ssh-user)
            require_arg "$1" "${2:-}"
            git_ssh_user="${2:-}"
            shift 2
            ;;
        --git-ssh-key-file)
            require_arg "$1" "${2:-}"
            git_ssh_key_file="${2:-}"
            shift 2
            ;;
        --git-ssh-passphrase)
            require_arg "$1" "${2:-}"
            git_ssh_passphrase="${2:-}"
            shift 2
            ;;
        --git-ssh-known-hosts)
            require_arg "$1" "${2:-}"
            git_ssh_known_hosts="${2:-}"
            shift 2
            ;;
        --git-ssh-ignore-host-key)
            git_ssh_insecure="true"
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

project_name=""
if [[ -z "${KEYCLOAK_CONTAINER_NAME:-}" || "$KEYCLOAK_CONTAINER_NAME" == "keycloak-declarest-test" ]]; then
    if [[ -n "$work_dir_override" ]]; then
        project_name="$(basename "$work_dir_override")"
    elif [[ -n "${DECLAREST_WORK_DIR:-}" ]]; then
        project_name="$(basename "$DECLAREST_WORK_DIR")"
    fi
    if [[ -n "$project_name" ]]; then
        export KEYCLOAK_CONTAINER_NAME="${project_name}_keycloak-declarest-test_1"
    fi
fi

if [[ "$repo_type" != "git-remote" ]]; then
    if [[ -n "$git_repo" || -n "$git_provider" || -n "$git_auth_type" || -n "$git_auth_value" || -n "$git_ssh_user" || -n "$git_ssh_key_file" || -n "$git_ssh_passphrase" || -n "$git_ssh_known_hosts" || -n "$git_ssh_insecure" ]]; then
        die "Git-remote options require --repo-type git-remote"
    fi
elif [[ -z "$populate_repo" ]]; then
    populate_repo="0"
fi

if [[ "$repo_type" == "git-remote" ]]; then
    if ! is_tty; then
        if [[ -z "$git_repo" || -z "$git_provider" || -z "$git_auth_type" ]]; then
            die "Missing git-remote options; provide --git-repo, --git-provider, and --git-auth-type"
        fi
        git_auth_type="${git_auth_type,,}"
        if [[ "$git_auth_type" == "token" ]]; then
            git_auth_type="pat"
        fi
        case "$git_auth_type" in
            basic|pat)
                if [[ -z "$git_auth_value" ]]; then
                    die "Missing git auth credentials; provide --git-auth"
                fi
                ;;
            ssh)
                if [[ -z "$git_ssh_key_file" ]]; then
                    die "Missing SSH key file; provide --git-ssh-key-file"
                fi
                ;;
            *)
                die "Invalid --git-auth-type: ${git_auth_type} (expected basic, pat, or ssh)"
                ;;
        esac
    fi

    if [[ -z "$git_repo" ]]; then
        git_repo="$(prompt_required "Git repository URL: ")"
    fi

    if [[ -z "$git_provider" ]]; then
        inferred_provider="$(infer_git_provider "$git_repo")"
        if [[ -n "$inferred_provider" ]]; then
            git_provider="$(prompt_choice_default "Git provider (gitlab/gitea/github) [${inferred_provider}]: " "$inferred_provider" gitlab gitea github)"
        else
            git_provider="$(prompt_choice "Git provider (gitlab/gitea/github): " gitlab gitea github)"
        fi
    else
        git_provider="${git_provider,,}"
    fi

    case "$git_provider" in
        gitlab|gitea|github)
            ;;
        *)
            die "Invalid --git-provider: ${git_provider} (expected gitlab, gitea, or github)"
            ;;
    esac

    if [[ -z "$git_auth_type" ]]; then
        git_auth_type="$(prompt_choice "Git auth type (basic/pat/ssh): " basic pat ssh token)"
    else
        git_auth_type="${git_auth_type,,}"
    fi

    if [[ "$git_auth_type" == "token" ]]; then
        git_auth_type="pat"
    fi

    case "$git_auth_type" in
        basic|pat|ssh)
            ;;
        *)
            die "Invalid --git-auth-type: ${git_auth_type} (expected basic, pat, or ssh)"
            ;;
    esac

    if [[ "$git_auth_type" == "basic" || "$git_auth_type" == "pat" ]]; then
        if [[ -z "$git_auth_value" ]]; then
            if [[ "$git_auth_type" == "basic" ]]; then
                git_auth_value="$(prompt_required "Auth (<user>:<password>): ")"
            else
                git_auth_value="$(prompt_secret_required "Auth token (PAT): ")"
            fi
        fi
    else
        if [[ -n "$git_auth_value" ]]; then
            die "SSH auth does not use --git-auth"
        fi
        if [[ -z "$git_ssh_key_file" ]]; then
            git_ssh_key_file="$(prompt_required "SSH private key file: ")"
        fi
        if is_tty; then
            if [[ -z "$git_ssh_user" ]]; then
                git_ssh_user="$(prompt_optional "SSH user (leave blank to autodetect): ")"
            fi
            if [[ -z "$git_ssh_passphrase" ]]; then
                git_ssh_passphrase="$(prompt_optional "SSH passphrase (leave blank for none): ")"
            fi
            if [[ -z "$git_ssh_known_hosts" ]]; then
                git_ssh_known_hosts="$(prompt_optional "SSH known hosts file (leave blank for default): ")"
            fi
            if [[ -z "$git_ssh_insecure" ]]; then
                insecure_choice="$(prompt_optional "Ignore host key verification? (y/N): ")"
                case "${insecure_choice,,}" in
                    y|yes)
                        git_ssh_insecure="true"
                        ;;
                    n|no|"")
                        ;;
                    *)
                        die "Invalid choice: ${insecure_choice}"
                        ;;
                esac
            fi
        fi
    fi

    if [[ "$git_auth_type" == "basic" ]]; then
        if [[ "$git_auth_value" != *:* ]]; then
            die "Basic auth must be in '<user>:<password>' format"
        fi
        user="${git_auth_value%%:*}"
        pass="${git_auth_value#*:}"
        if [[ -z "$user" || -z "$pass" ]]; then
            die "Basic auth requires both username and password"
        fi
    fi
fi

DECLAREST_REPO_TYPE="$repo_type" \
DECLAREST_SERVER_AUTH_TYPE="$server_auth_type" \
DECLAREST_REPO_REMOTE_URL="${git_repo:-}" \
DECLAREST_REPO_PROVIDER="${git_provider:-}" \
DECLAREST_REPO_AUTH_TYPE="${git_auth_type:-}" \
DECLAREST_REPO_AUTH="${git_auth_value:-}" \
DECLAREST_REPO_SSH_USER="${git_ssh_user:-}" \
DECLAREST_REPO_SSH_KEY_FILE="${git_ssh_key_file:-}" \
DECLAREST_REPO_SSH_PASSPHRASE="${git_ssh_passphrase:-}" \
DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE="${git_ssh_known_hosts:-}" \
DECLAREST_REPO_SSH_INSECURE_IGNORE_HOST_KEY="${git_ssh_insecure:-}" \
DECLAREST_POPULATE_REPO="${populate_repo:-}" \
exec "$KEYCLOAK_DIR/run.sh" setup "${args[@]}"
