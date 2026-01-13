#!/usr/bin/env bash

set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
export CONTAINER_RUNTIME

if [[ -z "${DECLAREST_DEBUG_GROUPS:-}" ]]; then
    export DECLAREST_DEBUG_GROUPS="network"
fi

usage() {
    cat <<EOF
Usage: ./tests/run-tests.sh [--e2e|--interactive] --managed-server <name> --repo-provider <name> --secret-provider <type> [-- <extra args>]

Options:
  --e2e                 Run the managed server's e2e workflow (default).
  --interactive         Run the managed server's interactive workflow.
  --managed-server NAME Managed server to target (keycloak or rundeck).
  --repo-provider NAME  Repository provider (file, git, gitlab, gitea, github).
  --secret-provider TYPE Secret store provider (none, file, vault).
  --complete            Run the complete e2e profile (default, exercises all variants).
  --reduced             Run the reduced e2e profile (shorthand for --e2e-profile reduced; representative variants only).
  --e2e-profile NAME    E2E profile to execute (complete, reduced).
  --skip-testing-context   Skip the "Testing context operations" group.
  --skip-testing-metadata  Skip the "Testing metadata operations" group.
  --skip-testing-openapi   Skip the "Testing OpenAPI operations" group.
  --skip-testing-declarest Skip the "Testing DeclaREST main flows" group.
  --skip-testing-variation Skip the "Testing variation flows" group.
  --keep-workspace      Preserve the test workspace after the run.
  -h, --help            Show this help message.

Examples:
  ./tests/run-tests.sh --e2e --managed-server keycloak --repo-provider gitea --secret-provider vault
  ./tests/run-tests.sh --interactive --managed-server keycloak --repo-provider git --secret-provider file

Defaults:
  mode=e2e
  managed-server=keycloak
  repo-provider=git
  secret-provider=file (rundeck defaults to none)
  e2e-profile=complete
EOF
}

mode="e2e"
managed_server="keycloak"
repo_provider="git"
secret_provider="file"
repo_provider_set=0
secret_provider_set=0
extra_args=()
profile="complete"
skip_context_flag=0
skip_metadata_flag=0
skip_openapi_flag=0
skip_declarest_flag=0
skip_variation_flag=0
set_e2e_profile() {
    local desired="${1:-}"
    if [[ -z "$desired" ]]; then
        printf "Missing value for --e2e-profile\n" >&2
        usage >&2
        exit 1
    fi
    desired="${desired,,}"
    case "$desired" in
        complete|reduced)
            profile="$desired"
            ;;
        *)
            printf "Invalid --e2e-profile: %s (expected complete or reduced)\n" "$desired" >&2
            exit 1
            ;;
    esac
}
export DECLAREST_SKIP_TESTING_CONTEXT="0"
export DECLAREST_SKIP_TESTING_METADATA="0"
export DECLAREST_SKIP_TESTING_OPENAPI="0"
export DECLAREST_SKIP_TESTING_DECLAREST="0"
export DECLAREST_SKIP_TESTING_VARIATION="0"
export KEEP_WORKSPACE="0"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --e2e)
            mode="e2e"
            shift
            ;;
        --interactive)
            mode="interactive"
            shift
            ;;
        --managed-server)
            managed_server="${2:-}"
            shift 2
            ;;
        --repo-provider)
            repo_provider="${2:-}"
            repo_provider_set=1
            shift 2
            ;;
        --complete)
            set_e2e_profile complete
            shift
            ;;
        --reduced)
            set_e2e_profile reduced
            shift
            ;;
        --e2e-profile)
            set_e2e_profile "$2"
            shift 2
            ;;
        --skip-testing-context)
            skip_context_flag=1
            export DECLAREST_SKIP_TESTING_CONTEXT="1"
            shift
            ;;
        --skip-testing-metadata)
            skip_metadata_flag=1
            export DECLAREST_SKIP_TESTING_METADATA="1"
            shift
            ;;
        --skip-testing-openapi)
            skip_openapi_flag=1
            export DECLAREST_SKIP_TESTING_OPENAPI="1"
            shift
            ;;
        --skip-testing-declarest)
            skip_declarest_flag=1
            export DECLAREST_SKIP_TESTING_DECLAREST="1"
            shift
            ;;
        --skip-testing-variation)
            skip_variation_flag=1
            export DECLAREST_SKIP_TESTING_VARIATION="1"
            shift
            ;;
        --keep-workspace)
            export KEEP_WORKSPACE="1"
            shift
            ;;
        --secret-provider)
            secret_provider="${2:-}"
            secret_provider_set=1
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --)
            shift
            extra_args+=("$@")
            break
            ;;
        *)
            printf "Unknown option: %s\n" "$1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

