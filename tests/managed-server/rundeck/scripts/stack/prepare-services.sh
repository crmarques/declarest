#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"

require_cmd curl
require_cmd jq

wait_for_rundeck() {
    local url="$RUNDECK_BASE_URL/"
    local attempts="${RUNDECK_WAIT_ATTEMPTS:-60}"
    local delay="${RUNDECK_WAIT_DELAY:-2}"

    log_line "Waiting for Rundeck readiness at $url (${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        local code
        code="$(curl -s -o /dev/null -w "%{http_code}" "$url" || true)"
        if [[ "$code" != "000" ]]; then
            log_line "Rundeck is ready after attempt ${i}"
            return 0
        fi
        if (( i % 10 == 0 )); then
            log_line "Still waiting for Rundeck (${i}/${attempts})"
        fi
        sleep "$delay"
    done

    log_line "Rundeck did not become ready in time"
    return 1
}

wait_for_rundeck_api() {
    local url="$RUNDECK_BASE_URL/api/$RUNDECK_API_VERSION/system/info"
    local attempts="${RUNDECK_API_WAIT_ATTEMPTS:-60}"
    local delay="${RUNDECK_API_WAIT_DELAY:-2}"

    log_line "Waiting for Rundeck API readiness at $url (${attempts} attempts, ${delay}s delay)"
    for ((i=1; i<=attempts; i++)); do
        local code
        code="$(curl -s -o /dev/null -w "%{http_code}" \
            -u "$RUNDECK_USER:$RUNDECK_PASSWORD" \
            -H "Accept: application/json" \
            "$url" || true)"
        if [[ "$code" == "200" ]]; then
            log_line "Rundeck API is ready after attempt ${i}"
            return 0
        fi
        if [[ "$code" == "401" || "$code" == "403" ]]; then
            log_line "Rundeck API responded with status ${code}; proceeding to token creation"
            return 0
        fi
        if (( i % 10 == 0 )); then
            log_line "Still waiting for Rundeck API (${i}/${attempts}, last status ${code})"
        fi
        sleep "$delay"
    done

    log_line "Rundeck API did not become ready in time"
    return 1
}

if ! wait_for_rundeck; then
    die "Rundeck did not respond within timeout"
fi
if ! wait_for_rundeck_api; then
    die "Rundeck API did not become ready"
fi

if [[ -z "$RUNDECK_TOKEN" ]]; then
    log_line "Creating Rundeck API token"
    attempts="${RUNDECK_TOKEN_ATTEMPTS:-10}"
    delay="${RUNDECK_TOKEN_DELAY:-2}"
    for ((i=1; i<=attempts; i++)); do
        token_payload="$(jq -n \
            --arg user "$RUNDECK_USER" \
            --arg desc "declarest e2e ${DECLAREST_RUN_ID}" \
            '{user: $user, roles: ["admin"], duration: "0", description: $desc}')"
        response="$(curl -sS -u "$RUNDECK_USER:$RUNDECK_PASSWORD" \
            -H "Content-Type: application/json" \
            -H "Accept: application/json" \
            -X POST \
            -d "$token_payload" \
            -w "\n%{http_code}" \
            "$RUNDECK_BASE_URL/api/$RUNDECK_API_VERSION/tokens" || true)"
        token_body="${response%$'\n'*}"
        token_status="${response##*$'\n'}"
        RUNDECK_TOKEN="$(jq -r '.token // empty' <<<"$token_body" 2>/dev/null || true)"
        if [[ -z "$RUNDECK_TOKEN" && ( "$token_status" == "401" || "$token_status" == "403" ) ]]; then
            cookie_file="$DECLAREST_WORK_DIR/rundeck-cookie.jar"
            curl -sS -c "$cookie_file" "$RUNDECK_BASE_URL/user/login" >/dev/null || true
            login_response="$(curl -sS -c "$cookie_file" -b "$cookie_file" \
                -H "Content-Type: application/x-www-form-urlencoded" \
                --data-urlencode "j_username=$RUNDECK_USER" \
                --data-urlencode "j_password=$RUNDECK_PASSWORD" \
                -w "\n%{http_code}" \
                "$RUNDECK_BASE_URL/j_security_check" || true)"
            login_status="${login_response##*$'\n'}"
            case "$login_status" in
                2*|3*)
                    ;;
                *)
                    log_line "Login attempt failed (status ${login_status})"
                    login_status=""
                    ;;
            esac
            if [[ -n "$login_status" ]]; then
                response="$(curl -sS -b "$cookie_file" \
                    -H "Content-Type: application/json" \
                    -H "Accept: application/json" \
                    -X POST \
                    -d "$token_payload" \
                    -w "\n%{http_code}" \
                    "$RUNDECK_BASE_URL/api/$RUNDECK_API_VERSION/tokens" || true)"
                token_body="${response%$'\n'*}"
                token_status="${response##*$'\n'}"
                RUNDECK_TOKEN="$(jq -r '.token // empty' <<<"$token_body" 2>/dev/null || true)"
            fi
        fi
        if [[ -n "$RUNDECK_TOKEN" ]]; then
            log_line "Rundeck token created after attempt ${i}"
            break
        fi
        if [[ -n "$token_body" ]]; then
            log_line "Token request attempt ${i} failed (status ${token_status}): ${token_body}"
        else
            log_line "Token request attempt ${i} failed (status ${token_status})"
        fi
        sleep "$delay"
    done
fi

if [[ -z "$RUNDECK_TOKEN" ]]; then
    die "unable to obtain a Rundeck API token (set RUNDECK_TOKEN to skip token creation)"
fi

{
    printf 'export RUNDECK_HTTP_PORT=%q\n' "$RUNDECK_HTTP_PORT"
    printf 'export RUNDECK_BASE_URL=%q\n' "$RUNDECK_BASE_URL"
    printf 'export RUNDECK_AUTH_HEADER=%q\n' "$RUNDECK_AUTH_HEADER"
    printf 'export RUNDECK_TOKEN=%q\n' "$RUNDECK_TOKEN"
} > "$DECLAREST_RUNDECK_ENV_FILE"

log_line "Rundeck env written to $DECLAREST_RUNDECK_ENV_FILE"
