#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
printf "Rundeck interactive mode is not specialized; running e2e workflow.\n" >&2
exec "$SCRIPT_DIR/run-e2e.sh" "$@"
