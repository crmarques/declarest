#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"

keep_rundeck="${DECLAREST_KEEP_RUNDECK:-${KEEP_RUNDECK:-0}}"
if [[ "$keep_rundeck" == "1" ]]; then
    log_line "DECLAREST_KEEP_RUNDECK=1; preserving Rundeck container"
    exit 0
fi

if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
    log_line "Container runtime ${CONTAINER_RUNTIME} not available; skipping Rundeck cleanup"
    exit 0
fi

if "$CONTAINER_RUNTIME" ps -a --format '{{.Names}}' | grep -q "^${RUNDECK_CONTAINER_NAME}$"; then
    run_logged "remove Rundeck container" "$CONTAINER_RUNTIME" rm -f "$RUNDECK_CONTAINER_NAME"
fi
