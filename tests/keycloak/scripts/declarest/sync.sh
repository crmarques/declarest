#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/text.sh
source "$SCRIPTS_DIR/lib/text.sh"
# shellcheck source=../lib/cli.sh
source "$SCRIPTS_DIR/lib/cli.sh"

CLI_RETRY_PATTERN="502 Bad Gateway|503 Service Unavailable|504 Gateway Timeout|connection refused|connect: connection refused|EOF|i/o timeout"

log_line "Syncing repository resources to Keycloak"
local_output=$(capture_cli "list repository resources" resource list --repo)
split_lines_nonempty local_paths "$local_output"

if [[ ${#local_paths[@]} -eq 0 ]]; then
    log_line "No repository resources found; skipping sync."
    exit 0
fi

sort_paths_by_depth local_paths local_paths_parent_first asc

for local in "${local_paths_parent_first[@]}"; do
    if ! run_cli_retry_transient "apply $local" "${KEYCLOAK_RETRY_ATTEMPTS:-10}" "${KEYCLOAK_RETRY_DELAY:-2}" resource apply --path "$local" --sync; then
        log_line "Apply failed for $local"
        exit 1
    fi
done

log_line "Repository sync completed"
