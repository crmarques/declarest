#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_case_libs() {
  source_e2e_lib "common"
  source_e2e_lib "args"
  source_e2e_lib "profile"
  source_e2e_lib "components"
  source_e2e_lib "cases"
}

test_requirement_requested_explicitly_tracks_capability_selection() {
  load_case_libs

  E2E_EXPLICIT=()
  E2E_SECRET_PROVIDER='file'

  if case_requirement_requested_explicitly 'has-secret-provider'; then
    fail "expected implicit capability selection not to be marked explicit"
  fi

  e2e_mark_explicit 'secret-provider'
  if ! case_requirement_requested_explicitly 'has-secret-provider'; then
    fail "expected explicit secret provider selection to be marked explicit"
  fi
}

test_requirement_requested_explicitly_tracks_managed_server_auth_type_selector() {
  load_case_libs

  E2E_EXPLICIT=()
  E2E_MANAGED_SERVER_AUTH_TYPE='oauth2'

  if case_requirement_requested_explicitly 'managed-server-auth-type=oauth2'; then
    fail "expected implicit auth-type selection not to be marked explicit"
  fi

  e2e_mark_explicit 'managed-server-auth-type'
  if ! case_requirement_requested_explicitly 'managed-server-auth-type=oauth2'; then
    fail "expected explicit auth-type selection to be marked explicit"
  fi
}

test_requirement_requested_explicitly_tracks_proxy_capability() {
  load_case_libs

  E2E_EXPLICIT=()
  E2E_PROXY_MODE='external'

  if case_requirement_requested_explicitly 'has-proxy'; then
    fail "expected implicit proxy selection not to be marked explicit"
  fi

  e2e_mark_explicit 'proxy-mode'
  if ! case_requirement_requested_explicitly 'has-proxy'; then
    fail "expected explicit proxy selection to be marked explicit"
  fi
}

test_requirement_requested_explicitly_tracks_proxy_auth_type_selector() {
  load_case_libs

  E2E_EXPLICIT=()
  E2E_PROXY_MODE='external'
  E2E_PROXY_AUTH_TYPE='prompt'

  if case_requirement_requested_explicitly 'proxy-auth-type=prompt'; then
    fail "expected implicit proxy auth-type selection not to be marked explicit"
  fi

  e2e_mark_explicit 'proxy-auth-type'
  if ! case_requirement_requested_explicitly 'proxy-auth-type=prompt'; then
    fail "expected explicit proxy auth-type selection to be marked explicit"
  fi
}

test_requirement_requested_explicitly_tracks_generic_proxy_capability_and_selector() {
  load_case_libs

  E2E_EXPLICIT=()
  E2E_PROXY_MODE='local'
  E2E_PROXY_AUTH_TYPE='prompt'

  if case_requirement_requested_explicitly 'has-proxy'; then
    fail "expected implicit generic proxy selection not to be marked explicit"
  fi
  if case_requirement_requested_explicitly 'proxy-auth-type=prompt'; then
    fail "expected implicit generic proxy auth-type selection not to be marked explicit"
  fi

  e2e_mark_explicit 'proxy-mode'
  e2e_mark_explicit 'proxy-auth-type'
  if ! case_requirement_requested_explicitly 'has-proxy'; then
    fail "expected explicit generic proxy selection to be marked explicit"
  fi
  if ! case_requirement_requested_explicitly 'proxy-auth-type=prompt'; then
    fail "expected explicit generic proxy auth-type selection to be marked explicit"
  fi
}

