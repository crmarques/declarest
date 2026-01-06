#!/usr/bin/env bash

ensure_askpass() {
    local askpass="$1"
    if [[ -n "$askpass" && -f "$askpass" ]]; then
        printf "%s" "$askpass"
        return 0
    fi
    askpass="$DECLAREST_WORK_DIR/git-askpass.sh"
    cat <<'EOF' > "$askpass"
#!/usr/bin/env bash
case "$1" in
    *Username*) printf "%s" "${GIT_USERNAME:-}" ;;
    *Password*) printf "%s" "${GIT_PASSWORD:-}" ;;
    *) printf "%s" "${GIT_PASSWORD:-}" ;;
esac
EOF
    chmod 700 "$askpass"
    printf "%s" "$askpass"
}

pat_username_for_provider() {
    local provider="${1:-}"
    provider="${provider,,}"
    local pat_user="oauth2"
    case "$provider" in
        github)
            pat_user="x-access-token"
            ;;
        gitlab|"")
            pat_user="oauth2"
            ;;
        gitea)
            pat_user="${DECLAREST_GITEA_USER:-}"
            ;;
    esac
    if [[ -z "$pat_user" && "$provider" == "gitea" ]]; then
        pat_user="$DECLAREST_REPO_AUTH"
    fi
    printf "%s" "$pat_user"
}

git_ls_remote_head() {
    local branch="$1"
    local remote="$DECLAREST_REPO_REMOTE_URL"
    local auth_type="${DECLAREST_REPO_AUTH_TYPE:-}"
    auth_type="${auth_type,,}"

    case "$auth_type" in
        pat)
            local provider="${DECLAREST_REPO_PROVIDER:-}"
            provider="${provider,,}"
            local pat_user
            pat_user="$(pat_username_for_provider "$provider")"
            local askpass
            askpass="$(ensure_askpass "${DECLAREST_GIT_ASKPASS:-${DECLAREST_GITLAB_GIT_ASKPASS:-}}")"
            (
                export GIT_ASKPASS="$askpass"
                export GIT_USERNAME="$pat_user"
                export GIT_PASSWORD="$DECLAREST_REPO_AUTH"
                export GIT_TERMINAL_PROMPT=0
                git ls-remote "$remote" "refs/heads/$branch" | awk '{print $1}'
            )
            ;;
        basic)
            local creds="$DECLAREST_REPO_AUTH"
            local user="${creds%%:*}"
            local pass="${creds#*:}"
            if [[ -z "$user" || -z "$pass" ]]; then
                die "Invalid basic auth credentials"
            fi
            local askpass
            askpass="$(ensure_askpass "${DECLAREST_GIT_ASKPASS:-${DECLAREST_GITLAB_GIT_ASKPASS:-}}")"
            (
                export GIT_ASKPASS="$askpass"
                export GIT_USERNAME="$user"
                export GIT_PASSWORD="$pass"
                export GIT_TERMINAL_PROMPT=0
                git ls-remote "$remote" "refs/heads/$branch" | awk '{print $1}'
            )
            ;;
        ssh)
            local key_file="${DECLAREST_REPO_SSH_KEY_FILE:-}"
            local known_hosts="${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}"
            if [[ -z "$key_file" || ! -f "$key_file" ]]; then
                die "SSH key file not found for repo verification"
            fi
            if [[ -z "$known_hosts" || ! -f "$known_hosts" ]]; then
                die "Known hosts file not found for repo verification"
            fi
            (
                export GIT_SSH_COMMAND="ssh -i ${key_file} -o UserKnownHostsFile=${known_hosts} -o StrictHostKeyChecking=yes"
                git ls-remote "$remote" "refs/heads/$branch" | awk '{print $1}'
            )
            ;;
        "")
            git ls-remote "$remote" "refs/heads/$branch" | awk '{print $1}'
            ;;
        *)
            die "Unsupported repo auth type: $auth_type"
            ;;
    esac
}

repo_name_from_url() {
    local url="$1"
    local name="${url##*/}"
    name="${name%.git}"
    name="${name%/}"
    if [[ -z "$name" || "$name" == "$url" ]]; then
        name="repo"
    fi
    printf "%s" "$name"
}

resolve_clone_dir() {
    local base="${DECLAREST_OUTPUT_GIT_CLONE_DIR:-}"
    if [[ -z "$base" ]]; then
        printf "%s" ""
        return 0
    fi
    base="${base%/}"
    local name
    name="$(repo_name_from_url "$DECLAREST_REPO_REMOTE_URL")"
    local dir="${base}/${name}"
    if [[ -e "$dir" ]]; then
        local suffix="${DECLAREST_RUN_ID:-$(date +%s)}"
        dir="${base}/${name}-${suffix}"
        local counter=1
        while [[ -e "$dir" ]]; do
            dir="${base}/${name}-${suffix}-${counter}"
            counter=$((counter + 1))
        done
    fi
    printf "%s" "$dir"
}

clone_remote_repo() {
    local dest="$1"
    local remote="$DECLAREST_REPO_REMOTE_URL"
    local auth_type="${DECLAREST_REPO_AUTH_TYPE:-}"
    auth_type="${auth_type,,}"

    case "$auth_type" in
        pat)
            local provider="${DECLAREST_REPO_PROVIDER:-}"
            local pat_user
            pat_user="$(pat_username_for_provider "$provider")"
            local askpass
            askpass="$(ensure_askpass "${DECLAREST_GIT_ASKPASS:-${DECLAREST_GITLAB_GIT_ASKPASS:-}}")"
            (
                export GIT_ASKPASS="$askpass"
                export GIT_USERNAME="$pat_user"
                export GIT_PASSWORD="$DECLAREST_REPO_AUTH"
                export GIT_TERMINAL_PROMPT=0
                run_logged "clone remote repo" git clone "$remote" "$dest"
            )
            ;;
        basic)
            local creds="$DECLAREST_REPO_AUTH"
            local user="${creds%%:*}"
            local pass="${creds#*:}"
            if [[ -z "$user" || -z "$pass" ]]; then
                die "Invalid basic auth credentials"
            fi
            local askpass
            askpass="$(ensure_askpass "${DECLAREST_GIT_ASKPASS:-${DECLAREST_GITLAB_GIT_ASKPASS:-}}")"
            (
                export GIT_ASKPASS="$askpass"
                export GIT_USERNAME="$user"
                export GIT_PASSWORD="$pass"
                export GIT_TERMINAL_PROMPT=0
                run_logged "clone remote repo" git clone "$remote" "$dest"
            )
            ;;
        ssh)
            local key_file="${DECLAREST_REPO_SSH_KEY_FILE:-}"
            local known_hosts="${DECLAREST_REPO_SSH_KNOWN_HOSTS_FILE:-}"
            if [[ -z "$key_file" || ! -f "$key_file" ]]; then
                die "SSH key file not found for repo verification"
            fi
            if [[ -z "$known_hosts" || ! -f "$known_hosts" ]]; then
                die "Known hosts file not found for repo verification"
            fi
            (
                export GIT_SSH_COMMAND="ssh -i ${key_file} -o UserKnownHostsFile=${known_hosts} -o StrictHostKeyChecking=yes"
                run_logged "clone remote repo" git clone "$remote" "$dest"
            )
            ;;
        "")
            run_logged "clone remote repo" git clone "$remote" "$dest"
            ;;
        *)
            die "Unsupported repo auth type: $auth_type"
            ;;
    esac
}
