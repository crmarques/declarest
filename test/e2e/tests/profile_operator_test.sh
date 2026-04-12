#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_profile_libs() {
  unset E2E_EXPLICIT \
    E2E_MANAGED_SERVICE E2E_MANAGED_SERVICE_CONNECTION E2E_MANAGED_SERVICE_AUTH_TYPE E2E_MANAGED_SERVICE_MTLS \
    E2E_METADATA \
    E2E_REPO_TYPE E2E_GIT_PROVIDER E2E_GIT_PROVIDER_CONNECTION E2E_SECRET_PROVIDER E2E_SECRET_PROVIDER_CONNECTION \
    E2E_PROFILE E2E_PLATFORM E2E_LIST_COMPONENTS E2E_VALIDATE_COMPONENTS E2E_KEEP_RUNTIME E2E_VERBOSE E2E_CLEAN_RUN_ID E2E_CLEAN_ALL \
    E2E_SELECTED_BY_PROFILE_DEFAULT || true
  source_e2e_libs common args profile
}

test_operator_profile_defaults_and_validation_pass() {
  reload_profile_libs

  e2e_parse_args --profile operator-manual
  e2e_apply_profile_defaults

  local operator_default_git_provider
  operator_default_git_provider=$(e2e_component_default_name_for_type 'git-provider' 'operator')

  assert_eq "${E2E_PLATFORM}" "kubernetes"
  assert_eq "${E2E_REPO_TYPE}" "git"
  assert_eq "${E2E_GIT_PROVIDER}" "${operator_default_git_provider}"

  e2e_validate_profile_rules
}

test_operator_profile_rejects_compose_platform() {
  reload_profile_libs

  e2e_parse_args --profile operator-manual --platform compose --repo-type git --git-provider gitea
  e2e_apply_profile_defaults

  local output status
  set +e
  output=$(e2e_validate_profile_rules 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "operator-* profiles require --platform kubernetes"
}

test_operator_profile_rejects_git_builtin_provider() {
  reload_profile_libs

  e2e_parse_args --profile operator-manual --repo-type git --git-provider git
  e2e_apply_profile_defaults

  local output status
  set +e
  output=$(e2e_validate_profile_rules 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "operator-* profiles require a git-provider component that declares REPOSITORY_WEBHOOK_PROVIDER; selected git does not"
}

test_operator_profile_rejects_secret_provider_none() {
  reload_profile_libs

  e2e_parse_args --profile operator-manual --repo-type git --git-provider gitea --secret-provider none
  e2e_apply_profile_defaults

  local output status
  set +e
  output=$(e2e_validate_profile_rules 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "operator-* profiles require a selected secret-provider component"
}

test_operator_profile_automated_scopes() {
  reload_profile_libs

  e2e_parse_args --profile operator-basic
  e2e_apply_profile_defaults
  mapfile -t scopes < <(e2e_profile_scopes)
  assert_eq "${scopes[*]}" "smoke operator-main"

  reload_profile_libs
  e2e_parse_args --profile operator-full
  e2e_apply_profile_defaults
  mapfile -t scopes < <(e2e_profile_scopes)
  assert_eq "${scopes[*]}" "smoke main operator-main corner"
}

test_cli_profile_automated_scopes() {
  reload_profile_libs

  e2e_parse_args --profile cli-basic
  e2e_apply_profile_defaults
  mapfile -t scopes < <(e2e_profile_scopes)
  assert_eq "${scopes[*]}" "smoke"

  reload_profile_libs
  e2e_parse_args --profile cli-full
  e2e_apply_profile_defaults
  mapfile -t scopes < <(e2e_profile_scopes)
  assert_eq "${scopes[*]}" "smoke main corner"
}

test_operator_profile_builds_linux_static_manager_binary() {
  local script="${REPO_ROOT}/test/e2e/run-e2e.sh"

  assert_file_contains "${script}" 'go_arch=$(e2e_resolve_go_arch) || return 1'
  assert_file_contains "${script}" 'e2e_run_cmd env CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" go build -o "${cached_operator_tmp}" ./cmd/declarest-operator-manager || return 1'
  assert_file_contains "${script}" 'e2e_write_operator_runtime_dockerfile "${operator_runtime_dockerfile}" "${operator_binary_rel}" || return 1'
  assert_file_contains "${script}" 'e2e_run_cmd "${E2E_CONTAINER_ENGINE}" build -f "${operator_runtime_dockerfile}" -t "${E2E_OPERATOR_IMAGE}" "${E2E_BUILD_CACHE_DIR}" || return 1'
  assert_not_contains "$(cat "${script}")" 'podman build -f "${E2E_ROOT_DIR}/Dockerfile.operator"'
}

test_operator_profile_uses_supported_repository_poll_interval() {
  local script="${REPO_ROOT}/test/e2e/lib/operator.sh"

  assert_file_contains "${script}" 'pollInterval: 30s'
}

test_operator_profile_sets_home_to_writable_state_dir() {
  local script="${REPO_ROOT}/test/e2e/lib/operator.sh"

  assert_file_contains "${script}" '- name: HOME'
  assert_file_contains "${script}" 'value: ${runtime_root}'
}

test_operator_profile_sets_api_server_env_for_manager() {
  local script="${REPO_ROOT}/test/e2e/lib/operator.sh"

  assert_file_contains "${script}" 'e2e_operator_api_server_endpoint()'
  assert_file_contains "${script}" '- name: KUBERNETES_SERVICE_HOST'
  assert_file_contains "${script}" '- name: KUBERNETES_SERVICE_PORT'
}

test_operator_profile_defaults_and_validation_pass
test_operator_profile_rejects_compose_platform
test_operator_profile_rejects_git_builtin_provider
test_operator_profile_rejects_secret_provider_none
test_operator_profile_automated_scopes
test_cli_profile_automated_scopes
test_operator_profile_builds_linux_static_manager_binary
test_operator_profile_uses_supported_repository_poll_interval
test_operator_profile_sets_home_to_writable_state_dir
test_operator_profile_sets_api_server_env_for_manager
