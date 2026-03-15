#!/usr/bin/env bash

E2E_CONTEXT_NAME=''

e2e_context_has_proxy_basic_auth_values() {
  if declare -F e2e_has_proxy_basic_auth_values >/dev/null 2>&1; then
    e2e_has_proxy_basic_auth_values
    return
  fi

  [[ -n "${E2E_PROXY_AUTH_USERNAME:-}" || -n "${E2E_PROXY_AUTH_PASSWORD:-}" ]]
}

e2e_context_has_managed_server_proxy_basic_auth_values() {
  e2e_context_has_proxy_basic_auth_values
}

e2e_context_effective_proxy_auth_type() {
  if declare -F e2e_effective_proxy_auth_type >/dev/null 2>&1; then
    e2e_effective_proxy_auth_type
    return
  fi

  if [[ "${E2E_PROXY_MODE:-none}" == 'none' ]]; then
    printf 'none\n'
    return 0
  fi

  if [[ -n "${E2E_PROXY_AUTH_TYPE:-}" ]]; then
    printf '%s\n' "${E2E_PROXY_AUTH_TYPE}"
    return 0
  fi

  if e2e_context_has_proxy_basic_auth_values; then
    printf 'basic\n'
    return 0
  fi

  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    printf 'basic\n'
    return 0
  fi

  printf 'none\n'
}

e2e_context_effective_managed_server_proxy_auth_type() {
  e2e_context_effective_proxy_auth_type
}

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

  printf 'currentContext: %s\n' "${E2E_CONTEXT_NAME}" >>"${context_file}"
  e2e_context_insert_managed_server_openapi "${context_file}"
  e2e_context_insert_proxy_config "${context_file}"
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
    e2e_info 'skipping managedServer openapi patch: python interpreter unavailable'
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
        if stripped.startswith('managedServer:'):
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

    if stripped.startswith('baseURL:'):
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
    e2e_info "managedServer http.openapi injected into ${context_file}"
  fi

  return 0
}

e2e_context_service_network_host() {
  local service_name=$1

  if declare -F e2e_operator_service_network_host >/dev/null 2>&1; then
    e2e_operator_service_network_host "${service_name}"
    return
  fi

  if [[ -n "${E2E_K8S_NAMESPACE:-}" ]]; then
    printf '%s.%s.svc.cluster.local\n' "${service_name}" "${E2E_K8S_NAMESPACE}"
    return 0
  fi

  printf '%s\n' "${service_name}"
}

