#!/usr/bin/env bash

e2e_component_collect_manual_info() {
  local component_key=$1
  local script_path

  script_path=$(e2e_component_hook_script "${component_key}" 'manual-info')
  if [[ ! -f "${script_path}" ]]; then
    return 0
  fi

  local state_file
  state_file=$(e2e_component_state_file "${component_key}")
  mkdir -p -- "$(dirname -- "${state_file}")"
  [[ -f "${state_file}" ]] || : >"${state_file}"

  e2e_component_export_env "${component_key}" 'manual-info'
  bash "${script_path}"
}

e2e_runtime_state_file() {
  printf '%s/runtime.env\n' "${E2E_STATE_DIR}"
}

e2e_runtime_state_set() {
  local key=$1
  local value=$2
  local runtime_state

  runtime_state=$(e2e_runtime_state_file)
  mkdir -p -- "$(dirname -- "${runtime_state}")" || return 1
  [[ -f "${runtime_state}" ]] || : >"${runtime_state}"
  e2e_write_state_value "${runtime_state}" "${key}" "${value}"
}

e2e_runtime_state_record_platform() {
  e2e_runtime_state_set 'RUNTIME_PLATFORM' "${E2E_PLATFORM}" || return 1
  e2e_runtime_state_set 'RUNTIME_CONTAINER_ENGINE' "${E2E_CONTAINER_ENGINE}" || return 1
}

e2e_component_has_local_containerized_runtime() {
  local component_key

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    local connection
    connection=$(e2e_component_connection_for_key "${component_key}")
    if [[ "${connection}" != 'local' ]]; then
      continue
    fi
    if e2e_component_runtime_is_compose "${component_key}"; then
      return 0
    fi
  done

  return 1
}

e2e_kind_cluster_name_for_run() {
  local run_id=$1
  local cluster_name
  cluster_name=$(e2e_sanitize_project_name "declarest-e2e-${run_id}")
  printf '%.63s\n' "${cluster_name}"
}

e2e_k8s_namespace_for_run() {
  local run_id=$1
  local namespace
  namespace=$(e2e_sanitize_project_name "declarest-${run_id}")
  printf '%.63s\n' "${namespace}"
}

e2e_kubeconfig_path_for_run() {
  printf '%s/k8s/kubeconfig\n' "${E2E_RUN_DIR}"
}

e2e_kind_config_path_for_run() {
  printf '%s/k8s/kind-config.yaml\n' "${E2E_RUN_DIR}"
}

e2e_write_kind_config() {
  local config_path=$1
  mkdir -p -- "$(dirname -- "${config_path}")" || return 1

  cat >"${config_path}" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: ${E2E_ROOT_DIR}
        containerPath: ${E2E_KIND_NODE_ROOT}
        selinuxRelabel: false
        propagation: HostToContainer
EOF
}

e2e_kubernetes_runtime_ensure() {
  if [[ "${E2E_PLATFORM}" != 'kubernetes' ]]; then
    return 0
  fi

  if ! e2e_component_has_local_containerized_runtime; then
    return 0
  fi

  if [[ -n "${E2E_KIND_CLUSTER_NAME:-}" && -n "${E2E_KUBECONFIG:-}" && -f "${E2E_KUBECONFIG}" ]]; then
    return 0
  fi

  E2E_KIND_CLUSTER_NAME=$(e2e_kind_cluster_name_for_run "${E2E_RUN_ID}")
  E2E_K8S_NAMESPACE=$(e2e_k8s_namespace_for_run "${E2E_RUN_ID}")
  E2E_KUBECONFIG=$(e2e_kubeconfig_path_for_run)
  local kind_config
  kind_config=$(e2e_kind_config_path_for_run)

  e2e_write_kind_config "${kind_config}" || return 1
  e2e_info "creating kind cluster name=${E2E_KIND_CLUSTER_NAME} namespace=${E2E_K8S_NAMESPACE} kubeconfig=${E2E_KUBECONFIG}"
  e2e_kind_cmd create cluster --name "${E2E_KIND_CLUSTER_NAME}" --kubeconfig "${E2E_KUBECONFIG}" --config "${kind_config}" --wait 120s || return 1

  if ! kubectl --kubeconfig "${E2E_KUBECONFIG}" get namespace "${E2E_K8S_NAMESPACE}" >/dev/null 2>&1; then
    e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" create namespace "${E2E_K8S_NAMESPACE}" || return 1
  fi

  e2e_runtime_state_set 'KIND_CLUSTER_NAME' "${E2E_KIND_CLUSTER_NAME}" || return 1
  e2e_runtime_state_set 'K8S_NAMESPACE' "${E2E_K8S_NAMESPACE}" || return 1
  e2e_runtime_state_set 'KUBECONFIG_PATH' "${E2E_KUBECONFIG}" || return 1
  e2e_runtime_state_set 'KIND_CONFIG_PATH' "${kind_config}" || return 1
  e2e_runtime_state_set 'KIND_NODE_ROOT' "${E2E_KIND_NODE_ROOT}" || return 1
  return 0
}

