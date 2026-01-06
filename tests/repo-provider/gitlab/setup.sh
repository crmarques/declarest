#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"

wait_for_gitlab() {
    local base_url="http://localhost:${GITLAB_HTTP_PORT}"
    local attempts=${GITLAB_WAIT_ATTEMPTS:-200}
    local delay=${GITLAB_WAIT_DELAY:-5}
    local readiness_status=""
    local health_status=""
    local api_status=""
    local web_status=""

    log_line "Waiting for GitLab readiness at ${base_url} (/-/readiness or /-/health, ${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        readiness_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" "${base_url}/-/readiness" || true)"
        if [[ "$readiness_status" == "200" ]]; then
            log_line "GitLab readiness endpoint is ready after attempt ${i}"
            return 0
        fi
        health_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" "${base_url}/-/health" || true)"
        if [[ "$health_status" == "200" ]]; then
            log_line "GitLab health endpoint is ready after attempt ${i}"
            return 0
        fi
        api_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" "${base_url}/api/v4/version" || true)"
        if [[ "$api_status" == "200" || "$api_status" == "401" ]]; then
            log_line "GitLab API is responding (${api_status}) after attempt ${i}"
            return 0
        fi
        web_status="$(curl -sk --max-time 5 -o /dev/null -w "%{http_code}" "${base_url}/users/sign_in" || true)"
        if (( i % 10 == 0 )); then
            log_line "Still waiting for GitLab (${i}/${attempts}; readiness=${readiness_status:-?} health=${health_status:-?} api=${api_status:-?} web=${web_status:-?})"
        fi
        sleep "$delay"
    done
    return 1
}

