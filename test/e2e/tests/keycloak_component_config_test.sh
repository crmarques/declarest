#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_keycloak_k8s_config_defers_readiness_to_component_healthcheck() {
  local deployment_file="${E2E_SCRIPT_DIR}/components/managed-server/keycloak/k8s/deployment.yaml"
  local content
  content=$(cat "${deployment_file}")

  assert_not_contains "${content}" "readinessProbe:"
}

test_keycloak_k8s_config_defers_readiness_to_component_healthcheck
