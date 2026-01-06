#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"

wait_for_gitea() {
    local base_url="http://localhost:${GITEA_HTTP_PORT}"
    local attempts=${GITEA_WAIT_ATTEMPTS:-200}
    local delay=${GITEA_WAIT_DELAY:-5}
    local api_status=""

    log_line "Waiting for Gitea readiness at ${base_url} (api /api/v1/version, ${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        api_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" "${base_url}/api/v1/version" || true)"
        if [[ "$api_status" == "200" ]]; then
            log_line "Gitea API is ready after attempt ${i}"
            return 0
        fi
        if (( i % 10 == 0 )); then
            log_line "Still waiting for Gitea (${i}/${attempts}; api=${api_status:-?})"
        fi
        sleep "$delay"
    done
    return 1
}

find_gitea_container() {
    local container_id=""

    if [[ -n "${CONTAINER_RUNTIME:-}" && -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
        container_id="$("$CONTAINER_RUNTIME" ps -q \
            --filter "label=com.docker.compose.project=${COMPOSE_PROJECT_NAME}" \
            --filter "label=com.docker.compose.service=gitea" 2>/dev/null | head -n 1 || true)"
    fi
    if [[ -z "$container_id" && -n "${CONTAINER_RUNTIME:-}" && -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
        container_id="$("$CONTAINER_RUNTIME" ps -q \
            --filter "name=${COMPOSE_PROJECT_NAME}_gitea_1" 2>/dev/null | head -n 1 || true)"
    fi

    printf "%s" "$container_id"
}

gitea_admin() {
    local container_id="$1"
    shift
    "$CONTAINER_RUNTIME" exec -i "$container_id" \
        gitea --work-path /data/gitea --config "$GITEA_APP_INI" admin "$@"
}

extract_gitea_token() {
    local output="$1"
    local token

    token="$(printf "%s" "$output" | grep -Eo '[A-Fa-f0-9]{40}' | head -n 1)"
    if [[ -z "$token" ]]; then
        token="$(printf "%s" "$output" | awk '{print $NF}' | tail -n 1)"
    fi
    printf "%s" "$token"
}

gitea_select_flag() {
    local help_text="$1"
    shift
    local flags=("$@")
    local flag

    for flag in "${flags[@]}"; do
        if grep -q -- "$flag" <<<"$help_text"; then
            printf "%s" "$flag"
            return 0
        fi
    done
    printf "%s" ""
}

generate_gitea_token() {
    local container_id="$1"
    local user="$2"
    local name="$3"
    local output status token last_output help_text
    local name_flag user_flag
    local -a cmd args

    for cmd in "user generate-token" "user generate-access-token"; do
        help_text="$(gitea_admin "$container_id" $cmd --help 2>&1 || true)"
        name_flag="$(gitea_select_flag "$help_text" "--token-name" "--name")"
        user_flag="$(gitea_select_flag "$help_text" "--username" "--user" "-u")"

        args=($cmd)
        if [[ -n "$name_flag" ]]; then
            args+=("$name_flag" "$name")
        fi
        if [[ -n "$user_flag" ]]; then
            args+=("$user_flag" "$user")
        else
            args+=("$user")
        fi

        if output="$(gitea_admin "$container_id" "${args[@]}" 2>&1)"; then
            status=0
        else
            status=$?
        fi
        token="$(extract_gitea_token "$output")"
        if [[ $status -eq 0 && -n "$token" ]]; then
            printf "%s" "$token"
            return 0
        fi
        last_output="$output"
    done

    log_block "gitea admin token output" "$last_output"
    return 1
}

ensure_user() {
    local container_id="$1"
    local user_status
    user_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" "${GITEA_URL}/api/v1/users/${GITEA_USER_NAME}" || true)"
    if [[ "$user_status" == "200" ]]; then
        return 0
    fi

    log_line "Creating Gitea user ${GITEA_USER_NAME}"
    set +e
    output="$(gitea_admin "$container_id" user create \
        --username "$GITEA_USER_NAME" \
        --password "$GITEA_USER_PASS" \
        --email "$GITEA_USER_EMAIL" \
        --admin \
        --must-change-password=false 2>&1)"
    status=$?
    set -e
    if [[ $status -ne 0 ]]; then
        log_block "gitea admin user create output" "$output"
        user_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" "${GITEA_URL}/api/v1/users/${GITEA_USER_NAME}" || true)"
        if [[ "$user_status" != "200" ]]; then
            return 1
        fi
    fi
    return 0
}

ensure_gitea_config() {
    local container_id="$1"
    local app_ini_dir
    local config_content
    app_ini_dir="$(dirname "$GITEA_APP_INI")"

    if "$CONTAINER_RUNTIME" exec -i "$container_id" sh -c \
        "test -f '$GITEA_APP_INI' && grep -Eq '^[[:space:]]*INSTALL_LOCK[[:space:]]*=[[:space:]]*true' '$GITEA_APP_INI'"; then
        return 0
    fi

    log_line "Writing Gitea app.ini to ${GITEA_APP_INI}"
    config_content="$(cat <<EOF
APP_NAME = Gitea
RUN_USER = git
RUN_MODE = prod

[server]
PROTOCOL = http
DOMAIN = localhost
HTTP_PORT = 3000
ROOT_URL = http://localhost:${GITEA_HTTP_PORT}/
SSH_DOMAIN = localhost
SSH_LISTEN_PORT = ${GITEA_SSH_PORT}
SSH_PORT = ${GITEA_SSH_PORT}
START_SSH_SERVER = true

[database]
DB_TYPE = sqlite3
PATH = /data/gitea/gitea.db

[security]
INSTALL_LOCK = true

[service]
DISABLE_REGISTRATION = true

[mailer]
ENABLED = false
EOF
)"
    printf "%s\n" "$config_content" | "$CONTAINER_RUNTIME" exec -i "$container_id" sh -c \
        "mkdir -p '$app_ini_dir' && cat > '$GITEA_APP_INI'"
}

api_post_json() {
    local endpoint="$1"
    shift
    curl -sS --fail --request POST \
        --header "Authorization: token ${GITEA_PAT_TOKEN}" \
        --header "Content-Type: application/json" \
        "$GITEA_URL/api/v1${endpoint}" "$@"
}

seed_repo() {
    local project="$1"
    local remote_url="$2"
    local seed_dir="$DECLAREST_WORK_DIR/gitea-seed-${project}"

    rm -rf "$seed_dir"
    mkdir -p "$seed_dir"
    cp -R "$DECLAREST_TEMPLATE_REPO_DIR"/. "$seed_dir"/

    if git init -b main "$seed_dir" >/dev/null 2>&1; then
        :
    else
        git init "$seed_dir" >/dev/null 2>&1
        git -C "$seed_dir" checkout -b main >/dev/null 2>&1
    fi
    git -C "$seed_dir" config user.name "Declarest E2E"
    git -C "$seed_dir" config user.email "declarest-e2e@example.com"
    git -C "$seed_dir" add -A
    git -C "$seed_dir" commit -m "Seed template repository" >/dev/null 2>&1 || true
    git -C "$seed_dir" remote add origin "$remote_url"

    (
        export GIT_ASKPASS="$GITEA_GIT_ASKPASS"
        export GIT_USERNAME="$GITEA_PAT_USER"
        export GIT_PASSWORD="$GITEA_PAT_TOKEN"
        export GIT_TERMINAL_PROMPT=0
        run_logged "seed ${project} (push)" git -C "$seed_dir" push -u origin main
    )
}

require_cmd curl
require_cmd jq
require_cmd git
require_cmd ssh-keygen
require_cmd ssh-keyscan
require_cmd "$CONTAINER_RUNTIME"

if [[ ! -d "$DECLAREST_TEMPLATE_REPO_DIR" ]]; then
    die "Template repository not found at $DECLAREST_TEMPLATE_REPO_DIR"
fi

GITEA_APP_INI="${GITEA_APP_INI:-/data/gitea/conf/app.ini}"
GITEA_URL="http://localhost:${GITEA_HTTP_PORT}"
GITEA_USER_NAME="${GITEA_USER}"
GITEA_USER_PASS="${GITEA_USER_PASSWORD}"
GITEA_USER_EMAIL="${GITEA_USER_EMAIL}"

wait_for_gitea || die "Gitea did not become ready in time"

container_id="$(find_gitea_container)"
if [[ -z "$container_id" ]]; then
    die "Gitea container not found"
fi

ensure_gitea_config "$container_id"
ensure_user "$container_id" || die "Failed to ensure Gitea user ${GITEA_USER_NAME}"

log_line "Creating personal access token for ${GITEA_USER_NAME}"
GITEA_PAT_TOKEN="$(generate_gitea_token "$container_id" "$GITEA_USER_NAME" "declarest-e2e-${DECLAREST_RUN_ID}")"
if [[ -z "$GITEA_PAT_TOKEN" ]]; then
    die "Failed to create personal access token"
fi
GITEA_PAT_USER="$GITEA_USER_NAME"

GITEA_SSH_KEY_FILE="$DECLAREST_WORK_DIR/gitea-ssh-key"
if [[ ! -f "$GITEA_SSH_KEY_FILE" ]]; then
    ssh-keygen -t ed25519 -N "" -f "$GITEA_SSH_KEY_FILE" >/dev/null 2>&1
fi

log_line "Registering SSH key for ${GITEA_USER_NAME}"
key_payload="$(jq -n --arg title "declarest-e2e" --arg key "$(cat "${GITEA_SSH_KEY_FILE}.pub")" '{title: $title, key: $key}')"
set +e
api_post_json "/user/keys" --data "$key_payload" >/dev/null 2>&1
set -e

GITEA_KNOWN_HOSTS_FILE="$DECLAREST_WORK_DIR/gitea-known_hosts"
for attempt in {1..20}; do
    if ssh-keyscan -p "$GITEA_SSH_PORT" localhost 2>/dev/null > "$GITEA_KNOWN_HOSTS_FILE" && [[ -s "$GITEA_KNOWN_HOSTS_FILE" ]]; then
        break
    fi
    sleep 2
done
if [[ ! -s "$GITEA_KNOWN_HOSTS_FILE" ]]; then
    die "Failed to capture Gitea SSH host key"
fi

GITEA_GIT_ASKPASS="$DECLAREST_WORK_DIR/git-askpass.sh"
cat <<'EOF' > "$GITEA_GIT_ASKPASS"
#!/usr/bin/env bash
case "$1" in
    *Username*) printf "%s" "${GIT_USERNAME:-}" ;;
    *Password*) printf "%s" "${GIT_PASSWORD:-}" ;;
    *) printf "%s" "${GIT_PASSWORD:-}" ;;
