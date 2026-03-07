#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_contract_libs() {
  source_e2e_lib "common"
  source_e2e_lib "components"
}

prepare_contract_runtime() {
  local tmp=$1
  E2E_RUN_ID='contract-hooks'
  E2E_RUN_DIR="${tmp}/run"
  E2E_STATE_DIR="${tmp}/state"
  E2E_LOG_DIR="${tmp}/logs"
  E2E_CONTEXT_DIR="${tmp}/context"
  E2E_CONTEXT_FILE="${tmp}/contexts.yaml"
  E2E_PLATFORM='compose'
  E2E_PROFILE='cli-basic'
  E2E_METADATA='bundle'
  E2E_MANAGED_SERVER='demo'
  E2E_MANAGED_SERVER_CONNECTION='local'
  E2E_MANAGED_SERVER_AUTH_TYPE='oauth2'
  E2E_MANAGED_SERVER_MTLS='false'
  E2E_MANAGED_SERVER_PROXY='false'
  E2E_REPO_TYPE='filesystem'
  E2E_GIT_PROVIDER=''
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_SECRET_PROVIDER='none'
  E2E_SECRET_PROVIDER_CONNECTION='local'
  mkdir -p "${E2E_RUN_DIR}" "${E2E_STATE_DIR}" "${E2E_LOG_DIR}" "${E2E_CONTEXT_DIR}"
}

create_contract_component() {
  local root=$1
  local key=$2
  local init_body=$3
  local configure_body=$4
  local context_body=$5
  local type=${key%%:*}
  local name=${key#*:}
  local dir="${root}/${type}/${name}"

  mkdir -p "${dir}/scripts"
  cat >"${dir}/scripts/init.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
${init_body}
EOF
  cat >"${dir}/scripts/configure-auth.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
${configure_body}
EOF
  cat >"${dir}/scripts/context.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
${context_body}
EOF

  printf '%s\n' "${dir}"
}

register_contract_component() {
  local key=$1
  local path=$2

  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_DEPENDS_ON=()
  E2E_COMPONENT_RUNTIME_KIND=()
  E2E_SELECTED_COMPONENT_KEYS=("${key}")
  E2E_COMPONENT_PATH["${key}"]="${path}"
  E2E_COMPONENT_DEPENDS_ON["${key}"]=''
  E2E_COMPONENT_RUNTIME_KIND["${key}"]='native'
}

test_hook_contract_keeps_state_and_context_deterministic_on_reentry() {
  load_contract_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_contract_runtime "${tmp}"

  local key='managed-server:demo'
  local component_dir
  component_dir=$(create_contract_component \
    "${tmp}/components" \
    "${key}" \
    'printf "TOKEN=alpha\n" >"${E2E_COMPONENT_STATE_FILE}"' \
    'printf "TOKEN=alpha\nAUTH_MODE=oauth2\n" >"${E2E_COMPONENT_STATE_FILE}"' \
    'printf "managedServer:\n  http:\n    baseUrl: http://127.0.0.1:18080\n" >"${E2E_COMPONENT_CONTEXT_FRAGMENT}"')
  register_contract_component "${key}" "${component_dir}"

  e2e_component_run_hook "${key}" init
  e2e_component_run_hook "${key}" configure-auth
  e2e_component_run_hook "${key}" context

  local state_file fragment_file first_state first_fragment second_state second_fragment
  state_file=$(e2e_component_state_file "${key}")
  fragment_file=$(e2e_component_context_fragment_path "${key}")
  first_state=$(cat "${state_file}")
  first_fragment=$(cat "${fragment_file}")

  e2e_component_run_hook "${key}" init
  e2e_component_run_hook "${key}" configure-auth
  e2e_component_run_hook "${key}" context

  second_state=$(cat "${state_file}")
  second_fragment=$(cat "${fragment_file}")

  assert_eq "${first_state}" "${second_state}" "expected repeated init/configure-auth to remain deterministic"
  assert_eq "${first_fragment}" "${second_fragment}" "expected repeated context hook to remain deterministic"
}

test_hook_contract_rejects_missing_state_after_init() {
  load_contract_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_contract_runtime "${tmp}"

  local key='managed-server:demo'
  local component_dir
  component_dir=$(create_contract_component \
    "${tmp}/components" \
    "${key}" \
    ':' \
    'printf "TOKEN=alpha\n" >"${E2E_COMPONENT_STATE_FILE}"' \
    'printf "managedServer:\n  http:\n    baseUrl: http://127.0.0.1:18080\n" >"${E2E_COMPONENT_CONTEXT_FRAGMENT}"')
  register_contract_component "${key}" "${component_dir}"

  local output status
  set +e
  output=$(e2e_component_run_hook "${key}" init 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "reason=missing-or-empty-state"
}

test_hook_contract_rejects_missing_context_fragment() {
  load_contract_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  prepare_contract_runtime "${tmp}"

  local key='managed-server:demo'
  local component_dir
  component_dir=$(create_contract_component \
    "${tmp}/components" \
    "${key}" \
    'printf "TOKEN=alpha\n" >"${E2E_COMPONENT_STATE_FILE}"' \
    'printf "TOKEN=alpha\nAUTH_MODE=oauth2\n" >"${E2E_COMPONENT_STATE_FILE}"' \
    ':')
  register_contract_component "${key}" "${component_dir}"

  e2e_component_run_hook "${key}" init
  e2e_component_run_hook "${key}" configure-auth

  local output status
  set +e
  output=$(e2e_component_run_hook "${key}" context 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "reason=missing-or-empty-context"
}

test_hook_contract_keeps_state_and_context_deterministic_on_reentry
test_hook_contract_rejects_missing_state_after_init
test_hook_contract_rejects_missing_context_fragment
