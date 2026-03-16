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
  assert_not_contains "${content}" "readinessProbe:"
}

test_gitlab_compose_config_allows_local_webhook_requests
test_gitlab_k8s_config_allows_local_webhook_requests