esac
EOF
chmod 700 "$GITEA_GIT_ASKPASS"

project_prefix="${GITEA_PROJECT_PREFIX}"

create_repo() {
    local name="$1"
    local repo_status
    repo_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" \
        --header "Authorization: token ${GITEA_PAT_TOKEN}" \
        "${GITEA_URL}/api/v1/repos/${GITEA_USER_NAME}/${name}" || true)"
    if [[ "$repo_status" == "200" ]]; then
        return 0
    fi

    repo_payload="$(jq -n --arg name "$name" '{name: $name, private: true}')"
    create_json="$(api_post_json "/user/repos" --data "$repo_payload")"
    repo_id="$(jq -r '.id // empty' <<<"$create_json")"
    if [[ -z "$repo_id" || "$repo_id" == "null" ]]; then
        die "Failed to create Gitea repository ${name}"
    fi
}

basic_project="${project_prefix}-basic"
pat_project="${project_prefix}-pat"
ssh_project="${project_prefix}-ssh"

create_repo "$basic_project"
create_repo "$pat_project"
create_repo "$ssh_project"

basic_http="${GITEA_URL}/${GITEA_USER_NAME}/${basic_project}.git"
pat_http="${GITEA_URL}/${GITEA_USER_NAME}/${pat_project}.git"
ssh_http="${GITEA_URL}/${GITEA_USER_NAME}/${ssh_project}.git"
ssh_url="ssh://git@localhost:${GITEA_SSH_PORT}/${GITEA_USER_NAME}/${ssh_project}.git"

