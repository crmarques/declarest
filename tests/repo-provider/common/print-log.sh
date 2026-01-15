#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/logging.sh"
source "$SCRIPTS_DIR/lib/shell.sh"
source "$SCRIPTS_DIR/lib/shell.sh"
source "$SCRIPTS_DIR/lib/repo.sh"

repo_type="$(resolve_repo_type)"
if [[ "$repo_type" != "git-local" ]]; then
    log_line "Git log skipped (repo type: ${repo_type:-unknown})"
    exit 0
fi

require_cmd git

log_block "Repository log" "$(git -C "$DECLAREST_REPO_DIR" log -n 20 --oneline)"
git -C "$DECLAREST_REPO_DIR" log -n 20 --oneline
