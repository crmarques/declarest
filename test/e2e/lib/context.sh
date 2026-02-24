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
  e2e_context_insert_resource_server_openapi "${context_file}"
}

e2e_context_insert_resource_server_openapi() {
  local context_file=$1

  if [[ "${E2E_RESOURCE_SERVER:-none}" == 'none' ]]; then
    return 0
  fi

  local component_key
  component_key=$(e2e_component_key 'resource-server' "${E2E_RESOURCE_SERVER}")
  local openapi_spec="${E2E_COMPONENT_OPENAPI_SPEC[${component_key}]:-}"
  if [[ -z "${openapi_spec}" || ! -f "${openapi_spec}" ]]; then
    return 0
  fi

  local python_cmd
  if command -v python3 >/dev/null 2>&1; then
    python_cmd='python3'
  elif command -v python >/dev/null 2>&1; then
    python_cmd='python'
  else
    e2e_info 'skipping resource-server openapi patch: python interpreter unavailable'
    return 0
  fi

  local patch_output
  patch_output=$(
    E2E_CONTEXT_FILE="${context_file}" \
    E2E_CONTEXT_OPENAPI_SPEC="${openapi_spec}" \
    "${python_cmd}" <<'PY'
import os
from pathlib import Path

context_path = Path(os.environ['E2E_CONTEXT_FILE'])
openapi_path = os.environ['E2E_CONTEXT_OPENAPI_SPEC']

if not context_path.exists():
    raise SystemExit(0)

lines = context_path.read_text().splitlines()

resource_indent = None
in_resource_block = False
base_url_idx = None
has_openapi = False

for idx, line in enumerate(lines):
    stripped = line.lstrip()
    if resource_indent is None:
        if stripped.startswith('resource-server:'):
            resource_indent = len(line) - len(stripped)
            in_resource_block = True
        continue

    if not in_resource_block:
        continue

    indent = len(line) - len(stripped)
    if stripped and indent <= resource_indent:
        break

    if stripped.startswith('openapi:'):
        has_openapi = True
        break

    if stripped.startswith('base-url:'):
        base_url_idx = idx

if has_openapi or base_url_idx is None:
    raise SystemExit(0)

indent = len(lines[base_url_idx]) - len(lines[base_url_idx].lstrip())
openapi_line = ' ' * indent + 'openapi: ' + openapi_path
lines.insert(base_url_idx + 1, openapi_line)
context_path.write_text('\n'.join(lines) + '\n')
print('PATCHED')
PY
  )

  if [[ "${patch_output}" == 'PATCHED' ]]; then
    e2e_info "resource-server http.openapi injected into ${context_file}"
  fi

  return 0
}
