#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"

rm -rf "$DECLAREST_WORK_DIR" "$DECLAREST_HOME_DIR"
mkdir -p "$DECLAREST_WORK_DIR" "$DECLAREST_LOG_DIR" "$DECLAREST_HOME_DIR" "$DECLAREST_BIN_DIR" "$DECLAREST_REPO_DIR"
rm -f "$DECLAREST_RUNDECK_ENV_FILE"

log_line "Workspace prepared at $DECLAREST_WORK_DIR"
log_line "Home directory prepared at $DECLAREST_HOME_DIR"