find_gitlab_container() {
    local container_id=""

    if [[ -n "${CONTAINER_RUNTIME:-}" && -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
        container_id="$("$CONTAINER_RUNTIME" ps -q \
            --filter "label=com.docker.compose.project=${COMPOSE_PROJECT_NAME}" \
            --filter "label=com.docker.compose.service=gitlab" 2>/dev/null | head -n 1 || true)"
    fi
    if [[ -z "$container_id" && -n "${CONTAINER_RUNTIME:-}" && -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
        container_id="$("$CONTAINER_RUNTIME" ps -q \
            --filter "name=${COMPOSE_PROJECT_NAME}_gitlab_1" 2>/dev/null | head -n 1 || true)"
    fi

    printf "%s" "$container_id"
}

get_root_token_via_session() {
    local body_file status token
    body_file="$(mktemp)"
    status="$(curl -sS -o "$body_file" -w "%{http_code}" \
        --request POST \
        --form "login=${GITLAB_ROOT_USER}" \
        --form "password=${GITLAB_ROOT_PASS}" \
        "$GITLAB_URL/api/v4/session" || true)"

    if [[ "$status" != "200" && "$status" != "201" ]]; then
        rm -f "$body_file"
        return 1
    fi
    token="$(jq -r '.private_token // empty' "$body_file")"
    rm -f "$body_file"

    if [[ -z "$token" || "$token" == "null" ]]; then
        return 1
    fi
    printf "%s" "$token"
}

get_root_token_via_rails() {
    require_cmd "$CONTAINER_RUNTIME"
    local container_id
    container_id="$(find_gitlab_container)"
    if [[ -z "$container_id" ]]; then
        log_line "GitLab container not found for token generation"
        return 1
    fi

    local ruby_script output token
    ruby_script="$(cat <<'RUBY'
require 'date'
require 'securerandom'

user = User.find_by_username('root')
if user.nil?
  puts "DECLAREST_TOKEN="
  exit 0
end

token = user.personal_access_tokens.new(
  name: "declarest-e2e-#{Time.now.to_i}",
  scopes: [:api, :read_repository, :write_repository],
  expires_at: Date.today + 30
)

raw_token = nil
if token.respond_to?(:set_token)
  arity = token.method(:set_token).arity
  if arity == 0 || arity == -1
    token.set_token
    raw_token = token.token
  else
    raw_token = SecureRandom.hex(32)
    token.set_token(raw_token)
  end
else
  raw_token = SecureRandom.hex(32)
  token.token = raw_token if token.respond_to?(:token=)
end

token.save!
raw_token ||= token.token
puts "DECLAREST_TOKEN=#{raw_token}"
RUBY
)"
    if ! output="$("$CONTAINER_RUNTIME" exec -i \
        "$container_id" /opt/gitlab/bin/gitlab-rails runner -e production "$ruby_script" 2>&1)"; then
        log_block "gitlab-rails token output (error)" "$output"
        return 1
    fi
    token="$(printf "%s" "$output" | awk -F= '/^DECLAREST_TOKEN=/{print $2}' | tail -n 1)"
    if [[ -z "$token" ]]; then
        log_block "gitlab-rails token output (no token)" "$output"
        return 1
    fi
    printf "%s" "$token"
}

api_get() {
    local endpoint="$1"
    shift
    api_request GET "$endpoint" --get "$@"
}

api_post_form() {
    local endpoint="$1"
    shift
    api_request POST "$endpoint" "$@"
}

API_LAST_STATUS=""
API_LAST_BODY=""

api_request() {
    local method="$1"
    local endpoint="$2"
    shift 2
    local response status body

    response="$(curl -sS --request "$method" \
        --header "PRIVATE-TOKEN: ${GITLAB_ROOT_TOKEN}" \
        -w $'\n%{http_code}' \
        "$GITLAB_URL/api/v4${endpoint}" "$@")"
    status="${response##*$'\n'}"
    body="${response%$'\n'*}"
    API_LAST_STATUS="$status"
    API_LAST_BODY="$body"

    if [[ "$status" -ge 200 && "$status" -lt 300 ]]; then
        printf "%s" "$body"
        return 0
    fi

    log_block "GitLab API ${method} ${endpoint} failed (status ${status})" "$body"
    return 1
}

generate_strong_password() {
    local suffix
    suffix="$(date +%s)"
    printf "KeycloakE2e!%s" "$suffix"
}

create_gitlab_user() {
    local password="$1"
    local body status

    set +e
    body="$(api_post_form "/users" \
        --form "email=${GITLAB_USER_EMAIL}" \
        --form "username=${GITLAB_USER_NAME}" \
        --form "name=${GITLAB_USER_NAME}" \
        --form "password=${password}" \
        --form "skip_confirmation=true")"
    status=$?
    set -e

    if [[ $status -ne 0 ]]; then
        printf "%s" ""
        return 1
    fi

    printf "%s" "$body"
    return 0
}

seed_repo() {
    local project="$1"
    local remote_url="$2"
    local seed_dir="$DECLAREST_WORK_DIR/gitlab-seed-${project}"

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
        export GIT_ASKPASS="$GITLAB_GIT_ASKPASS"
        export GIT_USERNAME="$GITLAB_PAT_USER"
        export GIT_PASSWORD="$GITLAB_PAT_TOKEN"
        export GIT_TERMINAL_PROMPT=0
        git_push_with_retry "$project" "$seed_dir"
    )
}

git_remote_has_branch() {
    local branch="$1"
    local remote="$2"
    local output status

    set +e
    output="$(git ls-remote "$remote" "refs/heads/$branch" 2>/dev/null)"
    status=$?
    set -e

    [[ $status -eq 0 && -n "$output" ]]
}

git_push_with_retry() {
    local project="$1"
    local seed_dir="$2"
    local remote
    remote="$(git -C "$seed_dir" remote get-url origin)"

    local attempts="${GITLAB_GIT_PUSH_ATTEMPTS:-5}"
    local delay="${GITLAB_GIT_PUSH_DELAY:-5}"
    local output status
    local retry_pattern="RPC failed; curl 52|RPC failed; HTTP 5[0-9]{2}|Empty reply from server|The requested URL returned error: 5[0-9]{2}|Connection reset by peer|failed to connect to|EOF"

    for ((attempt=1; attempt<=attempts; attempt++)); do
        set +e
        output="$(capture_logged "seed ${project} (push attempt ${attempt}/${attempts})" \
            git -C "$seed_dir" push -u origin main)"
        status=$?
        set -e

        if [[ $status -eq 0 ]]; then
            return 0
        fi

        if git_remote_has_branch "main" "$remote"; then
            log_line "Remote already has main for ${project}; treating push as successful"
            return 0
        fi

        if grep -Eq "RPC failed; HTTP 5[0-9]{2}|The requested URL returned error: 5[0-9]{2}" <<<"$output"; then
            log_line "GitLab HTTP error during push; waiting for readiness before retry"
            wait_for_gitlab || true
        fi

        if grep -Eq "$retry_pattern" <<<"$output"; then
            log_line "Transient git push failure for ${project}; retrying after ${delay}s"
            sleep "$delay"
            continue
        fi

        return $status
    done

    return 1
}

require_cmd curl
require_cmd jq
require_cmd git
require_cmd ssh-keygen
require_cmd ssh-keyscan

if [[ ! -d "$DECLAREST_TEMPLATE_REPO_DIR" ]]; then
    die "Template repository not found at $DECLAREST_TEMPLATE_REPO_DIR"
fi

GITLAB_URL="http://localhost:${GITLAB_HTTP_PORT}"
GITLAB_ROOT_USER="root"
GITLAB_ROOT_PASS="${GITLAB_ROOT_PASSWORD}"
GITLAB_ROOT_EMAIL="${GITLAB_ROOT_EMAIL:-root@example.com}"
GITLAB_USER_NAME="${GITLAB_USER}"
GITLAB_USER_PASS="${GITLAB_USER_PASSWORD}"
GITLAB_USER_EMAIL="${GITLAB_USER_EMAIL}"

wait_for_gitlab || die "GitLab did not become ready in time"

log_line "Authenticating as GitLab root user"
GITLAB_ROOT_TOKEN="$(get_root_token_via_session || true)"
if [[ -z "$GITLAB_ROOT_TOKEN" || "$GITLAB_ROOT_TOKEN" == "null" ]]; then
    token_attempts=${GITLAB_TOKEN_ATTEMPTS:-20}
    token_delay=${GITLAB_TOKEN_DELAY:-5}
    log_line "Session API unavailable; generating root token via gitlab-rails (${token_attempts} attempts, ${token_delay}s delay)"
    for ((attempt=1; attempt<=token_attempts; attempt++)); do
        GITLAB_ROOT_TOKEN="$(get_root_token_via_rails || true)"
        if [[ -n "$GITLAB_ROOT_TOKEN" && "$GITLAB_ROOT_TOKEN" != "null" ]]; then
            break
        fi
        log_line "GitLab root token not available yet (attempt ${attempt}/${token_attempts})"
        sleep "$token_delay"
    done
fi
if [[ -z "$GITLAB_ROOT_TOKEN" || "$GITLAB_ROOT_TOKEN" == "null" ]]; then
    die "Failed to obtain GitLab root token"
fi

log_line "Ensuring GitLab user ${GITLAB_USER_NAME}"
set +e
user_json="$(api_get "/users" --data-urlencode "username=${GITLAB_USER_NAME}")"
status=$?
set -e
if [[ $status -ne 0 ]]; then
    log_line "GitLab user lookup by username failed; retrying with search"
    set +e
    user_json="$(api_get "/users" --data-urlencode "search=${GITLAB_USER_NAME}")"
    status=$?
    set -e
    if [[ $status -ne 0 ]]; then
        die "Failed to query GitLab users"
    fi
fi
GITLAB_USER_ID="$(jq -r '.[0].id // empty' <<<"$user_json")"
if [[ -z "$GITLAB_USER_ID" ]]; then
    create_json="$(create_gitlab_user "$GITLAB_USER_PASS" || true)"
    if [[ -z "$create_json" ]]; then
        if grep -qi "password" <<<"${API_LAST_BODY:-}"; then
            log_line "GitLab user password rejected; retrying with generated password"
            GITLAB_USER_PASS="$(generate_strong_password)"
            create_json="$(create_gitlab_user "$GITLAB_USER_PASS")"
        else
            die "Failed to create GitLab user ${GITLAB_USER_NAME}"
        fi
    fi
    GITLAB_USER_ID="$(jq -r '.id // empty' <<<"$create_json")"
fi
if [[ -z "$GITLAB_USER_ID" || "$GITLAB_USER_ID" == "null" ]]; then
    die "Failed to create GitLab user ${GITLAB_USER_NAME}"
fi

namespace_json="$(api_get "/namespaces" --data-urlencode "search=${GITLAB_USER_NAME}")"
GITLAB_NAMESPACE_ID="$(jq -r ".[] | select(.kind==\"user\" and .path==\"${GITLAB_USER_NAME}\") | .id" <<<"$namespace_json" | head -n 1)"
if [[ -z "$GITLAB_NAMESPACE_ID" || "$GITLAB_NAMESPACE_ID" == "null" ]]; then
    die "Failed to resolve namespace for ${GITLAB_USER_NAME}"
fi

log_line "Creating personal access token for ${GITLAB_USER_NAME}"
token_json="$(api_post_form "/users/${GITLAB_USER_ID}/personal_access_tokens" \
    --form "name=declarest-e2e-${DECLAREST_RUN_ID}" \
    --form "scopes[]=api" \
    --form "scopes[]=read_repository" \
    --form "scopes[]=write_repository")"
GITLAB_PAT_TOKEN="$(jq -r '.token // empty' <<<"$token_json")"
if [[ -z "$GITLAB_PAT_TOKEN" || "$GITLAB_PAT_TOKEN" == "null" ]]; then
    die "Failed to create personal access token"
fi
GITLAB_PAT_USER="oauth2"

GITLAB_SSH_KEY_FILE="$DECLAREST_WORK_DIR/gitlab-ssh-key"
if [[ ! -f "$GITLAB_SSH_KEY_FILE" ]]; then
    ssh-keygen -t ed25519 -N "" -f "$GITLAB_SSH_KEY_FILE" >/dev/null 2>&1
fi

log_line "Registering SSH key for ${GITLAB_USER_NAME}"
set +e
api_post_form "/users/${GITLAB_USER_ID}/keys" \
    --form "title=declarest-e2e" \
    --form "key=$(cat "${GITLAB_SSH_KEY_FILE}.pub")" >/dev/null 2>&1
set -e

GITLAB_KNOWN_HOSTS_FILE="$DECLAREST_WORK_DIR/gitlab-known_hosts"
for attempt in {1..20}; do
    if ssh-keyscan -p "$GITLAB_SSH_PORT" localhost 2>/dev/null > "$GITLAB_KNOWN_HOSTS_FILE" && [[ -s "$GITLAB_KNOWN_HOSTS_FILE" ]]; then
        break
    fi
    sleep 2
done
if [[ ! -s "$GITLAB_KNOWN_HOSTS_FILE" ]]; then
    die "Failed to capture GitLab SSH host key"
fi

GITLAB_GIT_ASKPASS="$DECLAREST_WORK_DIR/git-askpass.sh"
cat <<'EOF' > "$GITLAB_GIT_ASKPASS"
#!/usr/bin/env bash
case "$1" in
    *Username*) printf "%s" "${GIT_USERNAME:-}" ;;
    *Password*) printf "%s" "${GIT_PASSWORD:-}" ;;
    *) printf "%s" "${GIT_PASSWORD:-}" ;;
