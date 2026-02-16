#!/usr/bin/env bash

CASE_LAST_OUTPUT=''
CASE_LAST_STATUS=0

case_run_declarest() {
  local output

  set +e
  output=$(DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" "$@" 2>&1)
  CASE_LAST_STATUS=$?
  set -e

  CASE_LAST_OUTPUT="${output}"
  return 0
}

case_expect_status() {
  local expected=$1
  if ((CASE_LAST_STATUS != expected)); then
    printf 'expected exit status %d but got %d\n' "${expected}" "${CASE_LAST_STATUS}" >&2
    printf 'command output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_expect_success() {
  case_expect_status 0
}

case_expect_failure() {
  if ((CASE_LAST_STATUS == 0)); then
    printf 'expected command failure but got success\n' >&2
    printf 'command output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_expect_output_contains() {
  local expected=$1
  if ! grep -Fq -- "${expected}" <<<"${CASE_LAST_OUTPUT}"; then
    printf 'expected output to contain: %s\n' "${expected}" >&2
    printf 'actual output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_expect_output_not_contains() {
  local forbidden=$1
  if grep -Fq -- "${forbidden}" <<<"${CASE_LAST_OUTPUT}"; then
    printf 'expected output not to contain: %s\n' "${forbidden}" >&2
    printf 'actual output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_write_json() {
  local path=$1
  local payload=$2
  printf '%s\n' "${payload}" >"${path}"
}

case_jq_value() {
  local jq_expr=$1
  jq -r "${jq_expr}" <<<"${CASE_LAST_OUTPUT}"
}

