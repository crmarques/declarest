#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_common_libs() {
  source_e2e_libs common
}

test_go_build_target_is_stale_when_target_missing() {
  load_common_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_ROOT_DIR="${tmp}"
  mkdir -p "${E2E_ROOT_DIR}/internal/app"
  : >"${E2E_ROOT_DIR}/go.mod"
  : >"${E2E_ROOT_DIR}/internal/app/service.go"

  if ! e2e_go_build_target_is_stale "${tmp}/bin/declarest" "${E2E_ROOT_DIR}/go.mod" "${E2E_ROOT_DIR}/internal"; then
    fail "expected missing target binary to be treated as stale"
  fi
}

test_go_build_target_is_fresh_when_sources_are_older() {
  load_common_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_ROOT_DIR="${tmp}"
  mkdir -p "${E2E_ROOT_DIR}/internal/app" "${tmp}/bin"
  : >"${E2E_ROOT_DIR}/go.mod"
  : >"${E2E_ROOT_DIR}/internal/app/service.go"
  : >"${tmp}/bin/declarest"
  chmod +x "${tmp}/bin/declarest"

  touch -t 202603010000 "${E2E_ROOT_DIR}/go.mod" "${E2E_ROOT_DIR}/internal/app/service.go"
  touch -t 202603020000 "${tmp}/bin/declarest"

  if e2e_go_build_target_is_stale "${tmp}/bin/declarest" "${E2E_ROOT_DIR}/go.mod" "${E2E_ROOT_DIR}/internal"; then
    fail "expected newer target binary to be treated as fresh"
  fi
}

test_go_build_target_is_stale_when_go_sources_are_newer() {
  load_common_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_ROOT_DIR="${tmp}"
  mkdir -p "${E2E_ROOT_DIR}/internal/app" "${tmp}/bin"
  : >"${E2E_ROOT_DIR}/go.mod"
  : >"${E2E_ROOT_DIR}/internal/app/service.go"
  : >"${tmp}/bin/declarest"
  chmod +x "${tmp}/bin/declarest"

  touch -t 202603010000 "${tmp}/bin/declarest"
  touch -t 202603020000 "${E2E_ROOT_DIR}/internal/app/service.go"

  if ! e2e_go_build_target_is_stale "${tmp}/bin/declarest" "${E2E_ROOT_DIR}/go.mod" "${E2E_ROOT_DIR}/internal"; then
    fail "expected newer Go source to mark target binary as stale"
  fi
}

test_stage_cached_binary_copies_executable_to_run_path() {
  load_common_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  local cached="${tmp}/cache/declarest"
  local staged="${tmp}/runs/test/bin/declarest"
  mkdir -p "$(dirname -- "${cached}")"
  printf '#!/usr/bin/env bash\nprintf cached\\n\n' >"${cached}"
  chmod +x "${cached}"

  e2e_stage_cached_binary "${cached}" "${staged}"

  assert_path_exists "${staged}"
  assert_file_contains "${staged}" 'printf cached'
  [[ -x "${staged}" ]] || fail "expected staged binary to remain executable"
}

test_go_build_target_is_stale_when_target_missing
test_go_build_target_is_fresh_when_sources_are_older
test_go_build_target_is_stale_when_go_sources_are_newer
test_stage_cached_binary_copies_executable_to_run_path