export DECLAREST_E2E_PROFILE="$profile"

managed_server="${managed_server,,}"
repo_provider="${repo_provider,,}"
secret_provider="${secret_provider,,}"
export DECLAREST_MANAGED_SERVER="$managed_server"
export DECLAREST_REPO_PROVIDER="$repo_provider"
export DECLAREST_SECRET_PROVIDER="$secret_provider"

check_container_runtime() {
    if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
        printf "Container runtime %s is not installed or not in PATH.\n" "$CONTAINER_RUNTIME" >&2
        printf "Install it or set CONTAINER_RUNTIME to another runtime (docker, podman).\n" >&2
        exit 1
    fi
    local runtime_err
    if ! runtime_err="$("$CONTAINER_RUNTIME" ps 2>&1 >/dev/null)"; then
        if [[ "$runtime_err" == *"alive.lck"* ]]; then
            printf "Container runtime %s cannot acquire its runtime lock:\n" "$CONTAINER_RUNTIME" >&2
            printf "%s\n" "$runtime_err" >&2
            printf "This environment restricts rootless %s; try running with a different runtime\n" "$CONTAINER_RUNTIME" >&2
            printf "or configure %s with the privileges described in %s.\n" "$CONTAINER_RUNTIME" "https://podman.io/" >&2
        else
            printf "Container runtime %s is not usable:\n" "$CONTAINER_RUNTIME" >&2
            printf "%s\n" "$runtime_err" >&2
        fi
        exit 1
    fi
}

case "$managed_server" in
    keycloak|rundeck)
        ;;
    *)
        printf "Unsupported managed server: %s\n" "$managed_server" >&2
        exit 1
        ;;
esac

if [[ "$managed_server" == "rundeck" ]]; then
    if [[ $repo_provider_set -eq 0 ]]; then
        repo_provider="file"
    fi
    if [[ $secret_provider_set -eq 0 ]]; then
        secret_provider="none"
    fi
    if [[ "$repo_provider" != "file" ]]; then
        printf "Rundeck harness supports only repo-provider file (got %s)\n" "$repo_provider" >&2
        exit 1
    fi
    if [[ "$secret_provider" != "none" ]]; then
        printf "Rundeck harness does not support secret providers (got %s)\n" "$secret_provider" >&2
        exit 1
    fi
fi

case "$repo_provider" in
    file|git|gitlab|gitea|github)
        ;;
    *)
        printf "Unsupported repo provider: %s\n" "$repo_provider" >&2
        exit 1
        ;;
esac

case "$secret_provider" in
    none|file|vault)
        ;;
    *)
        printf "Unsupported secret provider: %s\n" "$secret_provider" >&2
        exit 1
        ;;
esac

check_container_runtime

managed_dir="$TESTS_DIR/managed-server/$managed_server"
if [[ ! -d "$managed_dir" ]]; then
    printf "Managed server directory not found: %s\n" "$managed_dir" >&2
    exit 1
fi

case "$mode" in
    e2e)
        runner="$managed_dir/run-e2e.sh"
        ;;
    interactive)
        runner="$managed_dir/run-interactive.sh"
        ;;
    *)
        printf "Unsupported mode: %s\n" "$mode" >&2
        exit 1
        ;;
esac

if [[ ! -x "$runner" ]]; then
    printf "Runner script not found or not executable: %s\n" "$runner" >&2
    exit 1
fi

TOTAL_GROUPS=10
current_group_index=0
tty_output=0
if [[ -t 1 ]]; then
    tty_output=1
fi

STEP_HEADER="Step"
STATUS_HEADER="Status"
EXEC_HEADER="Execution"
DURATION_HEADER="Duration"

