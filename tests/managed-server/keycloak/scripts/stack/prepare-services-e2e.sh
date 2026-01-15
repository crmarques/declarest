#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPTS_DIR/lib/env.sh"

provider_setup="${1:-}"

"$SCRIPT_DIR/prepare-services.sh"
if [[ -n "$provider_setup" ]]; then
    "$provider_setup"
fi

