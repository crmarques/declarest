#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"

mkdir -p "$DECLAREST_BIN_DIR"

log_line "Building declarest CLI"
run_logged "go build declarest" bash -c "cd \"$DECLAREST_TEST_DIR/../..\" && go build -o \"$DECLAREST_BIN_DIR/declarest\" ./cli"