test_collect_case_files_is_deterministic_global_then_component() {
  load_case_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  E2E_DIR="${tmp}"
  mkdir -p "${E2E_DIR}/cases/smoke"
  mkdir -p "${E2E_DIR}/components/managed-server/demo/cases/smoke"

  cat >"${E2E_DIR}/cases/smoke/02-global-b.sh" <<'EOF'
CASE_ID='g2'
CASE_SCOPE='smoke'
CASE_PROFILES='cli operator'
case_run(){ :; }
EOF
  cat >"${E2E_DIR}/cases/smoke/01-global-a.sh" <<'EOF'
CASE_ID='g1'
CASE_SCOPE='smoke'
CASE_PROFILES='cli operator'
case_run(){ :; }
EOF
  cat >"${E2E_DIR}/components/managed-server/demo/cases/smoke/03-component-c.sh" <<'EOF'
CASE_ID='c3'
CASE_SCOPE='smoke'
case_run(){ :; }
EOF

  E2E_PROFILE='cli-basic'
  E2E_SELECTED_COMPONENT_KEYS=('managed-server:demo')
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['managed-server:demo']="${E2E_DIR}/components/managed-server/demo"

  e2e_collect_case_files

  assert_eq "${#E2E_CASE_FILES[@]}" "3"
  assert_eq "${E2E_CASE_FILES[0]}" "${E2E_DIR}/cases/smoke/01-global-a.sh"
  assert_eq "${E2E_CASE_FILES[1]}" "${E2E_DIR}/cases/smoke/02-global-b.sh"
  assert_eq "${E2E_CASE_FILES[2]}" "${E2E_DIR}/components/managed-server/demo/cases/smoke/03-component-c.sh"
}

test_collect_case_files_filters_by_profile_family() {
  load_case_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  E2E_DIR="${tmp}"
  mkdir -p "${E2E_DIR}/cases/smoke"
  mkdir -p "${E2E_DIR}/cases/main"
  mkdir -p "${E2E_DIR}/cases/operator-main"

  cat >"${E2E_DIR}/cases/smoke/01-shared.sh" <<'EOF'
CASE_ID='shared'
CASE_SCOPE='smoke'
CASE_PROFILES='cli operator'
case_run(){ :; }
EOF
  cat >"${E2E_DIR}/cases/main/02-cli-only.sh" <<'EOF'
CASE_ID='cli-only'
CASE_SCOPE='main'
case_run(){ :; }
EOF
  cat >"${E2E_DIR}/cases/main/03-operator-compatible.sh" <<'EOF'
CASE_ID='operator-compatible'
CASE_SCOPE='main'
CASE_PROFILES='cli operator'
case_run(){ :; }
EOF
  cat >"${E2E_DIR}/cases/operator-main/04-operator.sh" <<'EOF'
CASE_ID='operator-only'
CASE_SCOPE='operator-main'
case_run(){ :; }
EOF

  E2E_PROFILE='operator-full'
  E2E_SELECTED_COMPONENT_KEYS=()
  E2E_COMPONENT_PATH=()

  e2e_collect_case_files

  assert_eq "${#E2E_CASE_FILES[@]}" "3"
  assert_eq "${E2E_CASE_FILES[0]}" "${E2E_DIR}/cases/smoke/01-shared.sh"
  assert_eq "${E2E_CASE_FILES[1]}" "${E2E_DIR}/cases/main/03-operator-compatible.sh"
  assert_eq "${E2E_CASE_FILES[2]}" "${E2E_DIR}/cases/operator-main/04-operator.sh"
}

test_collect_case_files_rejects_invalid_case_profiles() {
  load_case_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  E2E_DIR="${tmp}"
  mkdir -p "${E2E_DIR}/cases/smoke"

  cat >"${E2E_DIR}/cases/smoke/01-invalid.sh" <<'EOF'
CASE_ID='invalid'
CASE_SCOPE='smoke'
CASE_PROFILES='cli invalid'
case_run(){ :; }
EOF

  E2E_PROFILE='cli-basic'
  E2E_SELECTED_COMPONENT_KEYS=()
  E2E_COMPONENT_PATH=()

  local output status
  set +e
  output=$(e2e_collect_scope_case_files smoke "${E2E_DIR}/cases" 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "invalid CASE_PROFILES entry"
}

test_requirement_requested_explicitly_tracks_capability_selection
test_requirement_requested_explicitly_tracks_managed_server_auth_type_selector
test_requirement_requested_explicitly_tracks_proxy_capability
test_requirement_requested_explicitly_tracks_proxy_auth_type_selector
test_requirement_requested_explicitly_tracks_generic_proxy_capability_and_selector
test_collect_case_files_is_deterministic_global_then_component
test_collect_case_files_filters_by_profile_family
test_collect_case_files_rejects_invalid_case_profiles
