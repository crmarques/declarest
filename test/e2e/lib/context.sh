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
  e2e_context_insert_managed_server_openapi "${context_file}"
  e2e_context_insert_managed_server_proxy "${context_file}"
}

e2e_context_insert_managed_server_openapi() {
  local context_file=$1

  if [[ "${E2E_MANAGED_SERVER:-none}" == 'none' ]]; then
    return 0
  fi

  local component_key
  component_key=$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")
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
    e2e_info 'skipping managed-server openapi patch: python interpreter unavailable'
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
        if stripped.startswith('managed-server:'):
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
    e2e_info "managed-server http.openapi injected into ${context_file}"
  fi

  return 0
}

e2e_context_insert_managed_server_proxy() {
  local context_file=$1

  if [[ "${E2E_MANAGED_SERVER_PROXY:-false}" != 'true' ]]; then
    return 0
  fi

  if [[ "${E2E_MANAGED_SERVER:-none}" == 'none' ]]; then
    return 0
  fi

  local proxy_http_url="${E2E_MANAGED_SERVER_PROXY_HTTP_URL:-}"
  local proxy_https_url="${E2E_MANAGED_SERVER_PROXY_HTTPS_URL:-}"
  local proxy_no_proxy="${E2E_MANAGED_SERVER_PROXY_NO_PROXY:-}"
  local proxy_auth_username="${E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME:-}"
  local proxy_auth_password="${E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD:-}"

  if [[ -z "${proxy_http_url}" && -z "${proxy_https_url}" ]]; then
    e2e_die "--managed-server-proxy requires DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTP_URL and/or DECLAREST_E2E_MANAGED_SERVER_PROXY_HTTPS_URL"
    return 1
  fi

  if [[ -n "${proxy_auth_username}" || -n "${proxy_auth_password}" ]]; then
    if [[ -z "${proxy_auth_username}" || -z "${proxy_auth_password}" ]]; then
      e2e_die 'managed-server proxy auth requires both username and password'
      return 1
    fi
  fi

  local python_cmd
  if command -v python3 >/dev/null 2>&1; then
    python_cmd='python3'
  elif command -v python >/dev/null 2>&1; then
    python_cmd='python'
  else
    e2e_info 'skipping managed-server proxy patch: python interpreter unavailable'
    return 0
  fi

  local patch_output
  patch_output=$(
    E2E_CONTEXT_FILE="${context_file}" \
    E2E_PROXY_HTTP_URL="${proxy_http_url}" \
    E2E_PROXY_HTTPS_URL="${proxy_https_url}" \
    E2E_PROXY_NO_PROXY="${proxy_no_proxy}" \
    E2E_PROXY_AUTH_USERNAME="${proxy_auth_username}" \
    E2E_PROXY_AUTH_PASSWORD="${proxy_auth_password}" \
    "${python_cmd}" <<'PY'
import os
from pathlib import Path

context_path = Path(os.environ["E2E_CONTEXT_FILE"])
http_url = os.environ.get("E2E_PROXY_HTTP_URL", "")
https_url = os.environ.get("E2E_PROXY_HTTPS_URL", "")
no_proxy = os.environ.get("E2E_PROXY_NO_PROXY", "")
auth_username = os.environ.get("E2E_PROXY_AUTH_USERNAME", "")
auth_password = os.environ.get("E2E_PROXY_AUTH_PASSWORD", "")

if not context_path.exists():
    raise SystemExit(0)

def y(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"

lines = context_path.read_text().splitlines()

resource_indent = None
http_indent = None
in_resource = False
in_http = False
insert_idx = None
has_proxy = False

for idx, line in enumerate(lines):
    stripped = line.lstrip()
    indent = len(line) - len(stripped)

    if resource_indent is None:
        if stripped.startswith("managed-server:"):
            resource_indent = indent
            in_resource = True
        continue

    if in_resource and not in_http:
        if stripped and indent <= resource_indent:
            break
        if stripped.startswith("http:") and indent > resource_indent:
            http_indent = indent
            in_http = True
        continue

    if in_http:
        if stripped and indent <= http_indent:
            break
        if stripped.startswith("proxy:"):
            has_proxy = True
            break
        if stripped.startswith("base-url:") or stripped.startswith("openapi:"):
            insert_idx = idx + 1
        continue

if has_proxy or insert_idx is None:
    raise SystemExit(0)

field_indent = len(lines[insert_idx - 1]) - len(lines[insert_idx - 1].lstrip())
proxy_indent = field_indent
nested_indent = proxy_indent + 2
auth_field_indent = nested_indent + 2

block = [" " * proxy_indent + "proxy:"]
if http_url:
    block.append(" " * nested_indent + f"http-url: {y(http_url)}")
if https_url:
    block.append(" " * nested_indent + f"https-url: {y(https_url)}")
if no_proxy:
    block.append(" " * nested_indent + f"no-proxy: {y(no_proxy)}")
if auth_username or auth_password:
    block.append(" " * nested_indent + "auth:")
    block.append(" " * auth_field_indent + f"username: {y(auth_username)}")
    block.append(" " * auth_field_indent + f"password: {y(auth_password)}")

for offset, value in enumerate(block):
    lines.insert(insert_idx + offset, value)

context_path.write_text("\n".join(lines) + "\n")
print("PATCHED")
PY
  )

  if [[ "${patch_output}" == 'PATCHED' ]]; then
    e2e_info "managed-server http.proxy injected into ${context_file}"
  fi

  return 0
}
