#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
exec "${SCRIPT_DIR}/e2e/run-e2e.sh" "$@"
