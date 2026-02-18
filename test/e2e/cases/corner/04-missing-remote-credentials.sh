#!/usr/bin/env bash

CASE_ID='missing-remote-credentials'
CASE_SCOPE='corner'
CASE_REQUIRES='repo-type=git git-provider-connection=remote'

case_run() {
  local temp_context="${E2E_CASE_TMP_DIR}/contexts-missing-creds.yaml"
  cp "${E2E_CONTEXT_FILE}" "${temp_context}"

  # Remove access-key token from context to validate failure behavior.
  sed -i '/access-key:/,/token:/d' "${temp_context}"

  set +e
  output=$(DECLAREST_CONTEXTS_FILE="${temp_context}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" repo push 2>&1)
  status=$?
  set -e

  if ((status == 0)); then
    printf 'expected repo push to fail with missing remote credentials\n' >&2
    return 1
  fi

  if ! grep -Eiq 'auth|invalid|requires|configuration' <<<"${output}"; then
    printf 'expected auth/configuration failure output, got:\n%s\n' "${output}" >&2
    return 1
  fi
}
