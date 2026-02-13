#!/usr/bin/env bash

ARGS_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_ROOT="$(cd "$ARGS_LIB_DIR/../../../.." && pwd)"
source "$ARGS_LIB_DIR/shell.sh"
source "$TESTS_ROOT/scripts/components.sh"

require_arg() {
    local opt="$1"
    local value="${2:-}"
    if [[ -z "$value" ]]; then
        die "Missing value for ${opt}"
    fi
}

parse_common_flags() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --managed-server)
                require_arg "$1" "${2:-}"
                managed_server="${2:-}"
                shift 2
                ;;
            --repo-provider)
                require_arg "$1" "${2:-}"
                repo_provider="${2:-}"
                shift 2
                ;;
            --secret-provider)
                require_arg "$1" "${2:-}"
                secret_provider="${2:-}"
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

    managed_server="${managed_server,,}"
    case "$managed_server" in
        keycloak)
            ;;
        *)
            die "Invalid --managed-server: ${managed_server} (expected keycloak)"
            ;;
    esac

    repo_provider="${repo_provider,,}"
    if [[ -z "$repo_provider" || ! component_exists "repo-provider" "$repo_provider" ]]; then
        local available
        available="$(list_components "repo-provider" | paste -sd ", " -)"
        die "Invalid --repo-provider: ${repo_provider} (available: ${available})"
    fi

    secret_provider="${secret_provider,,}"
    if [[ -z "$secret_provider" || ! component_exists "secret-provider" "$secret_provider" ]]; then
        local available
        available="$(list_components "secret-provider" | paste -sd ", " -)"
        die "Invalid --secret-provider: ${secret_provider} (available: ${available})"
    fi
}
