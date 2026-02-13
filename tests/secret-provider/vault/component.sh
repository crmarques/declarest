#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

SECRET_PROVIDER_NAME="vault"
SECRET_PROVIDER_PRIMARY_AUTH="token"
SECRET_PROVIDER_SECONDARY_AUTH=(password approle)

secret_provider_apply_env() {
    export DECLAREST_SECRET_STORE_TYPE="vault"
}

secret_provider_prepare_services() {
    "$SCRIPT_DIR/setup.sh"
}

secret_provider_configure_auth() {
    local auth="$1"
    export DECLAREST_VAULT_AUTH_TYPE="$auth"
}
