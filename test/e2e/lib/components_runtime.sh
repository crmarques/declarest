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

e2e_kind_create_retry_attempts() {
  local attempts=${DECLAREST_E2E_KIND_CREATE_RETRY_ATTEMPTS:-2}

  if ! [[ "${attempts}" =~ ^[0-9]+$ ]] || ((attempts <= 0)); then
    e2e_warn "invalid kind create retry attempts: ${attempts} (using default 2)"
    attempts=2
  fi

  printf '%s\n' "${attempts}"
}

e2e_kind_create_retryable_failure() {
  local kind_log_file=$1
  [[ -f "${kind_log_file}" ]] || return 1

  grep -Eq \
    'could not find a log line that matches "Reached target .*Multi-User System|detected cgroup v1"|The kubelet is not healthy after|failed to init node with kubeadm' \
    "${kind_log_file}"
}

e2e_kind_reuse_existing_on_create_failure_enabled() {
  local raw=${DECLAREST_E2E_KIND_REUSE_EXISTING_ON_CREATE_FAILURE:-true}
  case "${raw,,}" in
    true|1|yes|on)
      return 0
      ;;
    false|0|no|off)
      return 1
      ;;
    *)
      e2e_warn "invalid DECLAREST_E2E_KIND_REUSE_EXISTING_ON_CREATE_FAILURE=${raw}; using true"
      return 0
      ;;
  esac
}

e2e_kind_list_clusters() {
  if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
    KIND_EXPERIMENTAL_PROVIDER=podman kind get clusters
    return $?
  fi
  kind get clusters
}

e2e_kind_pick_existing_cluster_for_reuse() {
  local failed_cluster_name=$1
  local clusters_output

  set +e
  clusters_output=$(e2e_kind_list_clusters 2>/dev/null)
  local rc=$?
  set -e
  if ((rc != 0)); then
    return 1
  fi

  local cluster_name
  while IFS= read -r cluster_name; do
    [[ -n "${cluster_name}" ]] || continue
    [[ "${cluster_name}" == "${failed_cluster_name}" ]] && continue
    printf '%s\n' "${cluster_name}"
    return 0
  done <<<"${clusters_output}"

  return 1
}

e2e_kind_export_kubeconfig_for_cluster() {
  local cluster_name=$1
  local kubeconfig=$2

  e2e_kind_cmd_locked export kubeconfig --name "${cluster_name}" --kubeconfig "${kubeconfig}" || return 1
}

e2e_kind_delete_cluster_quiet_locked() {
  local cluster_name=$1
  [[ -n "${cluster_name}" ]] || return 0

  if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
    KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name "${cluster_name}" >/dev/null 2>&1
    return $?
  fi

  kind delete cluster --name "${cluster_name}" >/dev/null 2>&1
}

e2e_kind_delete_cluster_quiet() {
  local cluster_name=$1
  [[ -n "${cluster_name}" ]] || return 0

  local rc=0
  set +e
  if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
    e2e_with_lock_timeout "$(e2e_kind_lock_name)" "$(e2e_kind_lock_wait_seconds)" \
      e2e_kind_delete_cluster_quiet_locked "${cluster_name}"
    rc=$?
  else
    e2e_kind_delete_cluster_quiet_locked "${cluster_name}"
    rc=$?
  fi
  set -e

  if ((rc == 0)); then
    e2e_info "deleted failed kind cluster attempt name=${cluster_name}"
  fi
  return 0
}

e2e_kind_lock_name() {
  printf 'kind-%s\n' "${E2E_CONTAINER_ENGINE}"
}

e2e_kind_lock_wait_seconds() {
  local seconds=${DECLAREST_E2E_KIND_CREATE_LOCK_WAIT_SECONDS:-600}

  if ! [[ "${seconds}" =~ ^[0-9]+$ ]] || ((seconds <= 0)); then
    e2e_warn "invalid DECLAREST_E2E_KIND_CREATE_LOCK_WAIT_SECONDS=${seconds}; using default 600"
    seconds=600
  fi

  printf '%s\n' "${seconds}"
}

e2e_kind_cmd_locked() {
  if [[ "${E2E_CONTAINER_ENGINE}" != 'podman' ]]; then
    e2e_kind_cmd "$@"
    return $?
  fi

  e2e_with_lock_timeout "$(e2e_kind_lock_name)" "$(e2e_kind_lock_wait_seconds)" e2e_kind_cmd "$@"
}

