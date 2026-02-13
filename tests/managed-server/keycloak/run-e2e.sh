#!/usr/bin/env bash

set -euo pipefail

script_invoked_directly=0
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    script_invoked_directly=1
fi

KEYCLOAK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$KEYCLOAK_DIR/scripts"

source "$SCRIPTS_DIR/lib/args.sh"

managed_server="${DECLAREST_MANAGED_SERVER:-keycloak}"
repo_provider="${DECLAREST_REPO_PROVIDER:-git}"
secret_provider="${DECLAREST_SECRET_PROVIDER:-file}"

e2e_profile="${DECLAREST_E2E_PROFILE:-complete}"
skip_testing_context="${DECLAREST_SKIP_TESTING_CONTEXT:-0}"
skip_testing_metadata="${DECLAREST_SKIP_TESTING_METADATA:-0}"
skip_testing_openapi="${DECLAREST_SKIP_TESTING_OPENAPI:-0}"
skip_testing_declarest="${DECLAREST_SKIP_TESTING_DECLAREST:-0}"
skip_testing_variation="${DECLAREST_SKIP_TESTING_VARIATION:-0}"
parse_common_flags
if [[ "$e2e_profile" == "reduced" ]]; then
    skip_testing_metadata=1
    skip_testing_openapi=1
    skip_testing_variation=1
fi

load_repo_provider_component "$repo_provider"
load_secret_provider_component "$secret_provider"
repo_provider_apply_env
secret_provider_apply_env

repo_type="${REPO_PROVIDER_TYPE}"

should_run_context=$((skip_testing_context == 0 ? 1 : 0))
should_run_metadata=$((skip_testing_metadata == 0 ? 1 : 0))
should_run_openapi=$((skip_testing_openapi == 0 ? 1 : 0))
should_run_declarest=$((skip_testing_declarest == 0 ? 1 : 0))
if [[ "$skip_testing_variation" -eq 0 && "$should_run_declarest" -eq 1 ]]; then
    should_run_variation=1
else
    should_run_variation=0
fi

source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

REPO_SCRIPTS_DIR="$DECLAREST_TESTS_ROOT/repo-provider/common"

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
    if [[ "${DECLAREST_GROUP_ORCHESTRATOR:-0}" == "1" ]]; then
        return 0
    fi
    local label="$1"
    local title="$2"

    if is_tty; then
        printf "\r[RUN ] %s | %s..." "$label" "$title"
    else
        printf "[RUN ] %s | %s...\n" "$label" "$title"
    fi
}

print_step_result() {
    if [[ "${DECLAREST_GROUP_ORCHESTRATOR:-0}" == "1" ]]; then
        return 0
    fi
    local state="$1"
    local label="$2"
    local display_title="$3"
    local duration="$4"

    if is_tty; then
        printf "\r\033[K"
    fi
    printf "[%s] %s | %s" "$state" "$label" "$display_title"
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

    if [[ "${KEEP_WORKSPACE:-0}" == "1" ]]; then
        log_line "KEEP_WORKSPACE=1; skipping workspace cleanup"
    else
        log_line "Cleaning up work directory"
        "$SCRIPTS_DIR/workspace/cleanup.sh" >>"$RUN_LOG" 2>&1 || true
    fi

    if [[ $status -ne 0 ]]; then
        printf "\nRun failed (exit %s). See log: %s\n" "$status" "$RUN_LOG"
    fi
}

trap 'cleanup "$?"' EXIT INT TERM

configure_repo_auth() {
    local auth="$1"
    if [[ "$repo_type" != "git-remote" ]]; then
        export DECLAREST_REPO_REMOTE_URL=""
        return 0
    fi
    repo_provider_configure_auth "$auth"
}

