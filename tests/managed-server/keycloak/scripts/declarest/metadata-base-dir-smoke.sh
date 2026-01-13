#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required for metadata base dir validation"
fi

log_line "Validating metadata base_dir override"

if [[ -z "${DECLAREST_METADATA_DIR:-}" ]]; then
    die "DECLAREST_METADATA_DIR must be set for metadata base_dir validation"
fi
if [[ -z "${DECLAREST_REPO_DIR:-}" ]]; then
    die "DECLAREST_REPO_DIR is required to locate repository metadata"
fi

TARGET_PATH="/admin/realms/metadata-base-dir-test/"
METADATA_REL="admin/realms/metadata-base-dir-test/_/metadata.json"
METADATA_FILE="${DECLAREST_METADATA_DIR%/}/$METADATA_REL"
REPO_METADATA_FILE="${DECLAREST_REPO_DIR%/}/$METADATA_REL"

mkdir -p "$(dirname "$METADATA_FILE")"

run_cli "metadata set base dir" metadata set --attribute resourceInfo.description --value '"metadata base dir test"' "$TARGET_PATH"

if [[ ! -f "$METADATA_FILE" ]]; then
    die "expected metadata file at $METADATA_FILE, but it does not exist"
fi

if [[ -f "$REPO_METADATA_FILE" ]]; then
    die "metadata file unexpectedly written under repository path $REPO_METADATA_FILE"
fi

if ! jq -e '.resourceInfo.description == "metadata base dir test"' "$METADATA_FILE"; then
    die "metadata file missing description in ${METADATA_FILE}"
fi

log_line "Metadata base dir smoke test confirmed metadata lives in $METADATA_FILE"