e2e_kind_active_cluster_slots() {
  if [[ "${E2E_CONTAINER_ENGINE}" != 'podman' ]]; then
    printf '0\n'
    return 0
  fi

  local slots=${DECLAREST_E2E_KIND_ACTIVE_CLUSTER_SLOTS:-2}
  if ! [[ "${slots}" =~ ^[0-9]+$ ]] || ((slots <= 0)); then
    e2e_warn "invalid DECLAREST_E2E_KIND_ACTIVE_CLUSTER_SLOTS=${slots}; using default 2"
    slots=2
  fi

  printf '%s\n' "${slots}"
}

e2e_kind_active_cluster_lock_wait_seconds() {
  local seconds=${DECLAREST_E2E_KIND_ACTIVE_CLUSTER_LOCK_WAIT_SECONDS:-3600}

  if ! [[ "${seconds}" =~ ^[0-9]+$ ]] || ((seconds <= 0)); then
    e2e_warn "invalid DECLAREST_E2E_KIND_ACTIVE_CLUSTER_LOCK_WAIT_SECONDS=${seconds}; using default 3600"
    seconds=3600
  fi

  printf '%s\n' "${seconds}"
}

e2e_kind_active_cluster_lock_name() {
  local slot=$1
  printf 'kind-active-cluster-%s-%02d\n' "${E2E_CONTAINER_ENGINE}" "${slot}"
}

e2e_kind_active_cluster_slot_acquire() {
  if [[ "${E2E_CONTAINER_ENGINE}" != 'podman' ]]; then
    return 0
  fi

  if [[ -n "${E2E_KIND_ACTIVE_CLUSTER_LOCK_PATH:-}" ]]; then
    return 0
  fi

  local slots
  local wait_seconds
  local deadline
  local current_epoch
  local slot
  local lock_path=''

  slots=$(e2e_kind_active_cluster_slots) || return 1
  if ((slots <= 0)); then
    return 0
  fi

  wait_seconds=$(e2e_kind_active_cluster_lock_wait_seconds) || return 1
  deadline=$(( $(e2e_epoch_now) + wait_seconds ))

  while :; do
    for ((slot = 1; slot <= slots; slot++)); do
      lock_path=$(e2e_lock_try_acquire "$(e2e_kind_active_cluster_lock_name "${slot}")" || true)
      if [[ -n "${lock_path}" ]]; then
        E2E_KIND_ACTIVE_CLUSTER_SLOT=${slot}
        E2E_KIND_ACTIVE_CLUSTER_LOCK_PATH=${lock_path}
        e2e_info "acquired active kind cluster slot=${slot}/${slots} engine=${E2E_CONTAINER_ENGINE}"
        return 0
      fi
    done

    current_epoch=$(e2e_epoch_now)
    if ((current_epoch >= deadline)); then
      e2e_die "timed out waiting for active kind cluster slot (engine=${E2E_CONTAINER_ENGINE} slots=${slots})"
      return 1
    fi
    sleep 1
  done
}

e2e_kind_active_cluster_slot_release() {
  if [[ -z "${E2E_KIND_ACTIVE_CLUSTER_LOCK_PATH:-}" ]]; then
    return 0
  fi

  e2e_lock_release "${E2E_KIND_ACTIVE_CLUSTER_LOCK_PATH}"
  e2e_info "released active kind cluster slot=${E2E_KIND_ACTIVE_CLUSTER_SLOT:-} engine=${E2E_CONTAINER_ENGINE}"
  E2E_KIND_ACTIVE_CLUSTER_LOCK_PATH=''
  E2E_KIND_ACTIVE_CLUSTER_SLOT=''
}

e2e_kind_create_cluster_with_retry_locked() {
  local cluster_name=$1
  local kubeconfig=$2
  local kind_config=$3
  local wait_timeout=$4
  E2E_KIND_EFFECTIVE_CLUSTER_NAME=''
  local attempts
  attempts=$(e2e_kind_create_retry_attempts)

  local kind_log_file
  kind_log_file=$(mktemp /tmp/declarest-e2e-kind-create.XXXXXX) || return 1

  local attempt
  local last_rc=1
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    local -a cmd=(
      kind create cluster
      --name "${cluster_name}"
      --kubeconfig "${kubeconfig}"
      --config "${kind_config}"
      --wait "${wait_timeout}"
    )
    if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
      cmd=(env KIND_EXPERIMENTAL_PROVIDER=podman "${cmd[@]}")
    fi

    e2e_info "cmd: $(e2e_quote_cmd "${cmd[@]}")"

    local rc=0
    set +e
    "${cmd[@]}" 2>&1 | tee "${kind_log_file}"
    rc=${PIPESTATUS[0]}
    set -e

    if ((rc == 0)); then
      rm -f "${kind_log_file}" || true
      E2E_KIND_EFFECTIVE_CLUSTER_NAME="${cluster_name}"
      return 0
    fi
    last_rc=${rc}

    e2e_error "cmd failed rc=${rc}: $(e2e_quote_cmd "${cmd[@]}")"

    if ((attempt < attempts)) && [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]] && e2e_kind_create_retryable_failure "${kind_log_file}"; then
      e2e_warn "retryable kind podman bootstrap failure detected; retrying create attempt $((attempt + 1))/${attempts}"
      e2e_kind_delete_cluster_quiet "${cluster_name}" || true
      sleep 2
      continue
    fi

    break
  done

  rm -f "${kind_log_file}" || true
  return "${last_rc}"
}