configure_secret_auth() {
    local auth="$1"
    secret_provider_configure_auth "$auth"
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


server_auth_primary="oauth2"
server_auth_secondary=(bearer)
secret_auth_primary="$SECRET_PROVIDER_PRIMARY_AUTH"
secret_auth_secondary=("${SECRET_PROVIDER_SECONDARY_AUTH[@]}")

repo_auth_primary="$REPO_PROVIDER_PRIMARY_AUTH"
repo_auth_secondary=("${REPO_PROVIDER_SECONDARY_AUTH[@]}")

calculate_variant_count() {
    local array_name="$1"
    local length=0
    eval "length=\${#${array_name}[@]}"
    if [[ "$length" -eq 0 ]]; then
        printf "0"
        return
    fi
    if [[ "$e2e_profile" == "complete" ]]; then
        printf "%s" "$length"
    else
        printf "1"
    fi
}

calculate_total_steps() {
    local total=0

    total=$((total + 1))
    if [[ "$secret_provider" == "none" ]]; then
        total=$((total + 1))
    fi
    total=$((total + 1))

    total=$((total + 2))

    total=$((total + 2))
    if [[ "$repo_type" == "git-remote" ]]; then
        total=$((total + 1))
    fi

    total=$((total + 3))

    total=$((total + 1))

    total=$((total + 6))

    total=$((total + 1))

    if [[ "$secret_provider" == "none" && "$repo_type" == "git-remote" ]]; then
        total=$((total + 1))
    fi
    total=$((total + 1))
    total=$((total + 1))

    if [[ "$secret_provider" != "none" ]]; then
        total=$((total + 1))
    fi

    total=$((total + server_variation_count + secret_variation_count + repo_variation_count + 1))

    TOTAL_STEPS="$total"
}

server_variation_count=$(calculate_variant_count server_auth_secondary)
secret_variation_count=$(calculate_variant_count secret_auth_secondary)
repo_variation_count=$(calculate_variant_count repo_auth_secondary)

calculate_total_steps
STEP_NUM_WIDTH=${#TOTAL_STEPS}
current_group=""

format_step_label() {
    local step="$1"
    printf "%*d/%s" "$STEP_NUM_WIDTH" "$step" "$TOTAL_STEPS"
}

format_group_title() {
    local title="$1"
    if [[ -n "$current_group" ]]; then
        printf "%s | %s" "$current_group" "$title"
    else
        printf "%s" "$title"
    fi
}

run_step() {
    local title="$1"
    local execute="$2"
    shift 2
    local cmd=("$@")

    current_step=$((current_step + 1))
    local label
    label=$(format_step_label "$current_step")
    local display_title
    display_title=$(format_group_title "$title")

    if [[ "$execute" -eq 0 ]]; then
        print_step_result "SKIPPED" "$label" "$display_title" ""
        log_line "STEP SKIPPED (${label}) ${display_title}"
        return 0
    fi

    log_line "STEP START (${label}) ${display_title}"
    print_step_start "$label" "$display_title"
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
        print_step_result "DONE" "$label" "$display_title" "$elapsed"
        log_line "STEP DONE (${label}) ${display_title} (${elapsed}s)"
        return 0
    fi

    print_step_result "FAIL" "$label" "$display_title" "$elapsed"
    log_line "STEP FAILED (${label}) ${display_title} (exit ${status}, ${elapsed}s)"
    if [[ "${DECLAREST_GROUP_ORCHESTRATOR:-0}" == "1" ]]; then
        return $status
    fi
    printf "See detailed log: %s\n" "$RUN_LOG"
    exit $status
}

run_preparing_workspace() {
    current_group="Preparing workspace"
    run_step "Preparing workspace" 1 "$SCRIPTS_DIR/workspace/prepare.sh"
    if [[ "$secret_provider" == "none" ]]; then
        template_dest="$DECLAREST_WORK_DIR/templates/repo-no-secrets"
        run_step "Preparing template (no secrets)" 1 "$REPO_SCRIPTS_DIR/strip-secrets.sh" "$DECLAREST_TEST_DIR/templates/repo" "$template_dest"
        export DECLAREST_TEMPLATE_REPO_DIR="$template_dest"
    fi
    run_step "Building declarest CLI" 1 "$SCRIPTS_DIR/declarest/build.sh"
    current_group=""
}

prepare_services() {
    "$SCRIPTS_DIR/stack/prepare-services-e2e.sh"
    repo_provider_prepare_services
}

run_preparing_services() {
    current_group="Preparing services"
    run_step "Starting stack" 1 "$SCRIPTS_DIR/stack/start-compose.sh"
    run_step "Preparing services" 1 prepare_services
    current_group=""
}

run_configuring_services() {
    current_group="Configuring services"
    run_step "Configuring server auth (primary)" 1 configure_server_auth "$server_auth_primary"
    run_step "Configuring secret auth (primary)" 1 configure_secret_auth "$secret_auth_primary"
    if [[ "$repo_type" == "git-remote" ]]; then
        run_step "Configuring repo auth (primary)" 1 configure_repo_auth "$repo_auth_primary"
    fi
    current_group=""
    set_context "primary"
}

run_configuring_context() {
    current_group="Configuring context"
    run_step "Preparing repo (primary)" 1 "$REPO_SCRIPTS_DIR/prepare.sh"
    run_step "Configuring declarest context (primary)" 1 "$SCRIPTS_DIR/context/render.sh"
    run_step "Registering declarest context (primary)" 1 "$SCRIPTS_DIR/context/register.sh"
    current_group=""
}

run_testing_context_operations() {
    current_group="Testing context operations"
    set_context "primary"
    run_step "Validating context overrides" "$should_run_context" "$SCRIPTS_DIR/declarest/env-context-smoke.sh"
    current_group=""
}

run_testing_metadata_operations() {
    current_group="Testing metadata operations"
    set_context "primary"
    run_step "Validating metadata inference (primary)" "$should_run_metadata" "$SCRIPTS_DIR/declarest/metadata-infer-smoke.sh"
    run_step "Validating metadata edit (primary)" "$should_run_metadata" "$SCRIPTS_DIR/declarest/metadata-edit-smoke.sh"
    run_step "Validating metadata inheritance (primary)" "$should_run_metadata" "$SCRIPTS_DIR/declarest/metadata-inheritance-smoke.sh"

    if [[ "$should_run_metadata" -eq 1 ]]; then
        set_context "metadata-base-dir"
        export DECLAREST_METADATA_DIR="$DECLAREST_WORK_DIR/metadata-base-dir"
    fi
    run_step "Configuring declarest context (metadata base dir)" "$should_run_metadata" "$SCRIPTS_DIR/context/render.sh"
    run_step "Registering declarest context (metadata base dir)" "$should_run_metadata" "$SCRIPTS_DIR/context/register.sh"
    run_step "Validating metadata base dir override" "$should_run_metadata" "$SCRIPTS_DIR/declarest/metadata-base-dir-smoke.sh"
    if [[ "$should_run_metadata" -eq 1 ]]; then
        unset DECLAREST_METADATA_DIR
    fi
    set_context "primary"
    if [[ "$should_run_metadata" -eq 1 ]]; then
        run_cli "restoring context (primary)" config set-current-context --name "keycloak-e2e"
    fi
    current_group=""
}

run_testing_openapi_operations() {
    current_group="Testing OpenAPI operations"
    set_context "primary"
    run_step "Validating OpenAPI defaults (primary)" "$should_run_openapi" "$SCRIPTS_DIR/declarest/openapi-smoke.sh"
    current_group=""
}

run_testing_declarest_main_flows() {
    current_group="Testing DeclaREST main flows"
    set_context "primary"
    if [[ "$secret_provider" == "none" && "$repo_type" == "git-remote" ]]; then
        run_step "Sanitizing repository (primary)" "$should_run_declarest" "$REPO_SCRIPTS_DIR/strip-secrets.sh" "$DECLAREST_REPO_DIR"
    fi
    run_step "Running declarest workflow (primary)" "$should_run_declarest" "$SCRIPTS_DIR/declarest/run.sh"
    if [[ "$repo_type" == "git-remote" ]]; then
        run_step "Verifying remote repo (primary)" "$should_run_declarest" "$REPO_SCRIPTS_DIR/verify.sh"
    elif [[ "$repo_type" == "git-local" ]]; then
        run_step "Printing git log (primary)" "$should_run_declarest" "$REPO_SCRIPTS_DIR/print-log.sh"
    fi
    current_group=""
}

run_testing_secret_check_metadata() {
    current_group="Testing secret check metadata"
    set_context "primary"
    local should_run="$should_run_declarest"
    if [[ "$secret_provider" == "none" ]]; then
        should_run=0
    fi
    run_step "Validating secret check metadata mapping" "$should_run" "$SCRIPTS_DIR/declarest/secret-check-metadata-smoke.sh"
    current_group=""
}

run_testing_variation_flows() {
    current_group="Testing variation flows"
    if [[ "$should_run_variation" -eq 0 ]]; then
        current_group=""
        return 0
    fi
    set_context "primary"

    configure_server_auth "$server_auth_primary"
    configure_secret_auth "$secret_auth_primary"
    if [[ "$repo_type" == "git-remote" ]]; then
        configure_repo_auth "$repo_auth_primary"
    fi

    local server_variants=()
    if [[ ${#server_auth_secondary[@]} -gt 0 ]]; then
        if [[ "$e2e_profile" == "complete" ]]; then
            server_variants=("${server_auth_secondary[@]}")
        else
            server_variants=("${server_auth_secondary[0]}")
        fi
    fi
    for server_auth in "${server_variants[@]}"; do
        configure_server_auth "$server_auth"
        configure_secret_auth "$secret_auth_primary"
        if [[ "$repo_type" == "git-remote" ]]; then
            configure_repo_auth "$repo_auth_primary"
        fi
        set_context "server-${server_auth}"
        run_step "Validating server auth (${server_auth})" "$should_run_variation" "$SCRIPTS_DIR/declarest/server-auth-smoke.sh"
    done

    configure_server_auth "$server_auth_primary"
    configure_secret_auth "$secret_auth_primary"
    if [[ "$repo_type" == "git-remote" ]]; then
        configure_repo_auth "$repo_auth_primary"
    fi
    set_context "primary"

    if [[ "$secret_provider" == "vault" && ${#secret_auth_secondary[@]} -gt 0 ]]; then
        local secret_variants=()
        if [[ "$e2e_profile" == "complete" ]]; then
            secret_variants=("${secret_auth_secondary[@]}")
        else
            secret_variants=("${secret_auth_secondary[0]}")
        fi
        for vault_auth in "${secret_variants[@]}"; do
            configure_server_auth "$server_auth_primary"
            configure_secret_auth "$vault_auth"
            if [[ "$repo_type" == "git-remote" ]]; then
                configure_repo_auth "$repo_auth_primary"
            fi
            set_context "vault-${vault_auth}"
            run_step "Validating vault auth (${vault_auth})" "$should_run_variation" "$SCRIPTS_DIR/declarest/secret-auth-smoke.sh"
        done
        configure_secret_auth "$secret_auth_primary"
        configure_server_auth "$server_auth_primary"
        if [[ "$repo_type" == "git-remote" ]]; then
            configure_repo_auth "$repo_auth_primary"
        fi
        set_context "primary"
    fi

    if [[ "$repo_type" == "git-remote" && ${#repo_auth_secondary[@]} -gt 0 ]]; then
        local repo_variants=()
        if [[ "$e2e_profile" == "complete" ]]; then
            repo_variants=("${repo_auth_secondary[@]}")
        else
            repo_variants=("${repo_auth_secondary[0]}")
        fi
        for repo_auth in "${repo_variants[@]}"; do
            configure_repo_auth "$repo_auth"
            run_step "Validating repo auth (${repo_auth})" "$should_run_variation" "$REPO_SCRIPTS_DIR/auth-smoke.sh"
        done
        configure_repo_auth "$repo_auth_primary"
    fi

    set_context "primary"
    run_step "Validating managed server TLS" "$should_run_variation" "$SCRIPTS_DIR/declarest/managed-server-tls-smoke.sh"
    set_context ""
    current_group=""
}

run_finishing_execution() {
    current_group="Finishing execution"
    run_step "Finalizing execution" 1 true
    current_group=""
}

current_step=0

managed_server_bootstrap() {
    log_line "Keycloak E2E run started"
    log_line "Container runtime: $CONTAINER_RUNTIME"
    log_line "E2E profile: $e2e_profile"
}

run_keycloak_full_flow() {
    if [[ "$script_invoked_directly" -eq 1 ]]; then
        echo "Starting Keycloak E2E run"
        echo "Detailed log: $RUN_LOG"
    fi

    managed_server_bootstrap

    run_preparing_workspace
    run_preparing_services
    run_configuring_services
    run_configuring_context

    run_testing_context_operations
    run_testing_metadata_operations
    run_testing_openapi_operations
    run_testing_declarest_main_flows
    run_testing_secret_check_metadata
    run_testing_variation_flows

    if [[ "$script_invoked_directly" -eq 1 ]]; then
        print_step_result "DONE" "$TOTAL_STEPS/$TOTAL_STEPS" "Completing E2E flow" ""
        log_line "E2E test completed successfully"
        echo "E2E test completed successfully. Log: $RUN_LOG"
    fi
}

if [[ "$script_invoked_directly" -eq 1 ]]; then
    run_keycloak_full_flow
fi
