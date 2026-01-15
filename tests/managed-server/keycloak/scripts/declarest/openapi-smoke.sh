#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/cli.sh"
source "$SCRIPTS_DIR/lib/shell.sh"

log_line "Validating OpenAPI defaults via metadata rendering"

output="$(capture_cli_all "metadata openapi defaults" metadata get /openapi-test/items/item-a --for-resource-only)"

if ! awk '
    /"updateResource"/ { in_update = 1 }
    in_update && /"httpMethod":/ {
        if ($0 ~ /"PATCH"/) { ok = 1 }
        exit
    }
    END { exit ok ? 0 : 1 }
' <<< "$output"; then
    die "Expected updateResource httpMethod PATCH from OpenAPI defaults"
fi

log_line "OpenAPI defaults validated"
