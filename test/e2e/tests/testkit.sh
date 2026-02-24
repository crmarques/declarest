#!/usr/bin/env bash
set -euo pipefail

TESTS_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd -- "${TESTS_DIR}/../../.." && pwd)
E2E_SCRIPT_DIR="${REPO_ROOT}/test/e2e"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  local got=$1
  local want=$2
  local msg=${3:-}
  if [[ "${got}" != "${want}" ]]; then
    fail "${msg:-expected ${want@Q}, got ${got@Q}}"
  fi
}

assert_contains() {
  local haystack=$1
  local needle=$2
  local msg=${3:-}
  if ! grep -Fq -- "${needle}" <<<"${haystack}"; then
    fail "${msg:-expected output to contain ${needle@Q}}"
  fi
}

assert_not_contains() {
  local haystack=$1
  local needle=$2
  local msg=${3:-}
  if grep -Fq -- "${needle}" <<<"${haystack}"; then
    fail "${msg:-expected output not to contain ${needle@Q}}"
  fi
}

assert_status() {
  local got=$1
  local want=$2
  local msg=${3:-}
  if [[ "${got}" != "${want}" ]]; then
    fail "${msg:-expected status ${want}, got ${got}}"
  fi
}

assert_path_exists() {
  local path=$1
  local msg=${2:-}
  if [[ ! -e "${path}" ]]; then
    fail "${msg:-expected path to exist: ${path}}"
  fi
}

assert_file_contains() {
  local path=$1
  local needle=$2
  local msg=${3:-}
  [[ -f "${path}" ]] || fail "expected file to exist: ${path}"
  if ! grep -Fq -- "${needle}" "${path}"; then
    fail "${msg:-expected ${path} to contain ${needle@Q}}"
  fi
}

new_temp_dir() {
  mktemp -d /tmp/declarest-e2e-tests.XXXXXX
}

source_e2e_lib() {
  local lib=$1
  export SCRIPT_DIR="${E2E_SCRIPT_DIR}"
  # shellcheck disable=SC1090
  source "${E2E_SCRIPT_DIR}/lib/${lib}.sh"
}

source_e2e_libs() {
  local lib
  for lib in "$@"; do
    source_e2e_lib "${lib}"
  done
}

run_capture() {
  local output
  set +e
  output=$("$@" 2>&1)
  local status=$?
  set -e
  printf '%s\n' "${status}"
  printf '%s' "${output}"
}
