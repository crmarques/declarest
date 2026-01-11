#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/cli.sh
source "$SCRIPTS_DIR/lib/cli.sh"

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required for metadata inheritance validation"
fi

log_line "Validating metadata id/alias scoping for collections"

TARGET_PATH="/admin/realms/master/components/"
output="$(capture_cli_all "metadata inheritance defaults" metadata get "$TARGET_PATH")"
pprint="$(strip_debug_info "$output")"

if ! printf '%s\n' "$pprint" | jq -e '(.resourceInfo.idFromAttribute == "id") and (.resourceInfo.aliasFromAttribute == "id")' >/dev/null; then
    die "metadata inheritance did not default id/alias for $TARGET_PATH"
fi

log_line "Metadata inheritance defaults validated"