e2e_kind_create_cluster_with_retry() {
  local cluster_name=$1
  local kubeconfig=$2
  local kind_config=$3
  local wait_timeout=$4

  if [[ "${E2E_CONTAINER_ENGINE}" != 'podman' ]]; then
    e2e_kind_create_cluster_with_retry_locked "${cluster_name}" "${kubeconfig}" "${kind_config}" "${wait_timeout}"
    return $?
  fi

  local lock_name
  local lock_wait_seconds
  lock_name=$(e2e_kind_lock_name)
  lock_wait_seconds=$(e2e_kind_lock_wait_seconds)
  e2e_with_lock_timeout "${lock_name}" "${lock_wait_seconds}" e2e_kind_create_cluster_with_retry_locked "${cluster_name}" "${kubeconfig}" "${kind_config}" "${wait_timeout}"
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

  e2e_kind_active_cluster_slot_acquire || return 1

  E2E_KIND_CLUSTER_NAME=$(e2e_kind_cluster_name_for_run "${E2E_RUN_ID}")
  E2E_K8S_NAMESPACE=$(e2e_k8s_namespace_for_run "${E2E_RUN_ID}")
  E2E_KUBECONFIG=$(e2e_kubeconfig_path_for_run)
  E2E_KIND_CLUSTER_REUSED=0
  local kind_config
  kind_config=$(e2e_kind_config_path_for_run)

  e2e_write_kind_config "${kind_config}" || return 1
  e2e_info "creating kind cluster name=${E2E_KIND_CLUSTER_NAME} namespace=${E2E_K8S_NAMESPACE} kubeconfig=${E2E_KUBECONFIG}"
  local effective_cluster_name="${E2E_KIND_CLUSTER_NAME}"
  e2e_kind_create_cluster_with_retry "${E2E_KIND_CLUSTER_NAME}" "${E2E_KUBECONFIG}" "${kind_config}" '120s' || return 1
  if [[ -n "${E2E_KIND_EFFECTIVE_CLUSTER_NAME:-}" ]]; then
    effective_cluster_name="${E2E_KIND_EFFECTIVE_CLUSTER_NAME}"
  fi
  if [[ "${effective_cluster_name}" != "${E2E_KIND_CLUSTER_NAME}" ]]; then
    E2E_KIND_CLUSTER_NAME="${effective_cluster_name}"
    E2E_KIND_CLUSTER_REUSED=1
  fi

  if ! kubectl --kubeconfig "${E2E_KUBECONFIG}" get namespace "${E2E_K8S_NAMESPACE}" >/dev/null 2>&1; then
    e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" create namespace "${E2E_K8S_NAMESPACE}" || return 1
  fi

  e2e_runtime_state_set 'KIND_CLUSTER_NAME' "${E2E_KIND_CLUSTER_NAME}" || return 1
  e2e_runtime_state_set 'K8S_NAMESPACE' "${E2E_K8S_NAMESPACE}" || return 1
  e2e_runtime_state_set 'KUBECONFIG_PATH' "${E2E_KUBECONFIG}" || return 1
  e2e_runtime_state_set 'KIND_CONFIG_PATH' "${kind_config}" || return 1
  e2e_runtime_state_set 'KIND_NODE_ROOT' "${E2E_KIND_NODE_ROOT}" || return 1
  e2e_runtime_state_set 'KIND_CLUSTER_REUSED' "${E2E_KIND_CLUSTER_REUSED}" || return 1
  return 0
}

