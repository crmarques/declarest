#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

if [[ "${DECLAREST_SECRET_STORE_TYPE:-file}" == "none" ]]; then
    log_line "Secret store disabled; skipping secret check metadata smoke"
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required for secret check metadata validation"
fi

TARGET_PATH="/admin/realms/publico/user-registry/ldap-test/mappers/secret-check-mapper"
RESOURCE_DIR="${DECLAREST_REPO_DIR%/}/admin/realms/publico/user-registry/ldap-test/mappers/secret-check-mapper"
RESOURCE_FILE="$RESOURCE_DIR/resource.json"
WILDCARD_METADATA="${DECLAREST_REPO_DIR%/}/admin/realms/_/user-registry/_/metadata.json"
BACKUP_METADATA="${WILDCARD_METADATA}.bak"

backup_exists=0
cleanup() {
    local status="$?"
    rm -f "$RESOURCE_FILE"
    rmdir "$RESOURCE_DIR" >/dev/null 2>&1 || true
    if [[ $backup_exists -eq 1 && -f "$BACKUP_METADATA" ]]; then
        mv "$BACKUP_METADATA" "$WILDCARD_METADATA"
    elif [[ $backup_exists -eq 0 ]]; then
        rm -f "$WILDCARD_METADATA"
    fi
    return "$status"
}
trap cleanup EXIT

log_line "Validating secret check metadata mapping for $TARGET_PATH"

mkdir -p "$RESOURCE_DIR"
cat <<'EOF' >"$RESOURCE_FILE"
{
    "id": "secret-check-mapper",
    "name": "secret-check-mapper",
    "config": {
        "bindCredential": ["secret-value"]
    }
}
EOF

if [[ -f "$WILDCARD_METADATA" ]]; then
    mv "$WILDCARD_METADATA" "$BACKUP_METADATA"
    backup_exists=1
fi

run_cli "secret check metadata" secret check --fix --path "$TARGET_PATH"

if [[ ! -f "$WILDCARD_METADATA" ]]; then
    die "Secret check did not generate metadata at $WILDCARD_METADATA"
fi

if ! jq -e '.resourceInfo.secretInAttributes[]? | select(. == "config.bindCredential[0]")' "$WILDCARD_METADATA" >/dev/null; then
    die "Wildcard metadata missing expected secret path at $WILDCARD_METADATA"
fi

log_line "Secret check mapped secrets into wildcard metadata"
