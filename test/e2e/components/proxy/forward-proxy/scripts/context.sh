#!/usr/bin/env bash
set -euo pipefail

fragment_file=${1:-${E2E_COMPONENT_CONTEXT_FRAGMENT:-}}
[[ -n "${fragment_file}" ]] || {
  printf 'missing context fragment output path\n' >&2
  exit 1
}

printf '# proxy context is injected by test/e2e/lib/context.sh\n' >"${fragment_file}"