e2e_kubernetes_runtime_teardown() {
  local cluster_name=${E2E_KIND_CLUSTER_NAME:-}
  local cluster_reused=${E2E_KIND_CLUSTER_REUSED:-}
  local release_active_slot=0

  if [[ -n "${E2E_KIND_ACTIVE_CLUSTER_LOCK_PATH:-}" ]]; then
    release_active_slot=1
  fi

  if [[ "${E2E_PLATFORM}" != 'kubernetes' && -z "${cluster_name}" && "${release_active_slot}" != '1' ]]; then
    return 0
  fi

  if [[ -z "${cluster_name}" ]]; then
    local runtime_state
    runtime_state=$(e2e_runtime_state_file)
    cluster_name=$(e2e_state_get "${runtime_state}" 'KIND_CLUSTER_NAME' || true)
    if [[ -z "${cluster_reused}" ]]; then
      cluster_reused=$(e2e_state_get "${runtime_state}" 'KIND_CLUSTER_REUSED' || true)
    fi
  fi

  if [[ -z "${cluster_name}" ]]; then
    return 0
  fi

  case "${cluster_reused,,}" in
    1|true|yes|on)
      e2e_info "skipping kind cluster deletion for reused cluster name=${cluster_name}"
      if ((release_active_slot == 1)); then
        e2e_kind_active_cluster_slot_release || true
      fi
      return 0
      ;;
  esac

  if ! command -v kind >/dev/null 2>&1; then
    e2e_warn "kind binary not found; skipping cluster teardown for ${cluster_name}"
    if ((release_active_slot == 1)); then
      e2e_kind_active_cluster_slot_release || true
    fi
    return 0
  fi

  local rc=0
  set +e
  if [[ "${E2E_CONTAINER_ENGINE}" == 'podman' ]]; then
    e2e_with_lock_timeout "$(e2e_kind_lock_name)" "$(e2e_kind_lock_wait_seconds)" \
      e2e_kind_delete_cluster_quiet_locked "${cluster_name}"
    rc=$?
  else
    e2e_kind_delete_cluster_quiet_locked "${cluster_name}"
    rc=$?
  fi
  set -e

  if ((rc != 0)); then
    e2e_warn "kind cluster deletion returned rc=${rc} for ${cluster_name}"
    if ((release_active_slot == 1)); then
      e2e_kind_active_cluster_slot_release || true
    fi
    return 1
  fi

  e2e_info "deleted kind cluster name=${cluster_name}"
  if ((release_active_slot == 1)); then
    e2e_kind_active_cluster_slot_release || true
  fi
  return 0
}

e2e_k8s_manifest_images() {
  local manifest_path=$1
  [[ -f "${manifest_path}" ]] || return 0

  awk '
    /^[[:space:]]*image:[[:space:]]*/ {
      line=$0
      sub(/^[[:space:]]*image:[[:space:]]*/, "", line)
      sub(/[[:space:]]+#.*/, "", line)
      gsub(/^["'"'"']|["'"'"']$/, "", line)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
      if (line != "") print line
    }
  ' "${manifest_path}"
}

e2e_component_collect_k8s_images() {
  local component_key=$1
  local state_file
  local source_manifest
  local rendered_manifest
  local rendered_dir

  state_file=$(e2e_component_state_file "${component_key}")
  rendered_dir="${E2E_STATE_DIR}/k8s-image-render/${component_key//[:\/]/-}"
  mkdir -p "${rendered_dir}" || return 1

  while IFS= read -r source_manifest; do
    [[ -n "${source_manifest}" ]] || continue
    rendered_manifest="${rendered_dir}/$(basename -- "${source_manifest}")"
    e2e_component_render_k8s_manifest "${component_key}" "${source_manifest}" "${state_file}" "${rendered_manifest}" || return 1
    e2e_k8s_manifest_images "${rendered_manifest}" || return 1
  done < <(e2e_component_k8s_manifest_files "${component_key}")
}

e2e_container_image_exists_local() {
  local image=$1

  case "${E2E_CONTAINER_ENGINE}" in
    podman)
      "${E2E_CONTAINER_ENGINE}" image exists "${image}" >/dev/null 2>&1
      ;;
    docker)
      "${E2E_CONTAINER_ENGINE}" image inspect "${image}" >/dev/null 2>&1
      ;;
    *)
      return 1
      ;;
  esac
}

e2e_container_image_id() {
  local image=$1
  local image_id=''

  case "${E2E_CONTAINER_ENGINE}" in
    podman|docker)
      image_id=$("${E2E_CONTAINER_ENGINE}" image inspect --format '{{.Id}}' "${image}" 2>/dev/null | head -n 1 || true)
      ;;
    *)
      return 1
      ;;
  esac

  [[ -n "${image_id}" ]] || {
    e2e_die "unable to resolve local image id for ${image}"
    return 1
  }

  printf '%s\n' "${image_id}"
}

