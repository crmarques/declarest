#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_cleanup_libs() {
  source_e2e_lib "common"
  source_e2e_lib "runner_cleanup"
}

write_runtime_state() {
  local run_id=$1
  shift
  local runtime_state="${E2E_RUNS_DIR}/${run_id}/state/runtime.env"
  mkdir -p "$(dirname -- "${runtime_state}")"
  : >"${runtime_state}"
  local kv
  for kv in "$@"; do
    printf '%s\n' "${kv}" >>"${runtime_state}"
  done
}

test_cleanup_run_id_validation() {
  load_cleanup_libs

  e2e_validate_cleanup_run_id "20260223-090000-12345"

  local output status
  set +e
  output=$(e2e_validate_cleanup_run_id "../bad" 2>&1)
  status=$?
  set -e
  assert_status "${status}" "1"
  assert_contains "${output}" "invalid cleanup run-id"
}

test_runner_cmdline_and_env_parsers_support_fake_proc_root() {
  load_cleanup_libs
  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  mkdir -p "${tmp}/1234"
  printf 'bash\0./test/e2e/run-e2e.sh\0--profile\0basic\0' >"${tmp}/1234/cmdline"
  printf 'USER=test\0E2E_RUNNER_PID=1234\0E2E_RUN_ID=test-run\0' >"${tmp}/1234/environ"

  E2E_PROC_ROOT="${tmp}"
  e2e_runner_cmdline_matches 1234
  e2e_runner_pid_marker_matches 1234
  e2e_runner_pid_matches_run_id 1234 test-run

  local output status
  set +e
  output=$(e2e_runner_pid_matches_run_id 1234 other-run 2>&1)
  status=$?
  set -e
  assert_status "${status}" "1"
  [[ -z "${output}" ]] || true
}

test_remove_run_bin_entry_from_path() {
  load_cleanup_libs

  local original_path="${PATH}"
  local run_id='cleanup-path-test'
  local run_bin="${E2E_RUNS_DIR}/${run_id}/bin"

  PATH="${run_bin}:${original_path}"
  e2e_remove_run_bin_from_path "${run_id}"
  assert_eq "${PATH}" "${original_path}"

  PATH=":${run_bin}:${original_path}"
  e2e_remove_run_bin_from_path "${run_id}"
  assert_eq "${PATH}" ":${original_path}"

  PATH="${original_path}"
}

test_cleanup_run_runtime_dispatches_by_recorded_platform() {
  load_cleanup_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  E2E_RUNS_DIR="${tmp}/runs"
  mkdir -p "${E2E_RUNS_DIR}"

  write_runtime_state "compose-run" "RUNTIME_PLATFORM=compose"
  write_runtime_state "k8s-run" "RUNTIME_PLATFORM=kubernetes"

  local compose_called=0
  local k8s_called=0

  e2e_cleanup_run_compose_runtime() {
    local run_id=$1
    [[ "${run_id}" == "compose-run" ]] || fail "unexpected compose cleanup run-id: ${run_id}"
    compose_called=1
  }
  e2e_cleanup_run_kubernetes_runtime() {
    local run_id=$1
    [[ "${run_id}" == "k8s-run" ]] || fail "unexpected kubernetes cleanup run-id: ${run_id}"
    k8s_called=1
  }

  e2e_cleanup_run_runtime "compose-run"
  e2e_cleanup_run_runtime "k8s-run"

  assert_eq "${compose_called}" "1" "expected compose runtime cleanup to be selected"
  assert_eq "${k8s_called}" "1" "expected kubernetes runtime cleanup to be selected"
}

test_cleanup_kubernetes_runtime_runs_kind_delete_with_podman_provider() {
  load_cleanup_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN
  E2E_RUNS_DIR="${tmp}/runs"
  mkdir -p "${E2E_RUNS_DIR}"
  write_runtime_state "k8s-run" \
    "RUNTIME_PLATFORM=kubernetes" \
    "RUNTIME_CONTAINER_ENGINE=podman" \
    "KIND_CLUSTER_NAME=declarest-e2e-test"

  local fake_bin="${tmp}/bin"
  local kind_log="${tmp}/kind.log"
  mkdir -p "${fake_bin}"
  cat >"${fake_bin}/kind" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'provider=%s cmd=%s\n' "${KIND_EXPERIMENTAL_PROVIDER:-}" "$*" >>"${FAKE_KIND_LOG}"
exit 0
EOF
  chmod +x "${fake_bin}/kind"

  local original_path="${PATH}"
  PATH="${fake_bin}:${original_path}"
  export PATH
  export FAKE_KIND_LOG="${kind_log}"

  e2e_cleanup_run_kubernetes_runtime "k8s-run"

  assert_file_contains "${kind_log}" "provider=podman cmd=delete cluster --name declarest-e2e-test"
}

test_cleanup_run_id_validation
test_runner_cmdline_and_env_parsers_support_fake_proc_root
test_remove_run_bin_entry_from_path
test_cleanup_run_runtime_dispatches_by_recorded_platform
test_cleanup_kubernetes_runtime_runs_kind_delete_with_podman_provider
