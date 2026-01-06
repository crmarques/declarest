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

run_cli "create project" resource create --path "/projects/$DECLAREST_PROJECT_NAME"
run_cli "create job" resource create --path "/projects/$DECLAREST_PROJECT_NAME/jobs/$DECLAREST_JOB_NAME"

log_line "Rundeck E2E workflow completed"
