#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"
source "$SCRIPTS_DIR/lib/logging.sh"

keep_work="${DECLAREST_KEEP_WORK:-${KEEP_WORK:-0}}"
if [[ "$keep_work" == "1" ]]; then
    log_line "DECLAREST_KEEP_WORK=1; preserving work directory at $DECLAREST_WORK_DIR"
    exit 0
fi

if [[ -z "${DECLAREST_WORK_DIR:-}" ]]; then
    exit 0
fi

chmod -R u+w "$DECLAREST_WORK_DIR" >/dev/null 2>&1 || true
rm -rf "$DECLAREST_WORK_DIR"
log_line "Work directory removed: $DECLAREST_WORK_DIR"
