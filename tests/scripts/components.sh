#!/usr/bin/env bash

components_root() {
    if [[ -n "${DECLAREST_TESTS_ROOT:-}" ]]; then
        printf "%s" "$DECLAREST_TESTS_ROOT"
        return 0
    fi
    local base
    base="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
    printf "%s" "$base"
}

component_script() {
    local group="$1"
    local name="$2"
    local root
    root="$(components_root)"
    printf "%s/%s/%s/component.sh" "$root" "$group" "$name"
}

component_exists() {
    local group="$1"
    local name="$2"
    [[ -f "$(component_script "$group" "$name")" ]]
}

list_components() {
    local group="$1"
    local root
    root="$(components_root)"
    if [[ ! -d "$root/$group" ]]; then
        return 0
    fi
    find "$root/$group" -mindepth 2 -maxdepth 2 -type f -name component.sh -print | while read -r path; do
        basename "$(dirname "$path")"
    done | sort -u
}

component_fail() {
    printf "%s\n" "$1" >&2
    return 1
}

require_component_var() {
    local var="$1"
    if [[ -z "${!var:-}" ]]; then
        component_fail "Component missing ${var}"
        return 1
    fi
}

require_component_func() {
    local func="$1"
    if ! declare -F "$func" >/dev/null 2>&1; then
        component_fail "Component missing function ${func}"
        return 1
    fi
}

load_managed_server_component() {
    local name="$1"
    local script
    script="$(component_script "managed-server" "$name")"
    if [[ ! -f "$script" ]]; then
        component_fail "Unknown managed server: ${name}"
        return 1
    fi
    source "$script"
    MANAGED_SERVER_COMPONENT_DIR="$(cd "$(dirname "$script")" && pwd)"
    require_component_var "MANAGED_SERVER_NAME" || return 1
    require_component_func "managed_server_default_repo_provider" || return 1
    require_component_func "managed_server_default_secret_provider" || return 1
    require_component_func "managed_server_validate" || return 1
    require_component_func "managed_server_runner" || return 1
}

load_repo_provider_component() {
    local name="$1"
    local script
    script="$(component_script "repo-provider" "$name")"
    if [[ ! -f "$script" ]]; then
        component_fail "Unknown repo provider: ${name}"
        return 1
    fi
    source "$script"
    REPO_PROVIDER_COMPONENT_DIR="$(cd "$(dirname "$script")" && pwd)"
    require_component_var "REPO_PROVIDER_NAME" || return 1
    require_component_var "REPO_PROVIDER_TYPE" || return 1
    require_component_var "REPO_PROVIDER_PRIMARY_AUTH" || return 1
    require_component_var "REPO_PROVIDER_REMOTE_PROVIDER" || return 1
    require_component_var "REPO_PROVIDER_INTERACTIVE_AUTH" || return 1
    require_component_func "repo_provider_apply_env" || return 1
    require_component_func "repo_provider_prepare_services" || return 1
    require_component_func "repo_provider_prepare_interactive" || return 1
    require_component_func "repo_provider_configure_auth" || return 1
}

load_secret_provider_component() {
    local name="$1"
    local script
    script="$(component_script "secret-provider" "$name")"
    if [[ ! -f "$script" ]]; then
        component_fail "Unknown secret provider: ${name}"
        return 1
    fi
    source "$script"
    SECRET_PROVIDER_COMPONENT_DIR="$(cd "$(dirname "$script")" && pwd)"
    require_component_var "SECRET_PROVIDER_NAME" || return 1
    require_component_var "SECRET_PROVIDER_PRIMARY_AUTH" || return 1
    require_component_func "secret_provider_apply_env" || return 1
    require_component_func "secret_provider_prepare_services" || return 1
    require_component_func "secret_provider_configure_auth" || return 1
}
