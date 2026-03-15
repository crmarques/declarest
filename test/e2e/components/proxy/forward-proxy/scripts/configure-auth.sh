#!/usr/bin/env bash
set -euo pipefail

if [[ ! -s "${E2E_COMPONENT_STATE_FILE}" ]]; then
  printf 'proxy component state file is missing: %s\n' "${E2E_COMPONENT_STATE_FILE}" >&2
  exit 1
fi
