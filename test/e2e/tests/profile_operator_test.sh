#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

reload_profile_libs() {
  unset E2E_EXPLICIT \
    E2E_MANAGED_SERVER E2E_MANAGED_SERVER_CONNECTION E2E_MANAGED_SERVER_AUTH_TYPE E2E_MANAGED_SERVER_MTLS \
    E2E_MANAGED_SERVER_PROXY E2E_MANAGED_SERVER_PROXY_HTTP_URL E2E_MANAGED_SERVER_PROXY_HTTPS_URL E2E_MANAGED_SERVER_PROXY_NO_PROXY \
    E2E_MANAGED_SERVER_PROXY_AUTH_USERNAME E2E_MANAGED_SERVER_PROXY_AUTH_PASSWORD \
    E2E_METADATA \
    E2E_REPO_TYPE E2E_GIT_PROVIDER E2E_GIT_PROVIDER_CONNECTION E2E_SECRET_PROVIDER E2E_SECRET_PROVIDER_CONNECTION \
    E2E_PROFILE E2E_PLATFORM E2E_LIST_COMPONENTS E2E_VALIDATE_COMPONENTS E2E_KEEP_RUNTIME E2E_VERBOSE E2E_CLEAN_RUN_ID E2E_CLEAN_ALL \
    E2E_SELECTED_BY_PROFILE_DEFAULT || true
  source_e2e_libs common args profile
}

test_operator_profile_defaults_and_validation_pass() {
  reload_profile_libs

  e2e_parse_args --profile operator
  e2e_apply_profile_defaults

  assert_eq "${E2E_PLATFORM}" "kubernetes"
  assert_eq "${E2E_REPO_TYPE}" "git"
  assert_eq "${E2E_GIT_PROVIDER}" "gitea"

  e2e_validate_profile_rules
}

test_operator_profile_rejects_compose_platform() {
  reload_profile_libs

  e2e_parse_args --profile operator --platform compose --repo-type git --git-provider gitea
  e2e_apply_profile_defaults

  local output status
  set +e
  output=$(e2e_validate_profile_rules 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "operator profile requires --platform kubernetes"
}

test_operator_profile_rejects_git_builtin_provider() {
  reload_profile_libs

  e2e_parse_args --profile operator --repo-type git --git-provider git
  e2e_apply_profile_defaults

  local output status
  set +e
  output=$(e2e_validate_profile_rules 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "operator profile does not support --git-provider git; choose gitea or gitlab"
}

test_operator_profile_rejects_secret_provider_none() {
  reload_profile_libs

  e2e_parse_args --profile operator --repo-type git --git-provider gitea --secret-provider none
  e2e_apply_profile_defaults

  local output status
  set +e
  output=$(e2e_validate_profile_rules 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "operator profile requires a secret provider (file or vault)"
}

test_operator_profile_defaults_and_validation_pass
test_operator_profile_rejects_compose_platform
test_operator_profile_rejects_git_builtin_provider
test_operator_profile_rejects_secret_provider_none
