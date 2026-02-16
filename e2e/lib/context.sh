#!/usr/bin/env bash

E2E_CONTEXT_NAME=''

e2e_context_build() {
  E2E_CONTEXT_NAME="e2e-${E2E_PROFILE}"

  local context_file="${E2E_CONTEXT_FILE}"
  local fragment

  : >"${context_file}"
  printf 'contexts:\n' >>"${context_file}"
  printf '  - name: %s\n' "${E2E_CONTEXT_NAME}" >>"${context_file}"

  while IFS= read -r fragment; do
    [[ -n "${fragment}" && -f "${fragment}" ]] || continue
    sed 's/^/    /' "${fragment}" >>"${context_file}"
  done < <(find "${E2E_CONTEXT_DIR}" -type f -name '*.yaml' | sort)

  printf 'current-ctx: %s\n' "${E2E_CONTEXT_NAME}" >>"${context_file}"
}

