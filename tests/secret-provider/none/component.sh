#!/usr/bin/env bash

SECRET_PROVIDER_NAME="none"
SECRET_PROVIDER_PRIMARY_AUTH="none"
SECRET_PROVIDER_SECONDARY_AUTH=()

secret_provider_apply_env() {
    export DECLAREST_SECRET_STORE_TYPE="none"
    export DECLAREST_VAULT_AUTH_TYPE=""
}

secret_provider_prepare_services() {
    return 0
}

secret_provider_configure_auth() {
    export DECLAREST_VAULT_AUTH_TYPE=""
}
