#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "${E2E_DIR}/lib/common.sh"
# shellcheck disable=SC1091
source "${E2E_COMPONENT_STATE_FILE}"

selected_auth_type=${E2E_MANAGED_SERVICE_AUTH_TYPE:-basic}
case "${selected_auth_type}" in
  basic|prompt) ;;
  *)
    e2e_die "managed-service haproxy does not support auth-type ${selected_auth_type} (supported: basic, prompt)"
    exit 1
    ;;
esac

e2e_write_state_value "${E2E_COMPONENT_STATE_FILE}" MANAGED_SERVICE_AUTH_KIND "basic"
e2e_write_state_value "${E2E_COMPONENT_STATE_FILE}" MANAGED_SERVICE_ACCESS_AUTH_MODE "basic"
