#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

test_rundeck_health_script_uses_configurable_defaults() {
  local script="${E2E_SCRIPT_DIR}/components/managed-server/rundeck/scripts/health.sh"
  local content
  content=$(cat "${script}")

  assert_contains "${content}" 'attempts=${DECLAREST_E2E_RUNDECK_HEALTH_ATTEMPTS:-${E2E_RUNDECK_HEALTH_ATTEMPTS:-150}}'
  assert_contains "${content}" 'interval_seconds=${DECLAREST_E2E_RUNDECK_HEALTH_INTERVAL_SECONDS:-${E2E_RUNDECK_HEALTH_INTERVAL_SECONDS:-1}}'
  assert_contains "${content}" 'rundeck healthcheck pending (%d/%d): %s/user/login'
  assert_contains "${content}" 'rundeck healthcheck failed after %d attempts (%ss interval)'
}

test_rundeck_health_script_uses_configurable_defaults
