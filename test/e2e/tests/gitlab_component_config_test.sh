#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_gitlab_compose_config_allows_local_webhook_requests() {
  local compose_file="${E2E_SCRIPT_DIR}/components/git-provider/gitlab/compose/compose.yaml"
  local content
  content=$(cat "${compose_file}")

  assert_contains "${content}" "gitlab_rails['allow_local_requests_from_web_hooks_and_services'] = true"
}

test_gitlab_k8s_config_allows_local_webhook_requests() {
  local deployment_file="${E2E_SCRIPT_DIR}/components/git-provider/gitlab/k8s/deployment.yaml"
  local content
  content=$(cat "${deployment_file}")

  assert_contains "${content}" "gitlab_rails['allow_local_requests_from_web_hooks_and_services'] = true"
  assert_contains "${content}" "startupProbe:"
  assert_contains "${content}" "readinessProbe:"
  assert_contains "${content}" "path: /users/sign_in"
}

test_gitlab_health_script_uses_configurable_defaults() {
  local script="${E2E_SCRIPT_DIR}/components/git-provider/gitlab/scripts/health.sh"
  local content
  content=$(cat "${script}")

  assert_contains "${content}" 'attempts=${DECLAREST_E2E_GITLAB_HEALTH_ATTEMPTS:-${E2E_GITLAB_HEALTH_ATTEMPTS:-150}}'
  assert_contains "${content}" 'interval_seconds=${DECLAREST_E2E_GITLAB_HEALTH_INTERVAL_SECONDS:-${E2E_GITLAB_HEALTH_INTERVAL_SECONDS:-2}}'
  assert_contains "${content}" 'gitlab healthcheck pending (%d/%d): %s/users/sign_in'
}

test_gitlab_compose_config_allows_local_webhook_requests
test_gitlab_k8s_config_allows_local_webhook_requests
test_gitlab_health_script_uses_configurable_defaults