e2e_context_component_service_binding() {
  local component_key=$1
  local state_file rendered_dir service_file service_name mapping remote_port

  [[ -n "${component_key}" ]] || return 1

  state_file=$(e2e_component_state_file "${component_key}")
  rendered_dir=$(e2e_state_get "${state_file}" 'K8S_RENDERED_DIR' 2>/dev/null || true)
  [[ -n "${rendered_dir}" && -d "${rendered_dir}" ]] || return 1

  while IFS= read -r service_file; do
    [[ -n "${service_file}" ]] || continue

    service_name=$(
      awk '
        /^[[:space:]]*kind:[[:space:]]*Service([[:space:]]*#.*)?$/ { in_service=1; next }
        in_service && /^[[:space:]]*metadata:[[:space:]]*$/ { in_metadata=1; next }
        in_service && in_metadata && /^[[:space:]]*name:[[:space:]]*/ {
          value = $0
          sub(/^[[:space:]]*name:[[:space:]]*/, "", value)
          gsub(/[[:space:]]+#.*$/, "", value)
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
          print value
          exit
        }
      ' "${service_file}"
    )
    [[ -n "${service_name}" ]] || continue

    mapping=$(
      awk '
        /declarest\.e2e\/port-forward:[[:space:]]*/ {
          value = $0
          sub(/^.*declarest\.e2e\/port-forward:[[:space:]]*/, "", value)
          gsub(/["'"'"'[:space:]]/, "", value)
          split(value, parts, ",")
          print parts[1]
          exit
        }
      ' "${service_file}"
    )
    [[ -n "${mapping}" ]] || continue

    if [[ "${mapping}" == *:* ]]; then
      remote_port=${mapping#*:}
    else
      remote_port=${mapping}
    fi
    [[ -n "${remote_port}" ]] || continue

    printf '%s\t%s\n' "$(e2e_context_service_network_host "${service_name}")" "${remote_port}"
    return 0
  done < <(find "${rendered_dir}" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.yml' \) | sort)

  return 1
}

e2e_context_managed_server_service_binding() {
  if [[ "${E2E_MANAGED_SERVER_CONNECTION:-local}" != 'local' || "${E2E_MANAGED_SERVER:-none}" == 'none' ]]; then
    return 1
  fi

  e2e_context_component_service_binding "$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")"
}

e2e_context_git_remote_service_binding() {
  if [[ "${E2E_REPO_TYPE:-filesystem}" != 'git' || "${E2E_GIT_PROVIDER_CONNECTION:-local}" != 'local' || -z "${E2E_GIT_PROVIDER:-}" ]]; then
    return 1
  fi

  e2e_context_component_service_binding "$(e2e_component_key 'git-provider' "${E2E_GIT_PROVIDER}")"
}

e2e_context_secret_store_service_binding() {
  if [[ "${E2E_SECRET_PROVIDER:-none}" != 'vault' || "${E2E_SECRET_PROVIDER_CONNECTION:-local}" != 'local' ]]; then
    return 1
  fi

  e2e_context_component_service_binding "$(e2e_component_key 'secret-provider' 'vault')"
}

e2e_context_proxy_state_get() {
  local key=$1
  local proxy_state_file
  proxy_state_file=$(e2e_component_state_file "$(e2e_proxy_component_key)")
  e2e_state_get "${proxy_state_file}" "${key}"
}

e2e_context_resolved_proxy_http_url() {
  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    e2e_context_proxy_state_get 'PROXY_HTTP_URL'
    return
  fi

  printf '%s\n' "${E2E_PROXY_HTTP_URL:-}"
}

e2e_context_resolved_proxy_https_url() {
  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    e2e_context_proxy_state_get 'PROXY_HTTPS_URL'
    return
  fi

  printf '%s\n' "${E2E_PROXY_HTTPS_URL:-}"
}

e2e_context_resolved_proxy_auth_username() {
  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    e2e_context_proxy_state_get 'PROXY_AUTH_USERNAME' 2>/dev/null || true
    return
  fi

  printf '%s\n' "${E2E_PROXY_AUTH_USERNAME:-}"
}

e2e_context_resolved_proxy_auth_password() {
  if [[ "${E2E_PROXY_MODE:-none}" == 'local' ]]; then
    e2e_context_proxy_state_get 'PROXY_AUTH_PASSWORD' 2>/dev/null || true
    return
  fi

  printf '%s\n' "${E2E_PROXY_AUTH_PASSWORD:-}"
}

e2e_context_insert_proxy_config() {
  local context_file=$1

  if [[ "${E2E_PROXY_MODE:-none}" == 'none' ]]; then
    return 0
  fi

  local proxy_http_url=''
  local proxy_https_url=''
  local proxy_no_proxy="${E2E_PROXY_NO_PROXY:-}"
  local proxy_auth_type=''
  local proxy_auth_username=''
  local proxy_auth_password=''

  proxy_http_url=$(e2e_context_resolved_proxy_http_url || true)
  proxy_https_url=$(e2e_context_resolved_proxy_https_url || true)
  proxy_auth_type=$(e2e_context_effective_proxy_auth_type) || return 1

  if [[ -z "${proxy_http_url}" && -z "${proxy_https_url}" ]]; then
    e2e_die '--proxy-mode requires proxy HTTP and/or HTTPS URL configuration'
    return 1
  fi

  case "${proxy_auth_type}" in
    none)
      ;;
    basic)
      proxy_auth_username=$(e2e_context_resolved_proxy_auth_username || true)
      proxy_auth_password=$(e2e_context_resolved_proxy_auth_password || true)
      if [[ -z "${proxy_auth_username}" || -z "${proxy_auth_password}" ]]; then
        e2e_die 'proxy auth-type basic requires both username and password'
        return 1
      fi
      ;;
    prompt)
      proxy_auth_username=''
      proxy_auth_password=''
      ;;
    *)
      e2e_die "invalid proxy auth type: ${proxy_auth_type}"
      return 1
      ;;
  esac

  local managed_server_host=''
  local managed_server_port=''
  local repo_host=''
  local repo_port=''
  local secret_host=''
  local secret_port=''
  local binding=''

  if [[ "${E2E_PROXY_MODE:-none}" == 'local' && "${E2E_PLATFORM:-}" == 'kubernetes' ]]; then
    if binding=$(e2e_context_managed_server_service_binding 2>/dev/null); then
      IFS=$'\t' read -r managed_server_host managed_server_port <<<"${binding}"
    fi
    if binding=$(e2e_context_git_remote_service_binding 2>/dev/null); then
      IFS=$'\t' read -r repo_host repo_port <<<"${binding}"
    fi
    if binding=$(e2e_context_secret_store_service_binding 2>/dev/null); then
      IFS=$'\t' read -r secret_host secret_port <<<"${binding}"
    fi
  fi

  local python_cmd
  if command -v python3 >/dev/null 2>&1; then
    python_cmd='python3'
  elif command -v python >/dev/null 2>&1; then
    python_cmd='python'
  else
    e2e_info 'skipping proxy patch: python interpreter unavailable'
    return 0
  fi

  local patch_output
  patch_output=$(
    E2E_CONTEXT_FILE="${context_file}" \
    E2E_PROXY_HTTP_URL="${proxy_http_url}" \
    E2E_PROXY_HTTPS_URL="${proxy_https_url}" \
    E2E_PROXY_NO_PROXY="${proxy_no_proxy}" \
    E2E_PROXY_AUTH_TYPE="${proxy_auth_type}" \
    E2E_PROXY_AUTH_USERNAME="${proxy_auth_username}" \
    E2E_PROXY_AUTH_PASSWORD="${proxy_auth_password}" \
    E2E_PROXY_MODE="${E2E_PROXY_MODE:-none}" \
    E2E_PLATFORM="${E2E_PLATFORM:-}" \
    E2E_CONTEXT_REWRITE_MANAGED_SERVER_HOST="${managed_server_host}" \
    E2E_CONTEXT_REWRITE_MANAGED_SERVER_PORT="${managed_server_port}" \
    E2E_CONTEXT_REWRITE_REPO_HOST="${repo_host}" \
    E2E_CONTEXT_REWRITE_REPO_PORT="${repo_port}" \
    E2E_CONTEXT_REWRITE_SECRET_HOST="${secret_host}" \
    E2E_CONTEXT_REWRITE_SECRET_PORT="${secret_port}" \
    "${python_cmd}" <<'PY'
import json
import os
from pathlib import Path
from urllib.parse import SplitResult, urlsplit, urlunsplit


context_path = Path(os.environ["E2E_CONTEXT_FILE"])
if not context_path.exists():
    raise SystemExit(0)

proxy = {
    "http_url": os.environ.get("E2E_PROXY_HTTP_URL", ""),
    "https_url": os.environ.get("E2E_PROXY_HTTPS_URL", ""),
    "no_proxy": os.environ.get("E2E_PROXY_NO_PROXY", ""),
    "auth_type": os.environ.get("E2E_PROXY_AUTH_TYPE", "none").strip() or "none",
    "auth_username": os.environ.get("E2E_PROXY_AUTH_USERNAME", ""),
    "auth_password": os.environ.get("E2E_PROXY_AUTH_PASSWORD", ""),
}
proxy_mode = os.environ.get("E2E_PROXY_MODE", "none").strip() or "none"
platform = os.environ.get("E2E_PLATFORM", "").strip()
rewrite_bindings = {
    "managed_server": (
        os.environ.get("E2E_CONTEXT_REWRITE_MANAGED_SERVER_HOST", ""),
        os.environ.get("E2E_CONTEXT_REWRITE_MANAGED_SERVER_PORT", ""),
    ),
    "repository_remote": (
        os.environ.get("E2E_CONTEXT_REWRITE_REPO_HOST", ""),
        os.environ.get("E2E_CONTEXT_REWRITE_REPO_PORT", ""),
    ),
    "secret_store_vault": (
        os.environ.get("E2E_CONTEXT_REWRITE_SECRET_HOST", ""),
        os.environ.get("E2E_CONTEXT_REWRITE_SECRET_PORT", ""),
    ),
}


def indent_of(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def stripped(line: str) -> str:
    return line.lstrip()


def is_content_line(line: str) -> bool:
    value = stripped(line)
    return bool(value) and not value.startswith("#")


def block_end(lines: list[str], start_idx: int, indent: int) -> int:
    for idx in range(start_idx + 1, len(lines)):
        if not is_content_line(lines[idx]):
            continue
        if indent_of(lines[idx]) <= indent:
            return idx
    return len(lines)


def find_context_range(lines: list[str]) -> tuple[int, int] | None:
    context_idx = None
    for idx, line in enumerate(lines):
        if stripped(line).startswith("- name:"):
            context_idx = idx
            break
    if context_idx is None:
        return None
    for idx in range(context_idx + 1, len(lines)):
        if is_content_line(lines[idx]) and indent_of(lines[idx]) == 0:
            return context_idx, idx
    return context_idx, len(lines)


def find_child(lines: list[str], start: int, end: int, parent_indent: int, key: str):
    target = f"{key}:"
    for idx in range(start + 1, end):
        if not is_content_line(lines[idx]):
            continue
        current_indent = indent_of(lines[idx])
        current_stripped = stripped(lines[idx])
        if current_indent <= parent_indent:
            return None
        if current_indent == parent_indent + 2 and current_stripped == target:
            child_end = block_end(lines, idx, current_indent)
            return idx, child_end, current_indent
    return None


def yaml_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def unwrap_scalar(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        inner = value[1:-1]
        if value[0] == "'":
            return inner.replace("''", "'")
        return inner
    return value


def render_scalar(original: str, value: str) -> str:
    original = original.strip()
    if len(original) >= 2 and original[0] == original[-1] == "'":
        return yaml_quote(value)
    if len(original) >= 2 and original[0] == original[-1] == '"':
        return json.dumps(value)
    return value


def proxy_block(indent: int, cfg: dict[str, str]) -> list[str]:
    block = [" " * indent + "proxy:"]
    child_indent = indent + 2
    auth_indent = child_indent + 2
    if cfg["http_url"]:
        block.append(" " * child_indent + f"httpURL: {yaml_quote(cfg['http_url'])}")
    if cfg["https_url"]:
        block.append(" " * child_indent + f"httpsURL: {yaml_quote(cfg['https_url'])}")
    if cfg["no_proxy"]:
        block.append(" " * child_indent + f"noProxy: {yaml_quote(cfg['no_proxy'])}")
    if cfg["auth_type"] == "basic":
        block.append(" " * child_indent + "auth:")
        block.append(" " * auth_indent + f"username: {yaml_quote(cfg['auth_username'])}")
        block.append(" " * auth_indent + f"password: {yaml_quote(cfg['auth_password'])}")
    elif cfg["auth_type"] == "prompt":
        block.append(" " * child_indent + "auth:")
        block.append(" " * auth_indent + "prompt: {}")
    return block


def delete_child_block(lines: list[str], parent_range, key: str):
    start, end, indent = parent_range
    existing = find_child(lines, start, end, indent, key)
    if existing is None:
        return None
    child_start, child_end, _ = existing
    del lines[child_start:child_end]
    return child_start


def preferred_insert_index(lines: list[str], parent_range, preferred_keys: list[str]) -> int:
    start, end, indent = parent_range
    for key in preferred_keys:
        child = find_child(lines, start, end, indent, key)
        if child is not None:
            return child[0]
    return end


def replace_or_insert_proxy(lines: list[str], parent_range, preferred_keys: list[str]):
    existing_start = delete_child_block(lines, parent_range, "proxy")
    parent_start, _, parent_indent = parent_range
    refreshed_range = (parent_start, block_end(lines, parent_start, parent_indent), parent_indent)
    insert_idx = existing_start
    if insert_idx is None:
        insert_idx = preferred_insert_index(lines, refreshed_range, preferred_keys)
    block = proxy_block(parent_indent + 2, proxy)
    lines[insert_idx:insert_idx] = block


def replace_mapping_scalar(lines: list[str], parent_range, key: str, transform) -> bool:
    start, end, indent = parent_range
    target = f"{key}:"
    for idx in range(start + 1, end):
        if not is_content_line(lines[idx]):
            continue
        current_indent = indent_of(lines[idx])
        current_stripped = stripped(lines[idx])
        if current_indent <= indent:
            break
        if current_indent != indent + 2 or not current_stripped.startswith(target):
            continue
        current_value = current_stripped.split(":", 1)[1].strip()
        updated_value = transform(unwrap_scalar(current_value))
        if updated_value == unwrap_scalar(current_value):
            return False
        lines[idx] = " " * current_indent + f"{key}: {render_scalar(current_value, updated_value)}"
        return True
    return False


def rewrite_local_url(raw_value: str, binding: tuple[str, str]) -> str:
    host, port = binding
    if proxy_mode != "local" or platform != "kubernetes" or not host or not port:
        return raw_value

    try:
        parsed = urlsplit(raw_value)
    except ValueError:
        return raw_value

    if parsed.scheme not in ("http", "https"):
        return raw_value
    if parsed.hostname not in ("127.0.0.1", "localhost"):
        return raw_value

    userinfo = ""
    if parsed.username:
        userinfo = parsed.username
        if parsed.password:
            userinfo += f":{parsed.password}"
        userinfo += "@"

    updated = SplitResult(
        scheme=parsed.scheme,
        netloc=f"{userinfo}{host}:{port}",
        path=parsed.path,
        query=parsed.query,
        fragment=parsed.fragment,
    )
    return urlunsplit(updated)


lines = context_path.read_text().splitlines()
context_range = find_context_range(lines)
if context_range is None:
    raise SystemExit(0)

context_start, context_end = context_range
context_indent = indent_of(lines[context_start])
sections = []


managed_server = find_child(lines, context_start, context_end, context_indent, "managedServer")
if managed_server is not None:
    http_section = find_child(lines, managed_server[0], managed_server[1], managed_server[2], "http")
    if http_section is not None:
        if replace_mapping_scalar(lines, http_section, "baseURL", lambda value: rewrite_local_url(value, rewrite_bindings["managed_server"])):
            sections.append("managedServer.baseURL")
            http_section = (http_section[0], block_end(lines, http_section[0], http_section[2]), http_section[2])
        if replace_mapping_scalar(lines, http_section, "healthCheck", lambda value: rewrite_local_url(value, rewrite_bindings["managed_server"])):
            sections.append("managedServer.healthCheck")
            http_section = (http_section[0], block_end(lines, http_section[0], http_section[2]), http_section[2])
        auth_section = find_child(lines, http_section[0], http_section[1], http_section[2], "auth")
        if auth_section is not None:
            oauth2_section = find_child(lines, auth_section[0], auth_section[1], auth_section[2], "oauth2")
            if oauth2_section is not None and replace_mapping_scalar(lines, oauth2_section, "tokenURL", lambda value: rewrite_local_url(value, rewrite_bindings["managed_server"])):
                sections.append("managedServer.tokenURL")
                http_section = (http_section[0], block_end(lines, http_section[0], http_section[2]), http_section[2])
        replace_or_insert_proxy(lines, http_section, ["auth", "tls"])
        sections.append("managedServer.proxy")
        context_start, context_end = find_context_range(lines)
        context_indent = indent_of(lines[context_start])


repository = find_child(lines, context_start, context_end, context_indent, "repository")
if repository is not None:
    git_section = find_child(lines, repository[0], repository[1], repository[2], "git")
    if git_section is not None:
        remote_section = find_child(lines, git_section[0], git_section[1], git_section[2], "remote")
        if remote_section is not None:
            remote_url = None
            for idx in range(remote_section[0] + 1, remote_section[1]):
                if not is_content_line(lines[idx]):
                    continue
                current_indent = indent_of(lines[idx])
                current_stripped = stripped(lines[idx])
                if current_indent <= remote_section[2]:
                    break
                if current_indent == remote_section[2] + 2 and current_stripped.startswith("url:"):
                    remote_url = unwrap_scalar(current_stripped.split(":", 1)[1].strip())
                    break
            if remote_url:
                rewritten = rewrite_local_url(remote_url, rewrite_bindings["repository_remote"])
                if rewritten != remote_url:
                    replace_mapping_scalar(lines, remote_section, "url", lambda _value: rewritten)
                    sections.append("repository.remoteURL")
                    remote_section = (remote_section[0], block_end(lines, remote_section[0], remote_section[2]), remote_section[2])
                if urlsplit(rewritten).scheme in ("http", "https"):
                    replace_or_insert_proxy(lines, remote_section, ["auth", "tls"])
                    sections.append("repository.proxy")
                    context_start, context_end = find_context_range(lines)
                    context_indent = indent_of(lines[context_start])


secret_store = find_child(lines, context_start, context_end, context_indent, "secretStore")
if secret_store is not None:
    vault_section = find_child(lines, secret_store[0], secret_store[1], secret_store[2], "vault")
    if vault_section is not None:
        if replace_mapping_scalar(lines, vault_section, "address", lambda value: rewrite_local_url(value, rewrite_bindings["secret_store_vault"])):
            sections.append("secretStore.address")
            vault_section = (vault_section[0], block_end(lines, vault_section[0], vault_section[2]), vault_section[2])
        replace_or_insert_proxy(lines, vault_section, ["auth", "tls"])
        sections.append("secretStore.proxy")
        context_start, context_end = find_context_range(lines)
        context_indent = indent_of(lines[context_start])


metadata = find_child(lines, context_start, context_end, context_indent, "metadata")
if metadata is not None:
    has_bundle = find_child(lines, metadata[0], metadata[1], metadata[2], "bundle") is not None
    if not has_bundle:
        for idx in range(metadata[0] + 1, metadata[1]):
            if not is_content_line(lines[idx]):
                continue
            current_indent = indent_of(lines[idx])
            current_stripped = stripped(lines[idx])
            if current_indent <= metadata[2]:
                break
            if current_indent == metadata[2] + 2 and current_stripped.startswith("bundle:"):
                has_bundle = True
                break
    if has_bundle:
        replace_or_insert_proxy(lines, metadata, [])
        sections.append("metadata.proxy")


context_path.write_text("\n".join(lines) + "\n")
if sections:
    print("PATCHED " + " ".join(dict.fromkeys(sections)))
PY
  )

  if [[ "${patch_output}" == PATCHED* ]]; then
    e2e_info "proxy context injected into ${context_file}: ${patch_output#PATCHED }"
  fi

  return 0
}

e2e_context_insert_managed_server_proxy() {
  local context_file=$1
  e2e_context_insert_proxy_config "${context_file}"
}
