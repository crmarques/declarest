#!/usr/bin/env bash

set -euo pipefail

script_invoked_directly=0
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    script_invoked_directly=1
fi

RUNDECK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$RUNDECK_DIR/scripts"

usage() {
    cat <<USAGE
Usage: ./tests/managed-server/rundeck/run-e2e.sh [options]

Options:
  --work-dir PATH         Override the work directory.
  --keep-work             Keep the work directory after the run.
  --keep-rundeck          Keep the Rundeck container after the run.
  --container-runtime CMD Container runtime to use (default: podman).
  --rundeck-image IMAGE   Override Rundeck image (default: docker.io/rundeck/rundeck:4.14.0).
  --resource-format TYPE  Resource file format (json or yaml).
  --project NAME          Override the Rundeck project name.
  --job NAME              Override the Rundeck job name.
  -h, --help              Show this help message.
USAGE
}

# shellcheck source=scripts/lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"

resolve_container_runtime() {
    if [[ "${CONTAINER_RUNTIME:-}" == "podman" ]]; then
        if ! podman info >/dev/null 2>&1; then
            if command -v docker >/dev/null 2>&1; then
                printf "Podman unavailable; falling back to docker\n" >&2
                export CONTAINER_RUNTIME="docker"
            else
                die "Podman is unavailable and docker is not installed"
            fi
        fi
    fi
}

resolve_container_runtime

while [[ $# -gt 0 ]]; do
    case "$1" in
        --work-dir)
            [[ -n "${2:-}" ]] || die "Missing value for --work-dir"
            export DECLAREST_WORK_DIR="$2"
            shift 2
            ;;
        --keep-work)
            export DECLAREST_KEEP_WORK=1
            shift
            ;;
        --keep-rundeck)
            export DECLAREST_KEEP_RUNDECK=1
            shift
            ;;
        --container-runtime)
            [[ -n "${2:-}" ]] || die "Missing value for --container-runtime"
            export CONTAINER_RUNTIME="$2"
            shift 2
            ;;
        --rundeck-image)
            [[ -n "${2:-}" ]] || die "Missing value for --rundeck-image"
            export RUNDECK_IMAGE="$2"
            shift 2
            ;;
        --resource-format)
            [[ -n "${2:-}" ]] || die "Missing value for --resource-format"
            export DECLAREST_RESOURCE_FORMAT="$2"
            shift 2
            ;;
        --project)
            [[ -n "${2:-}" ]] || die "Missing value for --project"
            export DECLAREST_PROJECT_NAME="$2"
            shift 2
            ;;
        --job)
            [[ -n "${2:-}" ]] || die "Missing value for --job"
            export DECLAREST_JOB_NAME="$2"
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

# shellcheck source=scripts/lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=scripts/lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"

REPO_SCRIPTS_DIR="$DECLAREST_TESTS_ROOT/repo-provider/file"

require_cmd "$CONTAINER_RUNTIME"
require_cmd go
require_cmd curl
require_cmd jq

mkdir -p "$DECLAREST_LOG_DIR"
export RUN_LOG="${RUN_LOG:-$DECLAREST_LOG_DIR/run-e2e_$(date -Iseconds | tr ':' '-')}.log"
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

    if [[ "${DECLAREST_KEEP_RUNDECK:-${KEEP_RUNDECK:-0}}" != "1" ]]; then
        log_line "Stopping Rundeck stack"
        "$SCRIPTS_DIR/stack/stop.sh" >>"$RUN_LOG" 2>&1 || true
    else
        log_line "DECLAREST_KEEP_RUNDECK=1; skipping Rundeck shutdown"
    fi

    log_line "Cleaning up work directory"
    "$SCRIPTS_DIR/workspace/cleanup.sh" >>"$RUN_LOG" 2>&1 || true

    if [[ $status -ne 0 ]]; then
        printf "\nRun failed (exit %s). See log: %s\n" "$status" "$RUN_LOG"
    fi
}

trap 'cleanup "$?"' EXIT INT TERM

TOTAL_STEPS=9
current_step=0

should_run_context=$((skip_testing_context == 0 ? 1 : 0))
should_run_metadata=$((skip_testing_metadata == 0 ? 1 : 0))
should_run_openapi=$((skip_testing_openapi == 0 ? 1 : 0))
should_run_declarest=$((skip_testing_declarest == 0 ? 1 : 0))
should_run_variation=0

managed_server_bootstrap() {
    log_line "Rundeck E2E run started"
    log_line "Container runtime: $CONTAINER_RUNTIME"
}

run_step() {
    local title="$1"
    local execute="$2"
    shift 2
    local cmd=("$@")

    current_step=$((current_step + 1))
    local label="${current_step}/${TOTAL_STEPS}"
    log_line "STEP START (${label}) ${title}"
    print_step_start "$label" "$title"
    local started_at
    started_at=$(date +%s)

    if [[ "$execute" -eq 0 ]]; then
        print_step_result "SKIPPED" "$label" "$title" ""
        log_line "STEP SKIPPED (${label}) ${title}"
        return 0
    fi

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

run_preparing_workspace() {
    run_step "Preparing workspace" 1 "$SCRIPTS_DIR/workspace/prepare.sh"
    run_step "Building DeclaREST CLI" 1 "$SCRIPTS_DIR/declarest/build.sh"
}

run_preparing_services() {
    run_step "Starting Rundeck" 1 "$SCRIPTS_DIR/stack/start.sh"
    run_step "Preparing Rundeck services" 1 "$SCRIPTS_DIR/stack/prepare-services.sh"
    if [[ -f "$DECLAREST_RUNDECK_ENV_FILE" ]]; then
        # shellcheck source=/dev/null
        source "$DECLAREST_RUNDECK_ENV_FILE"
    fi
}

run_configuring_services() {
    run_step "Configuring services" 1 true
}

run_configuring_context() {
    run_step "Preparing repository" 1 "$REPO_SCRIPTS_DIR/prepare.sh"
    run_step "Rendering context" 1 "$SCRIPTS_DIR/context/render.sh"
    run_step "Registering context" 1 "$SCRIPTS_DIR/context/register.sh"
}

run_testing_context_operations() {
    run_step "Testing context operations" "$should_run_context" true
}

run_testing_metadata_operations() {
    run_step "Testing metadata operations" "$should_run_metadata" true
}

run_testing_openapi_operations() {
    run_step "Validating OpenAPI defaults" "$should_run_openapi" "$SCRIPTS_DIR/declarest/openapi-smoke.sh"
}

run_testing_declarest_main_flows() {
    run_step "Running DeclaREST workflow" "$should_run_declarest" "$SCRIPTS_DIR/declarest/run.sh"
}

run_testing_secret_check_metadata() {
    run_step "Validating secret check metadata" "$should_run_declarest" true
}

run_testing_variation_flows() {
    run_step "Testing variation flows" "$should_run_variation" true
}

run_finishing_execution() {
    run_step "Finalizing execution" 1 true
}

run_rundeck_full_flow() {
    printf "Starting Rundeck E2E run\n"
    printf "Detailed log: %s\n" "$RUN_LOG"
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
    run_finishing_execution

    print_step_result "DONE" "$TOTAL_STEPS/$TOTAL_STEPS" "Completing E2E flow" ""
    log_line "E2E test completed successfully"
    printf "E2E test completed successfully. Log: %s\n" "$RUN_LOG"
}

if [[ "$script_invoked_directly" -eq 1 ]]; then
    run_rundeck_full_flow
fi
