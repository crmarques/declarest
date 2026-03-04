#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_gitea_compose_config_allows_internal_webhook_hosts() {
  local compose_file="${E2E_SCRIPT_DIR}/components/git-provider/gitea/compose/compose.yaml"
  local content
  content=$(cat "${compose_file}")

  assert_contains "${content}" "GITEA__webhook__ALLOWED_HOST_LIST: external,loopback,private"
}

test_gitea_k8s_config_allows_internal_webhook_hosts() {
  local deployment_file="${E2E_SCRIPT_DIR}/components/git-provider/gitea/k8s/deployment.yaml"
  local content
  content=$(cat "${deployment_file}")

  assert_contains "${content}" "- name: GITEA__webhook__ALLOWED_HOST_LIST"
  assert_contains "${content}" "value: \"external,loopback,private\""
}

test_gitea_compose_config_allows_internal_webhook_hosts
test_gitea_k8s_config_allows_internal_webhook_hosts
