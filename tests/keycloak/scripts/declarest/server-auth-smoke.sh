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
# shellcheck source=../lib/cli.sh
source "$SCRIPTS_DIR/lib/cli.sh"

if [[ -z "${DECLAREST_CONTEXT_FILE:-}" ]]; then
    die "Context file is not configured"
fi

"$SCRIPTS_DIR/context/render.sh"
"$SCRIPTS_DIR/context/register.sh"

secret_store_type="${DECLAREST_SECRET_STORE_TYPE:-}"
secret_store_type="${secret_store_type,,}"
repo_type="${DECLAREST_REPO_TYPE:-}"
repo_type="${repo_type,,}"
if [[ "$secret_store_type" == "none" && "$repo_type" == "git-remote" ]]; then
    "$SCRIPTS_DIR/repo/strip-secrets.sh" "$DECLAREST_REPO_DIR"
fi

run_cli "server auth smoke list" resource list --remote

log_line "Server auth smoke completed"