esac
EOF
chmod 700 "$GITLAB_GIT_ASKPASS"

project_prefix="${GITLAB_PROJECT_PREFIX}"

create_project() {
    local name="$1"
    local project_json project_id
    project_json="$(api_get "/projects" --data-urlencode "search=${name}")"
    project_id="$(jq -r ".[] | select(.path==\"${name}\" and .namespace.path==\"${GITLAB_USER_NAME}\") | .id" <<<"$project_json" | head -n 1)"
    if [[ -n "$project_id" ]]; then
        printf "%s" "$project_id"
        return 0
    fi
    project_json="$(api_post_form "/projects" \
        --form "name=${name}" \
        --form "namespace_id=${GITLAB_NAMESPACE_ID}" \
        --form "visibility=private")"
    project_id="$(jq -r '.id // empty' <<<"$project_json")"
    if [[ -z "$project_id" || "$project_id" == "null" ]]; then
        die "Failed to create GitLab project ${name}"
    fi
    printf "%s" "$project_id"
}

basic_project="${project_prefix}-basic"
pat_project="${project_prefix}-pat"
ssh_project="${project_prefix}-ssh"

create_project "$basic_project" >/dev/null
create_project "$pat_project" >/dev/null
create_project "$ssh_project" >/dev/null

basic_http="${GITLAB_URL}/${GITLAB_USER_NAME}/${basic_project}.git"
pat_http="${GITLAB_URL}/${GITLAB_USER_NAME}/${pat_project}.git"
ssh_http="${GITLAB_URL}/${GITLAB_USER_NAME}/${ssh_project}.git"
ssh_url="ssh://git@localhost:${GITLAB_SSH_PORT}/${GITLAB_USER_NAME}/${ssh_project}.git"

