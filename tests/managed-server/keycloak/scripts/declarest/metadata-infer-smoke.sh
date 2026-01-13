#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required for metadata inference validation"
fi

log_line "Validating metadata inference using Keycloak OpenAPI paths"

TARGET_PATH="/admin/realms/publico/clients/"
METADATA_FILE="${DECLAREST_REPO_DIR%/}/admin/realms/publico/clients/_/metadata.json"
WILDCARD_CLIENTS_METADATA="${DECLAREST_REPO_DIR%/}/admin/realms/_/clients/_/metadata.json"
WILDCARD_MAPPERS_METADATA="${DECLAREST_REPO_DIR%/}/admin/realms/_/user-registry/_/mappers/_/metadata.json"

cleanup_metadata_for_inference() {
    rm -f "$METADATA_FILE" "$WILDCARD_CLIENTS_METADATA"
}

pickup_metadata_file() {
    if [[ -f "$METADATA_FILE" ]]; then
        printf "%s" "$METADATA_FILE"
        return 0
    fi
    if [[ -f "$WILDCARD_CLIENTS_METADATA" ]]; then
        printf "%s" "$WILDCARD_CLIENTS_METADATA"
        return 0
    fi
    return 1
}

cleanup_metadata_for_inference

output="$(capture_cli_all "metadata infer clients" metadata infer --no-status --apply "$TARGET_PATH")"
pprint="$(printf '%s\n' "$output")"
if ! printf '%s\n' "$pprint" | grep -q '"resourceInfo": {'; then
    die "metadata infer output missing resourceInfo block"
fi
if ! printf '%s\n' "$pprint" | grep -q '"idFromAttribute": "id"'; then
    die "metadata infer output did not include the expected idFromAttribute"
fi
if ! printf '%s\n' "$pprint" | grep -q '"aliasFromAttribute": "clientId"'; then
    die "metadata infer output did not include the expected aliasFromAttribute"
fi

metadata_file="$(pickup_metadata_file || true)"

if [[ -z "$metadata_file" ]]; then
    die "metadata file not generated at $METADATA_FILE or $WILDCARD_CLIENTS_METADATA"
fi

if ! jq -e '(.resourceInfo.idFromAttribute == "id") and (.resourceInfo.aliasFromAttribute == "clientId")' "$metadata_file"; then
    die "generated metadata does not include the expected id/alias values"
fi

log_line "Validating metadata inference recursively"

rm -f "$WILDCARD_CLIENTS_METADATA" "$WILDCARD_MAPPERS_METADATA"

recursive_output="$(capture_cli_all "metadata infer recursive" metadata infer --no-status --apply --recursively --path /)"
pprint="$(strip_debug_info "$recursive_output")"

if ! printf '%s\n' "$pprint" | jq -e 'any(.results[]; .path == "/admin/realms/_/clients")'; then
    die "recursive metadata infer output missing clients collection"
fi
if ! printf '%s\n' "$pprint" | jq -e 'any(.results[]; .path == "/admin/realms/_/user-registry/_/mappers")'; then
    die "recursive metadata infer output missing mapper collection"
fi

if [[ ! -f "$WILDCARD_CLIENTS_METADATA" ]]; then
    die "recursive metadata file missing at $WILDCARD_CLIENTS_METADATA"
fi
if ! jq -e '(.resourceInfo.idFromAttribute == "id") and (.resourceInfo.aliasFromAttribute == "clientId")' "$WILDCARD_CLIENTS_METADATA"; then
    die "recursive clients metadata does not include the expected id/alias values"
fi

if [[ ! -f "$WILDCARD_MAPPERS_METADATA" ]]; then
    die "recursive metadata file missing at $WILDCARD_MAPPERS_METADATA"
fi
if ! jq -e '(.resourceInfo.idFromAttribute == "id") and (.resourceInfo.aliasFromAttribute == "name")' "$WILDCARD_MAPPERS_METADATA"; then
    die "recursive mapper metadata does not include the expected id/alias values"
fi

log_line "Metadata inference smoke test completed"
