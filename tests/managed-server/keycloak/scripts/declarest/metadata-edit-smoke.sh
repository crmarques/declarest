#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/cli.sh"

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required for metadata edit validation"
fi

log_line "Validating metadata edit defaults stripping"

TARGET_PATH="/admin/realms/publico/clients/"
METADATA_FILE="${DECLAREST_REPO_DIR%/}/admin/realms/publico/clients/_/metadata.json"

if [[ ! -f "$METADATA_FILE" ]]; then
    die "metadata file missing at $METADATA_FILE"
fi

if ! jq -e '(.resourceInfo.idFromAttribute == "id") and (.resourceInfo.aliasFromAttribute == "clientId")' "$METADATA_FILE"; then
    die "metadata file is missing expected id/alias values before edit"
fi

EDITOR_SCRIPT="${DECLAREST_WORK_DIR%/}/metadata-edit.sh"
cat > "$EDITOR_SCRIPT" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# No edits needed; metadata edit should strip defaults when saving.
exit 0
EOF
chmod 0755 "$EDITOR_SCRIPT"

run_cli "metadata edit defaults" metadata edit --editor "$EDITOR_SCRIPT" "$TARGET_PATH"

if ! jq -e '(.resourceInfo.aliasFromAttribute == "clientId") and (.resourceInfo | has("idFromAttribute") | not)' "$METADATA_FILE"; then
    die "metadata edit did not strip default idFromAttribute"
fi

if jq -e '.operationInfo?' "$METADATA_FILE" >/dev/null; then
    die "metadata edit did not strip default operationInfo"
fi

log_line "Metadata edit smoke test completed"