seed_repo "$basic_project" "$basic_http"
seed_repo "$pat_project" "$pat_http"
seed_repo "$ssh_project" "$ssh_http"

env_file="${DECLAREST_WORK_DIR}/gitea.env"
{
    printf 'export DECLAREST_GITEA_URL=%q\n' "$GITEA_URL"
    printf 'export DECLAREST_GITEA_USER=%q\n' "$GITEA_USER_NAME"
    printf 'export DECLAREST_GITEA_PASSWORD=%q\n' "$GITEA_USER_PASS"
    printf 'export DECLAREST_GITEA_PAT=%q\n' "$GITEA_PAT_TOKEN"
    printf 'export DECLAREST_GITEA_BASIC_URL=%q\n' "$basic_http"
    printf 'export DECLAREST_GITEA_PAT_URL=%q\n' "$pat_http"
    printf 'export DECLAREST_GITEA_SSH_URL=%q\n' "$ssh_url"
    printf 'export DECLAREST_GITEA_SSH_KEY_FILE=%q\n' "$GITEA_SSH_KEY_FILE"
    printf 'export DECLAREST_GITEA_KNOWN_HOSTS_FILE=%q\n' "$GITEA_KNOWN_HOSTS_FILE"
    printf 'export DECLAREST_GITEA_GIT_ASKPASS=%q\n' "$GITEA_GIT_ASKPASS"
    printf 'export DECLAREST_GIT_ASKPASS=%q\n' "$GITEA_GIT_ASKPASS"
} > "$env_file"

log_line "Gitea projects ready: ${basic_project}, ${pat_project}, ${ssh_project}"
log_line "Gitea repo URLs: basic=${basic_http} pat=${pat_http} ssh=${ssh_url}"
log_line "Gitea env file written to ${env_file}"