e2e_k8s_pull_image_with_retry() {
  local image=$1
  local attempts=3
  local attempt

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if e2e_run_cmd "${E2E_CONTAINER_ENGINE}" pull "${image}"; then
      return 0
    fi
    if ((attempt < attempts)); then
      e2e_warn "k8s image preload pull failed image=${image} attempt=${attempt}/${attempts}; retrying"
      sleep 2
    fi
  done

  e2e_die "k8s image preload pull failed image=${image} attempts=${attempts}"
  return 1
}

e2e_k8s_prepare_image_archive_locked() {
  local image=$1
  local archive=$2
  local archive_id_file=$3
  local image_id=''
  local cached_image_id=''
  local archive_tmp=''
  local archive_id_tmp=''

  if ! e2e_container_image_exists_local "${image}"; then
    e2e_info "k8s image preload pulling image=${image}"
    e2e_k8s_pull_image_with_retry "${image}" || return 1
  else
    e2e_info "k8s image preload using local image cache image=${image}"
  fi

  image_id=$(e2e_container_image_id "${image}") || return 1
  cached_image_id=$(cat "${archive_id_file}" 2>/dev/null || true)
  if [[ -f "${archive}" && -n "${cached_image_id}" && "${cached_image_id}" == "${image_id}" ]]; then
    e2e_info "k8s image preload using cached exported archive image=${image} archive=${archive}"
    return 0
  fi

  archive_tmp="${archive}.${E2E_RUN_ID:-$$}.tmp"
  archive_id_tmp="${archive_id_file}.${E2E_RUN_ID:-$$}.tmp"
  rm -f "${archive_tmp}" "${archive_id_tmp}" || return 1

  e2e_info "k8s image preload exporting image=${image} archive=${archive}"
  e2e_run_cmd "${E2E_CONTAINER_ENGINE}" save -o "${archive_tmp}" "${image}" || return 1
  mv -f "${archive_tmp}" "${archive}" || return 1
  printf '%s\n' "${image_id}" >"${archive_id_tmp}" || return 1
  mv -f "${archive_id_tmp}" "${archive_id_file}" || return 1
}

e2e_k8s_preload_image_to_kind() {
  local image=$1
  local cache_dir="${E2E_BUILD_CACHE_DIR}/k8s-image-cache"
  local safe_image
  local archive
  local archive_id_file

  mkdir -p "${cache_dir}" || return 1
  safe_image=${image//[^a-zA-Z0-9._-]/_}
  archive="${cache_dir}/${safe_image}.tar"
  archive_id_file="${archive}.image-id"
  e2e_with_lock "k8s-image-archive-${safe_image}" \
    e2e_k8s_prepare_image_archive_locked "${image}" "${archive}" "${archive_id_file}" || return 1

  e2e_info "k8s image preload loading image into kind cluster=${E2E_KIND_CLUSTER_NAME} image=${image}"
  e2e_kind_cmd_locked load image-archive "${archive}" --name "${E2E_KIND_CLUSTER_NAME}" || return 1
}

e2e_kubernetes_preload_selected_images() {
  [[ "${E2E_PLATFORM}" == 'kubernetes' ]] || return 0
  [[ -n "${E2E_KIND_CLUSTER_NAME:-}" ]] || return 0

  local component_key
  local connection
  local image
  local -A images=()

  for component_key in "${E2E_SELECTED_COMPONENT_KEYS[@]}"; do
    connection=$(e2e_component_connection_for_key "${component_key}")
    if [[ "${connection}" != 'local' ]]; then
      continue
    fi
    if ! e2e_component_runtime_is_compose "${component_key}"; then
      continue
    fi

    while IFS= read -r image; do
      [[ -n "${image}" ]] || continue
      images["${image}"]=1
    done < <(e2e_component_collect_k8s_images "${component_key}")
  done

  if ((${#images[@]} == 0)); then
    return 0
  fi

  local sorted_images=()
  mapfile -t sorted_images < <(printf '%s\n' "${!images[@]}" | sort)
  for image in "${sorted_images[@]}"; do
    e2e_k8s_preload_image_to_kind "${image}" || return 1
  done

  e2e_info "k8s image preload complete images=${#sorted_images[@]}"
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
    e2e_kubernetes_preload_selected_images || return 1
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
