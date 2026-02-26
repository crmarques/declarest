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

test_requirement_requested_explicitly_tracks_resource_server_auth_type_selector() {
  load_case_libs

  E2E_EXPLICIT=()
  E2E_RESOURCE_SERVER_AUTH_TYPE='oauth2'

  if case_requirement_requested_explicitly 'resource-server-auth-type=oauth2'; then
    fail "expected implicit auth-type selection not to be marked explicit"
  fi

  e2e_mark_explicit 'resource-server-auth-type'
  if ! case_requirement_requested_explicitly 'resource-server-auth-type=oauth2'; then
    fail "expected explicit auth-type selection to be marked explicit"
  fi
}

test_collect_case_files_is_deterministic_global_then_component() {
  load_case_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  E2E_DIR="${tmp}"
  mkdir -p "${E2E_DIR}/cases/main"
  mkdir -p "${E2E_DIR}/components/resource-server/demo/cases/main"

  cat >"${E2E_DIR}/cases/main/02-global-b.sh" <<'EOF'
CASE_ID='g2'
CASE_SCOPE='main'
case_run(){ :; }
EOF
  cat >"${E2E_DIR}/cases/main/01-global-a.sh" <<'EOF'
CASE_ID='g1'
CASE_SCOPE='main'
case_run(){ :; }
EOF
  cat >"${E2E_DIR}/components/resource-server/demo/cases/main/03-component-c.sh" <<'EOF'
CASE_ID='c3'
CASE_SCOPE='main'
case_run(){ :; }
EOF

  E2E_PROFILE='basic'
  E2E_SELECTED_COMPONENT_KEYS=('resource-server:demo')
  E2E_COMPONENT_PATH=()
  E2E_COMPONENT_PATH['resource-server:demo']="${E2E_DIR}/components/resource-server/demo"

  e2e_collect_case_files

  assert_eq "${#E2E_CASE_FILES[@]}" "3"
  assert_eq "${E2E_CASE_FILES[0]}" "${E2E_DIR}/cases/main/01-global-a.sh"
  assert_eq "${E2E_CASE_FILES[1]}" "${E2E_DIR}/cases/main/02-global-b.sh"
  assert_eq "${E2E_CASE_FILES[2]}" "${E2E_DIR}/components/resource-server/demo/cases/main/03-component-c.sh"
}

test_requirement_requested_explicitly_tracks_capability_selection
test_requirement_requested_explicitly_tracks_resource_server_auth_type_selector
test_collect_case_files_is_deterministic_global_then_component