seed_repo "$basic_project" "$basic_http"
seed_repo "$pat_project" "$pat_http"
seed_repo "$ssh_project" "$ssh_http"

env_file="${DECLAREST_WORK_DIR}/gitlab.env"
{
    printf 'export DECLAREST_GITLAB_URL=%q\n' "$GITLAB_URL"
    printf 'export DECLAREST_GITLAB_USER=%q\n' "$GITLAB_USER_NAME"
    printf 'export DECLAREST_GITLAB_PASSWORD=%q\n' "$GITLAB_USER_PASS"
    printf 'export DECLAREST_GITLAB_PAT=%q\n' "$GITLAB_PAT_TOKEN"
    printf 'export DECLAREST_GITLAB_BASIC_URL=%q\n' "$basic_http"
    printf 'export DECLAREST_GITLAB_PAT_URL=%q\n' "$pat_http"
    printf 'export DECLAREST_GITLAB_SSH_URL=%q\n' "$ssh_url"
    printf 'export DECLAREST_GITLAB_SSH_KEY_FILE=%q\n' "$GITLAB_SSH_KEY_FILE"
    printf 'export DECLAREST_GITLAB_KNOWN_HOSTS_FILE=%q\n' "$GITLAB_KNOWN_HOSTS_FILE"
    printf 'export DECLAREST_GITLAB_GIT_ASKPASS=%q\n' "$GITLAB_GIT_ASKPASS"
} > "$env_file"

log_line "GitLab projects ready: ${basic_project}, ${pat_project}, ${ssh_project}"
log_line "GitLab repo URLs: basic=${basic_http} pat=${pat_http} ssh=${ssh_url}"
log_line "GitLab env file written to ${env_file}"
