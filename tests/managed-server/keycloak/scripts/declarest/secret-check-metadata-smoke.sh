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
METADATA_BASE="${DECLAREST_REPO_DIR%/}/admin/realms/_/user-registry/_"
declare -a backup_metadata_files=()
declare -a matched_metadata_files=()

cleanup() {
    local status="$?"
    rm -f "$RESOURCE_FILE"
    rmdir "$RESOURCE_DIR" >/dev/null 2>&1 || true
    for file in "${matched_metadata_files[@]}"; do
        rm -f "$file"
    done
    for file in "${backup_metadata_files[@]}"; do
        if [[ -f "${file}.bak" ]]; then
            mv "${file}.bak" "$file"
        fi
    done
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

while IFS= read -r file; do
    if [[ -f "$file" ]]; then
        mv "$file" "${file}.bak"
        backup_metadata_files+=("$file")
    fi
done < <(find "$METADATA_BASE" -name metadata.json -print)

run_cli "secret check metadata" secret check --fix --path "$TARGET_PATH"

while IFS= read -r file; do
    if jq -e '.resourceInfo.secretInAttributes[]? | select(. == "config.bindCredential[0]")' "$file" >/dev/null; then
        matched_metadata_files+=("$file")
    fi
done < <(find "$METADATA_BASE" -name metadata.json -print)

if [[ ${#matched_metadata_files[@]} -eq 0 ]]; then
    die "Secret check did not generate metadata with expected secret path under $METADATA_BASE"
fi

log_line "Secret check mapped secrets into metadata"

log_line "Secret check mapped secrets into wildcard metadata"
