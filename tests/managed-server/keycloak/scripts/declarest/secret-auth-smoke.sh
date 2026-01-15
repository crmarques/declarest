#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

if [[ -z "${DECLAREST_CONTEXT_FILE:-}" ]]; then
    die "Context file is not configured"
fi

secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-}"
secret_store_type="${secret_store_type,,}"
if [[ "$secret_store_type" == "none" ]]; then
    die "Secret store is disabled; cannot run secret auth smoke"
fi

"$SCRIPTS_DIR/context/render.sh"
"$SCRIPTS_DIR/context/register.sh"

SECRET_CLIENT_PATH="/admin/realms/publico/clients/testB"
SECRET_LDAP_PATH="/admin/realms/publico/user-registry/ldap-test"
SECRET_CLIENT_KEY="secret"
SECRET_LDAP_KEY="config.bindCredential[0]"

paths_output="$(capture_cli "secret list" --no-status secret list --paths-only)"
for path in "$SECRET_CLIENT_PATH" "$SECRET_LDAP_PATH"; do
    if ! grep -Fq "$path" <<<"$paths_output"; then
        die "Secret path missing from list: $path"
    fi
done

client_secret="$(capture_cli "secret get client" --no-status secret get --path "$SECRET_CLIENT_PATH" --key "$SECRET_CLIENT_KEY")"
if [[ -z "$client_secret" ]]; then
    die "Secret get returned empty value for $SECRET_CLIENT_PATH"
fi

ldap_secret="$(capture_cli "secret get ldap" --no-status secret get --path "$SECRET_LDAP_PATH" --key "$SECRET_LDAP_KEY")"
if [[ -z "$ldap_secret" ]]; then
    die "Secret get returned empty value for $SECRET_LDAP_PATH"
fi

log_line "Secret auth smoke completed"