STATUS_VALUES=(RUNNING SKIPPED DONE FAILED)
EXEC_GROUP_TITLES=(
    "Preparing workspace"
    "Preparing services"
    "Configuring services"
    "Configuring context"
    "Testing context operations"
    "Testing metadata operations"
    "Testing OpenAPI operations"
    "Testing DeclaREST main flows"
    "Testing variation flows"
    "Finishing execution"
)
STEP_COLUMN_WIDTH=${#STEP_HEADER}
progress_sample="(${TOTAL_GROUPS}/${TOTAL_GROUPS})"
if [[ ${#progress_sample} -gt STEP_COLUMN_WIDTH ]]; then
    STEP_COLUMN_WIDTH=${#progress_sample}
fi

STATUS_COLUMN_WIDTH=${#STATUS_HEADER}
for status in "${STATUS_VALUES[@]}"; do
    if [[ ${#status} -gt STATUS_COLUMN_WIDTH ]]; then
        STATUS_COLUMN_WIDTH=${#status}
    fi
done

EXEC_COLUMN_WIDTH=${#EXEC_HEADER}
for title in "${EXEC_GROUP_TITLES[@]}"; do
    if [[ ${#title} -gt EXEC_COLUMN_WIDTH ]]; then
        EXEC_COLUMN_WIDTH=${#title}
    fi
done

DURATION_COLUMN_WIDTH=${#DURATION_HEADER}

repeat_char() {
    local char="$1"
    local count="$2"
    if [[ "$count" -le 0 ]]; then
        printf ""
        return 0
    fi
    printf '%*s' "$count" '' | tr ' ' "$char"
}

truncate_value() {
    local value="$1"
    local width="$2"
    if [[ ${#value} -le width ]]; then
        printf "%s" "$value"
    else
        printf "%s" "${value:0:width}"
    fi
}

format_status_line() {
    local step="$1"
    local status="$2"
    local execution="${3:-}"
    local duration="${4:-}"
    printf "| %-*s | %-*s | %-*s | %-*s |" \
        "$STEP_COLUMN_WIDTH" "$step" \
        "$STATUS_COLUMN_WIDTH" "$status" \
        "$EXEC_COLUMN_WIDTH" "$execution" \
        "$DURATION_COLUMN_WIDTH" "$duration"
}

TABLE_HEADER_ROW=$(format_status_line "Step" "Status" "Execution" "$DURATION_HEADER")
TABLE_ROW_WIDTH=${#TABLE_HEADER_ROW}
TABLE_BORDER_TOP="+$(repeat_char '-' $((TABLE_ROW_WIDTH - 2)))+"
TABLE_HEADER_DIVIDER="|$(repeat_char '=' $((TABLE_ROW_WIDTH - 2)))|"

print_group_status_header() {
    printf "%s\n" "$TABLE_BORDER_TOP"
    printf "%s\n" "$TABLE_HEADER_ROW"
    printf "%s\n" "$TABLE_HEADER_DIVIDER"
}

print_group_status_inline() {
    local line
    local duration
    duration=$(truncate_value "$4" "$DURATION_COLUMN_WIDTH")
    line=$(format_status_line "$1" "$2" "$3" "$duration")
    if [[ $tty_output -eq 1 ]]; then
        printf "\r\033[K%s" "$line"
    else
        printf "%s\n" "$line"
    fi
}

print_group_status_final() {
    local line
    local duration
    duration=$(truncate_value "$4" "$DURATION_COLUMN_WIDTH")
    line=$(format_status_line "$1" "$2" "$3" "$duration")
    if [[ $tty_output -eq 1 ]]; then
        printf "\r\033[K%s\n" "$line"
    else
        printf "%s\n" "$line"
    fi
}

print_table_footer() {
    printf "%s\n" "$TABLE_BORDER_TOP"
}

run_group() {
    local title="$1"
    local func="$2"
    local skip_key="${3:-}"
    local skip_message=""
    local skip_flag=0
    current_group_index=$((current_group_index + 1))
    local progress="(${current_group_index}/${TOTAL_GROUPS})"

    case "$skip_key" in
        context)
            skip_flag="$skip_context_flag"
            skip_message="flag --skip-testing-context"
            ;;
        metadata)
            skip_flag="$skip_metadata_flag"
            skip_message="flag --skip-testing-metadata"
            ;;
        openapi)
            skip_flag="$skip_openapi_flag"
            skip_message="flag --skip-testing-openapi"
            ;;
        declarest)
            skip_flag="$skip_declarest_flag"
            skip_message="flag --skip-testing-declarest"
            ;;
        variation)
            if [[ "$skip_variation_flag" -eq 1 ]]; then
                skip_flag=1
                skip_message="flag --skip-testing-variation"
            elif [[ "$should_run_variation" -eq 0 ]]; then
                skip_flag=1
                if [[ "$should_run_declarest" -eq 0 ]]; then
                    skip_message="main flows disabled"
                else
                    skip_message="variation profile disabled"
                fi
            fi
            ;;
        *)
            skip_flag=0
            ;;
    esac

    if [[ "$skip_flag" -eq 1 ]]; then
        print_group_status_final "${progress}" "SKIPPED" "$title" "$skip_message"
        log_line "GROUP SKIPPED ($title): $skip_message"
        return 0
    fi

    local started_at
    started_at=$(date +%s)
    log_line "GROUP START ($title)"
    if [[ $tty_output -eq 1 ]]; then
        print_group_status_inline "${progress}" "RUNNING" "$title" "0s"
    fi

    local status
    if [[ $tty_output -eq 1 ]]; then
        set +e
        "$func" &
        local func_pid=$!
        local elapsed=0
        while kill -0 "$func_pid" >/dev/null 2>&1; do
            sleep 1
            elapsed=$((elapsed + 1))
            print_group_status_inline "${progress}" "RUNNING" "$title" "${elapsed}s"
        done
        wait "$func_pid"
        status=$?
        set -e
    else
        set +e
        "$func"
        status=$?
        set -e
    fi

    elapsed=$(( $(date +%s) - started_at ))
    if [[ $status -eq 0 ]]; then
        print_group_status_final "${progress}" "DONE" "$title" "${elapsed}s"
        log_line "GROUP DONE ($title) (${elapsed}s)"
    else
        print_group_status_final "${progress}" "FAILED" "$title" "${elapsed}s"
        log_line "GROUP FAILED ($title) (${elapsed}s)"
        printf "See detailed log: %s\n" "$RUN_LOG"
        exit $status
    fi
}

run_keycloak_e2e_flow() {
    local previous_orchestrated="${DECLAREST_GROUP_ORCHESTRATOR:-}"
    export DECLAREST_GROUP_ORCHESTRATOR="1"

    source "$managed_dir/run-e2e.sh"

    echo "Starting Keycloak E2E run"
    printf "Detailed log: %s\n" "$RUN_LOG"
    log_line "Keycloak E2E run started"
    log_line "Container runtime: $CONTAINER_RUNTIME"
    log_line "E2E profile: $e2e_profile"

    ensure_github_pat_ssh_credentials

    print_group_status_header

    run_group "Preparing workspace" run_preparing_workspace
    run_group "Preparing services" run_preparing_services
    run_group "Configuring services" run_configuring_services
    run_group "Configuring context" run_configuring_context
    run_group "Testing context operations" run_testing_context_operations context
    run_group "Testing metadata operations" run_testing_metadata_operations metadata
    run_group "Testing OpenAPI operations" run_testing_openapi_operations openapi
    run_group "Testing DeclaREST main flows" run_testing_declarest_main_flows declarest
    run_group "Testing variation flows" run_testing_variation_flows variation
    run_group "Finishing execution" run_finishing_execution

    print_table_footer

    log_line "E2E test completed successfully"
    printf "E2E test completed successfully. Log: %s\n" "$RUN_LOG"

    if [[ -z "$previous_orchestrated" ]]; then
        unset DECLAREST_GROUP_ORCHESTRATOR
    else
        export DECLAREST_GROUP_ORCHESTRATOR="$previous_orchestrated"
    fi
}

printf "Running mode=%s, managed-server=%s, repo-provider=%s, secret-provider=%s\n" "$mode" "$managed_server" "$repo_provider" "$secret_provider"

if [[ "$mode" == "interactive" ]]; then
    exec "$runner" \
        --managed-server "$managed_server" \
        --repo-provider "$repo_provider" \
        --secret-provider "$secret_provider" \
        "${extra_args[@]}"
fi

if [[ "$mode" == "e2e" && "$managed_server" == "keycloak" ]]; then
    run_keycloak_e2e_flow
else
    exec "$runner"
fi
