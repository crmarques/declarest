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

context_name="${DECLAREST_CONTEXT_NAME:-rundeck-e2e}"

log_line "Registering declarest context (${context_name})"
if ! run_cli "cli add-context" config add-context --name "$context_name" --config "$DECLAREST_CONTEXT_FILE"; then
    run_cli "cli set-context" config set-context --name "$context_name" --config "$DECLAREST_CONTEXT_FILE"
fi
run_cli "cli set-current-context" config set-current-context --name "$context_name"

log_line "Declarest context ready"