e2e_kubernetes_runtime_teardown() {
  local cluster_name=${E2E_KIND_CLUSTER_NAME:-}

  if [[ "${E2E_PLATFORM}" != 'kubernetes' && -z "${cluster_name}" ]]; then
    return 0
  fi

  if [[ -z "${cluster_name}" ]]; then
    local runtime_state
    runtime_state=$(e2e_runtime_state_file)
    cluster_name=$(e2e_state_get "${runtime_state}" 'KIND_CLUSTER_NAME' || true)
  fi

  if [[ -z "${cluster_name}" ]]; then
    return 0
  fi

  if ! command -v kind >/dev/null 2>&1; then
    e2e_warn "kind binary not found; skipping cluster teardown for ${cluster_name}"
    return 0
  fi

  local rc=0
  set +e
  if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
    KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name "${cluster_name}" >/dev/null 2>&1
    rc=$?
  else
    kind delete cluster --name "${cluster_name}" >/dev/null 2>&1
    rc=$?
  fi
  set -e

  if ((rc != 0)); then
    e2e_warn "kind cluster deletion returned rc=${rc} for ${cluster_name}"
    return 1
  fi

  e2e_info "deleted kind cluster name=${cluster_name}"
  return 0
}

e2e_components_start_local() {
  E2E_STARTED_COMPONENT_KEYS=()
  e2e_info "starting local containerized components platform=${E2E_PLATFORM} engine=${E2E_CONTAINER_ENGINE}"

  local started_components_file="${E2E_STATE_DIR}/started-components.tsv"
  : >"${started_components_file}"

  local -a start_candidates=()
  local component_key

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    local connection
    connection=$(e2e_component_connection_for_key "${component_key}")

    if [[ "${connection}" != 'local' ]]; then
      e2e_info "component start skipped key=${component_key} reason=connection:${connection}"
      continue
    fi

    if ! e2e_component_runtime_is_compose "${component_key}"; then
      e2e_info "component start skipped key=${component_key} reason=runtime:native"
      continue
    fi

    if [[ "${E2E_PLATFORM}" == 'compose' ]]; then
      E2E_COMPONENT_PROJECT["${component_key}"]=$(e2e_component_default_project_name "${component_key}")
    fi
    start_candidates+=("${component_key}")
  done

  if ((${#start_candidates[@]} == 0)); then
    return 0
  fi

  if [[ "${E2E_PLATFORM}" == 'kubernetes' ]]; then
    e2e_kubernetes_runtime_ensure || return 1
  fi

  e2e_components_run_hook_for_keys 'start' 'true' "${start_candidates[@]}" || return 1

  for component_key in "${start_candidates[@]}"; do
    E2E_STARTED_COMPONENT_KEYS+=("${component_key}")
    printf '%s\t%s\n' "${component_key}" "${E2E_COMPONENT_PROJECT[${component_key}]:-}" >>"${started_components_file}"
  done

  return 0
}

e2e_components_healthcheck_local() {
  if ((${#E2E_STARTED_COMPONENT_KEYS[@]} == 0)); then
    return 0
  fi

  e2e_components_run_hook_for_keys 'health' 'true' "${E2E_STARTED_COMPONENT_KEYS[@]}"
}

e2e_components_stop_started() {
  local index

  for ((index = ${#E2E_STARTED_COMPONENT_KEYS[@]} - 1; index >= 0; index--)); do
    local component_key=${E2E_STARTED_COMPONENT_KEYS[index]}
    e2e_component_run_hook "${component_key}" 'stop' || true
  done

  if [[ "${E2E_PLATFORM}" == 'kubernetes' ]]; then
    e2e_kubernetes_runtime_teardown || true
  fi
}
