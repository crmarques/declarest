#!/usr/bin/env bash
set -euo pipefail
fragment_file=${1:-${E2E_COMPONENT_CONTEXT_FRAGMENT:-}}
[[ -n "${fragment_file}" ]] || exit 0
: >"${fragment_file}"
