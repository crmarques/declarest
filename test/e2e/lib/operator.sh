#!/usr/bin/env bash

# Operator profile helpers for installing the manager as an in-cluster Deployment
# and applying generated CR instances based on selected component state.

declare -Ag E2E_COMPONENT_SERVICE_PORT=()
declare -Ag E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH=()
declare -Ag E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD=()
declare -Ag E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER=()

e2e_operator_profile_enabled() {
  e2e_profile_is_operator
}

e2e_operator_should_defer_repository_webhook_registration() {
  e2e_operator_profile_enabled || return 1
  [[ -n "${E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER:-}" ]]
}

e2e_operator_configure_repository_webhook_if_needed() {
  e2e_operator_should_defer_repository_webhook_registration || return 0
  [[ "${E2E_REPO_TYPE:-}" == 'git' ]] || return 0
  [[ -n "${E2E_GIT_PROVIDER:-}" && "${E2E_GIT_PROVIDER}" != 'none' ]] || return 0

  local git_provider_key
  git_provider_key=$(e2e_component_key 'git-provider' "${E2E_GIT_PROVIDER}") || return 1

  e2e_info "operator profile configuring repository webhook provider=${E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER} git-provider=${E2E_GIT_PROVIDER}"
  e2e_components_run_hook_for_keys 'configure-auth' 'false' "${git_provider_key}"
}

e2e_operator_manifest_dir() {
  printf '%s/operator/manifests\n' "${E2E_RUN_DIR}"
}

e2e_operator_yaml_quote() {
  local value=$1
  value=${value//\'/\'\'}
  printf "'%s'" "${value}"
}

e2e_operator_b64() {
  printf '%s' "$1" | base64 | tr -d '\n'
}

e2e_operator_managed_service_metadata_bundle_secret_name() {
  e2e_operator_scoped_name 'declarest-operator-managed-service-metadata'
}

e2e_operator_managed_service_metadata_bundle_mount_dir() {
  printf '/var/run/declarest-managed-service-metadata\n'
}

e2e_operator_managed_service_metadata_bundle_mount_path() {
  printf '%s/metadata-bundle.tar.gz\n' "$(e2e_operator_managed_service_metadata_bundle_mount_dir)"
}

e2e_operator_prepare_managed_service_metadata_bundle() {
  E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE=''
  E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH=''

  local archive_path="${E2E_RUN_DIR}/operator/managed-service-metadata-bundle.tar.gz"
  mkdir -p "${E2E_RUN_DIR}/operator" || return 1

  if [[ -n "${E2E_METADATA_BUNDLE:-}" ]]; then
    local bundle_ref="${E2E_METADATA_BUNDLE}"
    local bundle_name=${bundle_ref%%:*}
    local bundle_version=${bundle_ref#*:}
    local cache_dir="${HOME}/.declarest/metadata-bundles/${bundle_name}-${bundle_version}"

    if [[ -f "${cache_dir}/bundle.yaml" && -d "${cache_dir}/metadata" ]]; then
      local -a archive_entries=(bundle.yaml metadata)
      local openapi_name
      for openapi_name in openapi.yaml openapi.yml openapi.json; do
        if [[ -f "${cache_dir}/${openapi_name}" ]]; then
          archive_entries+=("${openapi_name}")
          break
        fi
      done

      rm -f -- "${archive_path}"
      if ! tar -C "${cache_dir}" -czf "${archive_path}" "${archive_entries[@]}"; then
        e2e_die "failed to create operator metadata bundle archive from cache: ${cache_dir}"
        return 1
      fi

      E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE="${archive_path}"
      E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH=$(e2e_operator_managed_service_metadata_bundle_mount_path)
      export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE
      export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH
      return 0
    fi

    e2e_info "operator metadata bundle cache unavailable for bundle=${bundle_ref}; using bundle ref without local archive"
    export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE
    export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH
    return 0
  fi

  if [[ -z "${E2E_METADATA_DIR:-}" ]]; then
    export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE
    export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH
    return 0
  fi

  if [[ ! -d "${E2E_METADATA_DIR}" ]]; then
    e2e_die "operator profile metadata directory is unavailable: ${E2E_METADATA_DIR}"
    return 1
  fi

  local bundle_name
  bundle_name=$(e2e_operator_sanitize_name "e2e-${E2E_MANAGED_SERVICE:-managed-service}-bundle")

  local bundle_root="${E2E_RUN_DIR}/operator/managed-service-metadata-bundle"
  local metadata_file_name

  rm -rf -- "${bundle_root}"
  rm -f -- "${archive_path}"
  mkdir -p "${bundle_root}/metadata" || return 1

  if ! cp -R "${E2E_METADATA_DIR}/." "${bundle_root}/metadata/"; then
    e2e_die "failed to copy metadata fixtures into operator bundle workspace: ${E2E_METADATA_DIR}"
    return 1
  fi

  metadata_file_name=$(e2e_metadata_file_name_for_root "${E2E_METADATA_DIR}") || return 1

  cat >"${bundle_root}/bundle.yaml" <<EOF_BUNDLE_MANIFEST
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: ${bundle_name}
version: 0.0.1
description: E2E metadata bundle for ${E2E_MANAGED_SERVICE:-managed-service}.
declarest:
  shorthand: ${bundle_name}
  metadataRoot: metadata
  metadataFileName: ${metadata_file_name}
EOF_BUNDLE_MANIFEST

  if ! tar -C "${bundle_root}" -czf "${archive_path}" bundle.yaml metadata; then
    e2e_die "failed to create operator metadata bundle archive: ${archive_path}"
    return 1
  fi

  E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE="${archive_path}"
  E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH=$(e2e_operator_managed_service_metadata_bundle_mount_path)
  export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE
  export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH
  return 0
}

e2e_operator_sanitize_name() {
  local value=$1
  value=${value//[^a-zA-Z0-9]/-}
  value=${value,,}
  while [[ "${value}" == *--* ]]; do
    value=${value//--/-}
  done
  value=${value#-}
  value=${value%-}
  if [[ -z "${value}" ]]; then
    value='e2e'
  fi
  printf '%s\n' "${value}"
}

e2e_operator_name_suffix() {
  local suffix
  suffix=$(e2e_operator_sanitize_name "${E2E_RUN_ID:-run}")
  printf '%s\n' "${suffix}"
}

e2e_operator_effective_namespace() {
  local namespace=${E2E_K8S_NAMESPACE:-}
  if [[ -n "${namespace}" ]]; then
    printf '%s\n' "${namespace}"
    return 0
  fi

  if [[ "${E2E_PLATFORM:-}" == 'kubernetes' && -n "${E2E_RUN_ID:-}" ]]; then
    if declare -F e2e_k8s_namespace_for_run >/dev/null 2>&1; then
      namespace=$(e2e_k8s_namespace_for_run "${E2E_RUN_ID}")
    else
      namespace=$(e2e_operator_sanitize_name "declarest-${E2E_RUN_ID}")
      namespace=$(printf '%.63s' "${namespace}")
    fi
    if [[ -n "${namespace}" ]]; then
      printf '%s\n' "${namespace}"
      return 0
    fi
  fi

  printf 'default\n'
}

e2e_operator_scoped_name() {
  local base=$1
  local max_len=${2:-63}
  local safe_base
  local suffix
  local keep_base

  safe_base=$(e2e_operator_sanitize_name "${base}")
  suffix=$(e2e_operator_name_suffix)
  if [[ -z "${suffix}" ]]; then
    suffix='run'
  fi

  if ((max_len < 3)); then
    printf 'e2e\n'
    return 0
  fi

  if (( ${#safe_base} + 1 + ${#suffix} <= max_len )); then
    printf '%s-%s\n' "${safe_base}" "${suffix}"
    return 0
  fi

  keep_base=$((max_len - 1 - ${#suffix}))
  if ((keep_base < 1)); then
    suffix=${suffix:0:$((max_len - 2))}
    suffix=${suffix%-}
    if [[ -z "${suffix}" ]]; then
      suffix='run'
    fi
    printf 'e-%s\n' "${suffix}"
    return 0
  fi

  safe_base=${safe_base:0:${keep_base}}
  safe_base=${safe_base%-}
  if [[ -z "${safe_base}" ]]; then
    safe_base='e2e'
  fi
  printf '%s-%s\n' "${safe_base}" "${suffix}"
}

e2e_operator_run_label_value() {
  local run_label
  run_label=$(e2e_operator_sanitize_name "${E2E_RUN_ID:-run}")
  printf '%.63s\n' "${run_label}"
}

e2e_operator_repo_url_with_username() {
  local remote_url=$1
  local username=$2

  if [[ -z "${username}" ]]; then
    printf '%s\n' "${remote_url}"
    return 0
  fi

  case "${remote_url}" in
    http://*@*|https://*@*)
      printf '%s\n' "${remote_url}"
      ;;
    http://*)
      printf 'http://%s@%s\n' "${username}" "${remote_url#http://}"
      ;;
    https://*)
      printf 'https://%s@%s\n' "${username}" "${remote_url#https://}"
      ;;
    *)
      printf '%s\n' "${remote_url}"
      ;;
  esac
}

e2e_operator_service_host() {
  local service_name=$1
  local namespace
  namespace=$(e2e_operator_effective_namespace)
  printf '%s.%s.svc.cluster.local\n' "${service_name}" "${namespace}"
}

e2e_operator_service_network_host() {
  local service_name=$1
  local namespace
  namespace=$(e2e_operator_effective_namespace)

  if [[ -n "${E2E_KUBECONFIG:-}" ]]; then
    local pod_ip
    pod_ip=$(
      kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" \
        get pods -l "app.kubernetes.io/name=${service_name}" \
        -o jsonpath='{.items[0].status.podIP}' 2>/dev/null || true
    )
    if [[ -n "${pod_ip}" ]]; then
      printf '%s\n' "${pod_ip}"
      return 0
    fi

    local cluster_ip
    cluster_ip=$(
      kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" \
        get "service/${service_name}" \
        -o jsonpath='{.spec.clusterIP}' 2>/dev/null || true
    )
    if [[ -n "${cluster_ip}" && "${cluster_ip}" != 'None' ]]; then
      printf '%s\n' "${cluster_ip}"
      return 0
    fi
  fi

  e2e_operator_service_host "${service_name}"
}

e2e_operator_api_server_endpoint() {
  if [[ -z "${E2E_KUBECONFIG:-}" ]]; then
    e2e_die 'operator profile kubeconfig is unavailable for API endpoint discovery'
    return 1
  fi

  local api_host
  api_host=$(
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n default \
      get service/kubernetes \
      -o jsonpath='{.spec.clusterIP}' 2>/dev/null || true
  )
  local api_port
  api_port=$(
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n default \
      get endpoints/kubernetes \
      -o jsonpath='{.subsets[0].ports[0].port}' 2>/dev/null || true
  )

  if [[ -z "${api_host}" || "${api_host}" == 'None' || -z "${api_port}" ]]; then
    e2e_die 'operator profile could not resolve kubernetes API endpoint from default/kubernetes'
    return 1
  fi

  # kind+podman clusters can expose broken service VIP routing from pods; prefer endpoint address.
  local endpoint_host
  endpoint_host=$(
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n default \
      get endpoints/kubernetes \
      -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || true
  )
  if [[ -n "${endpoint_host}" ]]; then
    api_host="${endpoint_host}"
  fi

  printf '%s %s\n' "${api_host}" "${api_port}"
}

e2e_operator_rewrite_local_url_to_service() {
  local raw_url=$1
  local service_name=$2
  local service_port=$3

  if [[ "${E2E_PLATFORM:-}" != 'kubernetes' ]]; then
    printf '%s\n' "${raw_url}"
    return 0
  fi

  if [[ ! "${raw_url}" =~ ^([a-zA-Z][a-zA-Z0-9+.-]*)://([^/@]+@)?([^/:?#]+)(:([0-9]+))?([/?#].*)?$ ]]; then
    printf '%s\n' "${raw_url}"
    return 0
  fi

  local scheme=${BASH_REMATCH[1]}
  local userinfo=${BASH_REMATCH[2]:-}
  local host=${BASH_REMATCH[3]}
  local suffix=${BASH_REMATCH[6]:-}

  case "${host}" in
    127.0.0.1|localhost) ;;
    *)
      printf '%s\n' "${raw_url}"
      return 0
      ;;
  esac

  printf '%s://%s%s:%s%s\n' \
    "${scheme}" \
    "${userinfo}" \
    "$(e2e_operator_service_network_host "${service_name}")" \
    "${service_port}" \
    "${suffix}"
}

e2e_operator_component_service_name() {
  local component_key=$1
  printf '%s-%s\n' "$(e2e_component_type "${component_key}")" "$(e2e_component_name "${component_key}")"
}

e2e_operator_rewrite_local_url_to_component_service() {
  local raw_url=$1
  local component_key=$2
  local service_port=${E2E_COMPONENT_SERVICE_PORT[${component_key}]:-}

  if [[ -z "${service_port}" ]]; then
    printf '%s\n' "${raw_url}"
    return 0
  fi

  e2e_operator_rewrite_local_url_to_service \
    "${raw_url}" \
    "$(e2e_operator_component_service_name "${component_key}")" \
    "${service_port}"
}

e2e_operator_rewrite_repo_url_for_cluster() {
  local repo_url=$1
  local git_provider_key

  if [[ "${E2E_PLATFORM:-}" != 'kubernetes' || "${E2E_GIT_PROVIDER_CONNECTION:-}" != 'local' ]]; then
    printf '%s\n' "${repo_url}"
    return 0
  fi

  [[ -n "${E2E_GIT_PROVIDER:-}" && "${E2E_GIT_PROVIDER}" != 'none' ]] || {
    printf '%s\n' "${repo_url}"
    return 0
  }

  git_provider_key=$(e2e_component_key 'git-provider' "${E2E_GIT_PROVIDER}")
  e2e_operator_rewrite_local_url_to_component_service "${repo_url}" "${git_provider_key}"
}

e2e_operator_generate_webhook_secret() {
  local random_block
  random_block=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 40 || true)
  if [[ ${#random_block} -lt 32 ]]; then
    random_block="$(date +%s%N)"
  fi
  printf 'declarest-webhook-%s\n' "${random_block}"
}

e2e_operator_repository_webhook_service_name() {
  e2e_operator_scoped_name 'declarest-operator-repo-webhook'
}

e2e_operator_prepare_repository_webhook() {
  E2E_OPERATOR_REPOSITORY_WEBHOOK_URL=''
  E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET=''
  E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER=''
  E2E_OPERATOR_REPOSITORY_WEBHOOK_SERVICE_NAME=''
  E2E_OPERATOR_REPOSITORY_NAME=''

  e2e_operator_profile_enabled || return 0
  [[ "${E2E_REPO_TYPE:-}" == 'git' ]] || return 0
  [[ -n "${E2E_GIT_PROVIDER:-}" && "${E2E_GIT_PROVIDER}" != 'none' ]] || return 0

  local git_provider_key
  git_provider_key=$(e2e_component_key 'git-provider' "${E2E_GIT_PROVIDER}")
  local webhook_provider=${E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER[${git_provider_key}]:-}
  [[ -n "${webhook_provider}" ]] || return 0

  local namespace
  namespace=$(e2e_operator_effective_namespace)
  local repository_name
  local service_name
  local webhook_secret

  repository_name=$(e2e_operator_scoped_name 'declarest-e2e-repository')
  service_name=$(e2e_operator_repository_webhook_service_name)
  webhook_secret=${DECLAREST_E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET:-}
  if [[ -z "${webhook_secret}" ]]; then
    webhook_secret=$(e2e_operator_generate_webhook_secret)
  fi

  E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER="${webhook_provider}"
  E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET="${webhook_secret}"
  E2E_OPERATOR_REPOSITORY_WEBHOOK_SERVICE_NAME="${service_name}"
  E2E_OPERATOR_REPOSITORY_NAME="${repository_name}"
  E2E_OPERATOR_REPOSITORY_WEBHOOK_URL="http://${service_name}.${namespace}.svc.cluster.local:18082/webhooks/repository/${namespace}/${repository_name}"

  export E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_SERVICE_NAME
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_URL
  export E2E_OPERATOR_REPOSITORY_NAME

  if [[ -n "${E2E_STATE_DIR:-}" ]] && command -v e2e_runtime_state_set >/dev/null 2>&1; then
    e2e_runtime_state_set 'OPERATOR_REPOSITORY_NAME' "${repository_name}" || return 1
    e2e_runtime_state_set 'OPERATOR_REPOSITORY_WEBHOOK_PROVIDER' "${E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER}" || return 1
    e2e_runtime_state_set 'OPERATOR_REPOSITORY_WEBHOOK_URL' "${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL}" || return 1
    e2e_runtime_state_set 'OPERATOR_REPOSITORY_WEBHOOK_SERVICE_NAME' "${E2E_OPERATOR_REPOSITORY_WEBHOOK_SERVICE_NAME}" || return 1
  fi
  return 0
}

e2e_operator_ready_timeout_seconds() {
  local timeout_seconds=${E2E_OPERATOR_READY_TIMEOUT_SECONDS:-120}

  if ! [[ "${timeout_seconds}" =~ ^[0-9]+$ ]] || ((timeout_seconds <= 0)); then
    e2e_die "invalid operator readiness timeout: ${timeout_seconds} (set DECLAREST_E2E_OPERATOR_READY_TIMEOUT_SECONDS to a positive integer up to 600)"
    return 1
  fi

  if ((timeout_seconds > 600)); then
    e2e_warn "operator readiness timeout capped from ${timeout_seconds}s to 600s"
    timeout_seconds=600
  fi

  printf '%s\n' "${timeout_seconds}"
}

e2e_operator_stop_manager() {
  e2e_operator_olm_cleanup_current_run || return 1
}

e2e_operator_olm_run_dir() {
  printf '%s/operator/olm\n' "${E2E_RUN_DIR}"
}

e2e_operator_olm_core_dir() {
  printf '%s/test/e2e/olm/v0.42.0\n' "${E2E_ROOT_DIR}"
}

e2e_operator_olm_core_crds_manifest_path() {
  printf '%s/crds.yaml\n' "$(e2e_operator_olm_core_dir)"
}

e2e_operator_olm_core_runtime_manifest_path() {
  printf '%s/olm.yaml\n' "$(e2e_operator_olm_core_dir)"
}

e2e_operator_olm_core_required_crds() {
  printf '%s\n' \
    catalogsources.operators.coreos.com \
    clusterserviceversions.operators.coreos.com \
    installplans.operators.coreos.com \
    olmconfigs.operators.coreos.com \
    operatorconditions.operators.coreos.com \
    operatorgroups.operators.coreos.com \
    operators.operators.coreos.com \
    subscriptions.operators.coreos.com
}

e2e_operator_olm_bundle_image() {
  printf 'localhost/declarest/e2e-operator-bundle:%s\n' "${E2E_RUN_ID}"
}

e2e_operator_olm_catalog_image() {
  printf 'localhost/declarest/e2e-operator-catalog:%s\n' "${E2E_RUN_ID}"
}

e2e_operator_olm_image_repo() {
  local image=$1
  printf '%s\n' "${image%:*}"
}

e2e_operator_olm_image_digest_ref() {
  local image=$1
  local digest=$2
  local repo
  repo=$(e2e_operator_olm_image_repo "${image}")
  printf '%s@%s\n' "${repo}" "${digest}"
}

e2e_operator_olm_bundle_image_ref() {
  local image
  image=$(e2e_operator_olm_bundle_image)
  if [[ -n "${E2E_OPERATOR_OLM_BUNDLE_DIGEST:-}" ]]; then
    e2e_operator_olm_image_digest_ref "${image}" "${E2E_OPERATOR_OLM_BUNDLE_DIGEST}"
    return 0
  fi
  printf '%s\n' "${image}"
}

e2e_operator_olm_catalog_image_ref() {
  local image
  image=$(e2e_operator_olm_catalog_image)
  if [[ -n "${E2E_OPERATOR_OLM_CATALOG_DIGEST:-}" ]]; then
    e2e_operator_olm_image_digest_ref "${image}" "${E2E_OPERATOR_OLM_CATALOG_DIGEST}"
    return 0
  fi
  printf '%s\n' "${image}"
}

e2e_operator_olm_catalog_source_name() {
  e2e_operator_scoped_name 'declarest-catalog'
}

e2e_operator_olm_operator_group_name() {
  e2e_operator_scoped_name 'declarest-operators'
}

e2e_operator_olm_subscription_name() {
  e2e_operator_scoped_name 'declarest-operator'
}

e2e_operator_olm_install_manifest_path() {
  printf '%s/olm-install.yaml\n' "$(e2e_operator_olm_run_dir)"
}

e2e_operator_olm_catalog_manifest_path() {
  printf '%s/olm-catalog.yaml\n' "$(e2e_operator_olm_run_dir)"
}

e2e_operator_olm_subscription_manifest_path() {
  printf '%s/olm-subscription.yaml\n' "$(e2e_operator_olm_run_dir)"
}

e2e_operator_olm_prepare_bundle_tree() {
  local src_bundle="${E2E_ROOT_DIR}/bundle"
  local src_dockerfile="${E2E_ROOT_DIR}/bundle.Dockerfile"
  if [[ ! -d "${src_bundle}" ]]; then
    e2e_die "operator OLM bundle directory is unavailable: ${src_bundle}"
    return 1
  fi
  if [[ ! -f "${src_dockerfile}" ]]; then
    e2e_die "operator OLM bundle dockerfile is unavailable: ${src_dockerfile}"
    return 1
  fi

  local run_dir
  run_dir=$(e2e_operator_olm_run_dir)
  local dst_bundle="${run_dir}/bundle"
  local dst_dockerfile="${run_dir}/bundle.Dockerfile"

  rm -rf "${dst_bundle}" || return 1
  mkdir -p "${run_dir}" || return 1
  cp -R "${src_bundle}" "${dst_bundle}" || return 1
  cp -f "${src_dockerfile}" "${dst_dockerfile}" || return 1

  local csv_file=''
  local candidate
  for candidate in "${dst_bundle}/manifests/"*.clusterserviceversion.yaml; do
    if [[ -f "${candidate}" ]]; then
      csv_file="${candidate}"
      break
    fi
  done
  if [[ -z "${csv_file}" ]]; then
    e2e_die "operator OLM CSV manifest not found under ${dst_bundle}/manifests"
    return 1
  fi

  e2e_operator_olm_patch_csv "${csv_file}" || return 1
  return 0
}

e2e_operator_olm_patch_csv() {
  local csv_file=$1
  local python_cmd
  if command -v python3 >/dev/null 2>&1; then
    python_cmd='python3'
  elif command -v python >/dev/null 2>&1; then
    python_cmd='python'
  else
    e2e_die 'python interpreter is required to patch the OLM bundle CSV'
    return 1
  fi

  local metadata_secret_name=''
  local metadata_mount_dir=''
  if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE:-}" ]]; then
    metadata_secret_name=$(e2e_operator_managed_service_metadata_bundle_secret_name)
    metadata_mount_dir=$(e2e_operator_managed_service_metadata_bundle_mount_dir)
  fi

  E2E_OLM_CSV_FILE="${csv_file}" \
  E2E_OLM_OPERATOR_IMAGE="${E2E_OPERATOR_IMAGE}" \
  E2E_OLM_WATCH_NAMESPACE="$(e2e_operator_effective_namespace)" \
  E2E_OLM_METADATA_SECRET_NAME="${metadata_secret_name}" \
  E2E_OLM_METADATA_MOUNT_DIR="${metadata_mount_dir}" \
  "${python_cmd}" <<'PY' || return 1
import os
import sys

try:
    import yaml
except ImportError:
    sys.stderr.write('PyYAML is required to patch the OLM bundle CSV\n')
    sys.exit(1)

csv_path = os.environ['E2E_OLM_CSV_FILE']
image = os.environ.get('E2E_OLM_OPERATOR_IMAGE', '')
watch_namespace = os.environ.get('E2E_OLM_WATCH_NAMESPACE', '')
metadata_secret = os.environ.get('E2E_OLM_METADATA_SECRET_NAME', '')
metadata_mount = os.environ.get('E2E_OLM_METADATA_MOUNT_DIR', '')
metadata_volume_name = 'managed-service-metadata'

with open(csv_path, 'r', encoding='utf-8') as handle:
    data = yaml.safe_load(handle)

spec = data.setdefault('spec', {})

install = spec.setdefault('install', {})
install_spec = install.setdefault('spec', {})
deployment_names = []
for deployment in install_spec.get('deployments', []) or []:
    deployment_name = deployment.get('name')
    if deployment_name:
        deployment_names.append(deployment_name)
    dep_spec = deployment.setdefault('spec', {})
    template = dep_spec.setdefault('template', {})
    pod_spec = template.setdefault('spec', {})

    volumes = pod_spec.get('volumes', []) or []
    for volume in volumes:
        if volume.pop('persistentVolumeClaim', None) is not None:
            volume['emptyDir'] = volume.get('emptyDir') or {}

    if metadata_secret and metadata_mount:
        if not any(v.get('name') == metadata_volume_name for v in volumes):
            volumes.append({
                'name': metadata_volume_name,
                'secret': {'secretName': metadata_secret},
            })
    pod_spec['volumes'] = volumes

    for container in pod_spec.get('containers', []) or []:
        if image:
            container['image'] = image
        args = []
        for arg in container.get('args') or []:
            if arg.startswith('--enable-admission-webhooks=') or arg == '--enable-admission-webhooks':
                continue
            if arg.startswith('--watch-namespace=') or arg == '--watch-namespace':
                continue
            if arg.startswith('--repository-webhook-bind-address=') or arg == '--repository-webhook-bind-address':
                continue
            args.append(arg)
        args.append('--enable-admission-webhooks=true')
        if watch_namespace:
            args.append(f'--watch-namespace={watch_namespace}')
        args.append('--repository-webhook-bind-address=:8082')
        container['args'] = args

        if metadata_secret and metadata_mount:
            mounts = container.get('volumeMounts', []) or []
            if not any(m.get('name') == metadata_volume_name for m in mounts):
                mounts.append({
                    'name': metadata_volume_name,
                    'mountPath': metadata_mount,
                    'readOnly': True,
                })
            container['volumeMounts'] = mounts

if deployment_names:
    default_deployment_name = deployment_names[0]
    for webhook in spec.get('webhookdefinitions') or []:
        if webhook.get('deploymentName') not in deployment_names:
            webhook['deploymentName'] = default_deployment_name
        webhook.setdefault('containerPort', 9443)
        webhook.setdefault('targetPort', 9443)

with open(csv_path, 'w', encoding='utf-8') as handle:
    yaml.safe_dump(data, handle, sort_keys=False, default_flow_style=False)
PY
  return 0
}

e2e_operator_olm_python_cmd() {
  if command -v python3 >/dev/null 2>&1; then
    printf 'python3\n'
    return 0
  fi
  if command -v python >/dev/null 2>&1; then
    printf 'python\n'
    return 0
  fi
  e2e_die 'python interpreter is required to parse OCI image archives'
  return 1
}

e2e_operator_olm_read_oci_archive_digest() {
  local archive=$1
  local python_cmd
  python_cmd=$(e2e_operator_olm_python_cmd) || return 1
  tar -xOf "${archive}" index.json | "${python_cmd}" -c '
import json
import sys

index = json.load(sys.stdin)
manifests = index.get("manifests", []) or []
if not manifests:
    sys.stderr.write("OCI archive index.json has no manifests\n")
    sys.exit(1)
print(manifests[0]["digest"])
'
}

e2e_operator_olm_save_oci_archive() {
  local image=$1
  local archive=$2
  e2e_run_cmd "${E2E_CONTAINER_ENGINE}" save --format oci-archive -o "${archive}" "${image}" || return 1
  return 0
}

e2e_operator_olm_build_bundle_image() {
  local run_dir
  run_dir=$(e2e_operator_olm_run_dir)
  local image
  image=$(e2e_operator_olm_bundle_image)

  e2e_info "operator profile building OLM bundle image image=${image}"
  e2e_run_cmd "${E2E_CONTAINER_ENGINE}" build \
    -f "${run_dir}/bundle.Dockerfile" \
    -t "${image}" \
    "${run_dir}" || return 1

  local archive_dir="${run_dir}/archives"
  mkdir -p "${archive_dir}" || return 1
  local bundle_archive="${archive_dir}/bundle.oci.tar"
  e2e_operator_olm_save_oci_archive "${image}" "${bundle_archive}" || return 1
  E2E_OPERATOR_OLM_BUNDLE_DIGEST=$(e2e_operator_olm_read_oci_archive_digest "${bundle_archive}") || return 1
  export E2E_OPERATOR_OLM_BUNDLE_DIGEST
  e2e_info "operator profile captured OLM bundle digest ${E2E_OPERATOR_OLM_BUNDLE_DIGEST}"
  return 0
}

e2e_operator_olm_prepare_catalog_tree() {
  local run_dir
  run_dir=$(e2e_operator_olm_run_dir)
  local catalog_dir="${run_dir}/catalog/declarest-operator"
  local catalog_dockerfile="${run_dir}/catalog.Dockerfile"
  local bundle_image
  bundle_image=$(e2e_operator_olm_bundle_image_ref)

  rm -rf "${run_dir}/catalog" || return 1
  mkdir -p "${catalog_dir}" || return 1

  cat >"${catalog_dir}/catalog.yaml" <<EOF_CATALOG
---
schema: olm.package
name: declarest-operator
defaultChannel: alpha
---
schema: olm.channel
package: declarest-operator
name: alpha
entries:
  - name: declarest-operator.v0.0.1
---
schema: olm.bundle
name: declarest-operator.v0.0.1
package: declarest-operator
image: ${bundle_image}
properties:
  - type: olm.package
    value:
      packageName: declarest-operator
      version: 0.0.1
EOF_CATALOG

  cat >"${catalog_dockerfile}" <<'EOF_CATALOG_DOCKER'
FROM quay.io/operator-framework/opm:v1.48.0

LABEL operators.operatorframework.io.index.configs.v1=/configs

ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs", "--cache-dir=/tmp/cache"]

ADD catalog /configs

RUN ["/bin/opm", "serve", "/configs", "--cache-dir=/tmp/cache", "--cache-only"]
EOF_CATALOG_DOCKER

  return 0
}

e2e_operator_olm_build_catalog_image() {
  local run_dir
  run_dir=$(e2e_operator_olm_run_dir)
  local image
  image=$(e2e_operator_olm_catalog_image)

  e2e_info "operator profile building OLM catalog image image=${image}"
  e2e_run_cmd "${E2E_CONTAINER_ENGINE}" build \
    -f "${run_dir}/catalog.Dockerfile" \
    -t "${image}" \
    "${run_dir}" || return 1

  local archive_dir="${run_dir}/archives"
  mkdir -p "${archive_dir}" || return 1
  local catalog_archive="${archive_dir}/catalog.oci.tar"
  e2e_operator_olm_save_oci_archive "${image}" "${catalog_archive}" || return 1
  E2E_OPERATOR_OLM_CATALOG_DIGEST=$(e2e_operator_olm_read_oci_archive_digest "${catalog_archive}") || return 1
  export E2E_OPERATOR_OLM_CATALOG_DIGEST
  e2e_info "operator profile captured OLM catalog digest ${E2E_OPERATOR_OLM_CATALOG_DIGEST}"
  return 0
}

e2e_operator_olm_kind_node_containers() {
  "${E2E_CONTAINER_ENGINE}" ps --filter "label=io.x-k8s.kind.cluster=${E2E_KIND_CLUSTER_NAME}" --format '{{.Names}}'
}

e2e_operator_olm_import_oci_archive_into_nodes() {
  local archive=$1
  local image=$2
  local digest=$3
  local node
  local nodes_out
  nodes_out=$(e2e_operator_olm_kind_node_containers) || return 1
  if [[ -z "${nodes_out}" ]]; then
    e2e_die "operator profile cannot locate kind nodes for cluster ${E2E_KIND_CLUSTER_NAME}"
    return 1
  fi
  local remote_archive="/tmp/$(basename -- "${archive}")"
  while IFS= read -r node; do
    [[ -n "${node}" ]] || continue
    e2e_run_cmd "${E2E_CONTAINER_ENGINE}" cp "${archive}" "${node}:${remote_archive}" || return 1
    e2e_run_cmd "${E2E_CONTAINER_ENGINE}" exec "${node}" \
      ctr --namespace=k8s.io images import --all-platforms "${remote_archive}" || return 1
    if [[ -n "${digest}" ]]; then
      local digest_ref
      digest_ref=$(e2e_operator_olm_image_digest_ref "${image}" "${digest}")
      e2e_run_cmd "${E2E_CONTAINER_ENGINE}" exec "${node}" \
        ctr --namespace=k8s.io images tag --force "${image}" "${digest_ref}" || return 1
    fi
    e2e_run_cmd "${E2E_CONTAINER_ENGINE}" exec "${node}" rm -f "${remote_archive}" || true
  done <<<"${nodes_out}"
  return 0
}

e2e_operator_olm_load_images_into_cluster() {
  local archive_dir
  archive_dir="$(e2e_operator_olm_run_dir)/archives"
  mkdir -p "${archive_dir}" || return 1

  local bundle_image catalog_image
  bundle_image=$(e2e_operator_olm_bundle_image)
  catalog_image=$(e2e_operator_olm_catalog_image)

  local bundle_archive="${archive_dir}/bundle.oci.tar"
  local catalog_archive="${archive_dir}/catalog.oci.tar"
  local manager_archive="${archive_dir}/manager.oci.tar"

  if [[ ! -f "${bundle_archive}" ]]; then
    e2e_operator_olm_save_oci_archive "${bundle_image}" "${bundle_archive}" || return 1
    E2E_OPERATOR_OLM_BUNDLE_DIGEST=$(e2e_operator_olm_read_oci_archive_digest "${bundle_archive}") || return 1
    export E2E_OPERATOR_OLM_BUNDLE_DIGEST
  fi
  if [[ ! -f "${catalog_archive}" ]]; then
    e2e_operator_olm_save_oci_archive "${catalog_image}" "${catalog_archive}" || return 1
    E2E_OPERATOR_OLM_CATALOG_DIGEST=$(e2e_operator_olm_read_oci_archive_digest "${catalog_archive}") || return 1
    export E2E_OPERATOR_OLM_CATALOG_DIGEST
  fi

  e2e_info "operator profile exporting OLM manager image archive dir=${archive_dir}"
  e2e_operator_olm_save_oci_archive "${E2E_OPERATOR_IMAGE}" "${manager_archive}" || return 1

  e2e_info "operator profile loading OLM image archives into kind cluster name=${E2E_KIND_CLUSTER_NAME}"
  e2e_operator_olm_import_oci_archive_into_nodes "${bundle_archive}" "${bundle_image}" "${E2E_OPERATOR_OLM_BUNDLE_DIGEST}" || return 1
  e2e_operator_olm_import_oci_archive_into_nodes "${catalog_archive}" "${catalog_image}" "${E2E_OPERATOR_OLM_CATALOG_DIGEST}" || return 1
  e2e_operator_olm_import_oci_archive_into_nodes "${manager_archive}" "${E2E_OPERATOR_IMAGE}" '' || return 1
  return 0
}

e2e_operator_olm_core_ready() {
  kubectl --kubeconfig "${E2E_KUBECONFIG}" get namespace olm >/dev/null 2>&1 || return 1

  local crd
  while IFS= read -r crd; do
    [[ -n "${crd}" ]] || continue
    kubectl --kubeconfig "${E2E_KUBECONFIG}" \
      wait --for=condition=Established "crd/${crd}" --timeout=1s >/dev/null 2>&1 || return 1
  done < <(e2e_operator_olm_core_required_crds)

  local deployment
  for deployment in olm-operator catalog-operator packageserver; do
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm \
      wait --for=condition=Available "deployment/${deployment}" --timeout=1s >/dev/null 2>&1 || return 1
  done

  return 0
}

e2e_operator_olm_wait_for_core_crds_ready() {
  local ready_timeout_seconds
  ready_timeout_seconds=$(e2e_operator_ready_timeout_seconds) || return 1

  local crd
  while IFS= read -r crd; do
    [[ -n "${crd}" ]] || continue
    e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" \
      wait --for=condition=Established "crd/${crd}" --timeout="${ready_timeout_seconds}s" || return 1
  done < <(e2e_operator_olm_core_required_crds)
  return 0
}

e2e_operator_olm_wait_for_deployment_created() {
  local namespace=$1
  local deployment=$2
  local timeout_seconds=$3
  local deadline
  deadline=$(( $(date +%s) + timeout_seconds ))

  while true; do
    if kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" \
      get "deployment/${deployment}" >/dev/null 2>&1; then
      return 0
    fi

    if (( $(date +%s) >= deadline )); then
      e2e_error "OLM deployment ${deployment} was not created within ${timeout_seconds}s"
      return 1
    fi
    sleep 1
  done
}

e2e_operator_olm_wait_for_core_deployments_ready() {
  local ready_timeout_seconds
  ready_timeout_seconds=$(e2e_operator_ready_timeout_seconds) || return 1

  local deployment
  for deployment in olm-operator catalog-operator packageserver; do
    e2e_operator_olm_wait_for_deployment_created 'olm' "${deployment}" "${ready_timeout_seconds}" || {
      kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm get pods -o wide || true
      return 1
    }
    if ! e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" -n olm \
      wait --for=condition=Available "deployment/${deployment}" --timeout="${ready_timeout_seconds}s"; then
      e2e_error "OLM deployment ${deployment} did not become Available"
      kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm describe "deployment/${deployment}" || true
      kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm get pods -o wide || true
      return 1
    fi
  done
  return 0
}

e2e_operator_olm_delete_default_catalogs() {
  if ! kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm \
    get catalogsource/operatorhubio-catalog >/dev/null 2>&1; then
    return 0
  fi

  e2e_info 'operator profile deleting upstream default OLM catalogsource operatorhubio-catalog'
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" -n olm \
    delete catalogsource/operatorhubio-catalog --ignore-not-found || return 1
  return 0
}

e2e_operator_olm_install_core() {
  if e2e_operator_olm_core_ready; then
    e2e_info 'operator profile OLM core already installed and ready; skipping'
    e2e_operator_olm_delete_default_catalogs || return 1
    return 0
  fi

  local crds_manifest runtime_manifest
  crds_manifest=$(e2e_operator_olm_core_crds_manifest_path)
  runtime_manifest=$(e2e_operator_olm_core_runtime_manifest_path)

  [[ -f "${crds_manifest}" ]] || {
    e2e_die "vendored OLM CRD manifest is unavailable: ${crds_manifest}"
    return 1
  }
  [[ -f "${runtime_manifest}" ]] || {
    e2e_die "vendored OLM runtime manifest is unavailable: ${runtime_manifest}"
    return 1
  }

  e2e_info "operator profile installing OLM core from vendored YAML manifests dir=$(e2e_operator_olm_core_dir)"
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply --server-side=true -f "${crds_manifest}" || return 1
  e2e_operator_olm_wait_for_core_crds_ready || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${runtime_manifest}" || return 1
  e2e_operator_olm_wait_for_core_deployments_ready || return 1
  e2e_operator_olm_delete_default_catalogs || return 1
  return 0
}

e2e_operator_olm_write_install_manifest() {
  local manifest_path catalog_manifest_path subscription_manifest_path
  manifest_path=$(e2e_operator_olm_install_manifest_path)
  catalog_manifest_path=$(e2e_operator_olm_catalog_manifest_path)
  subscription_manifest_path=$(e2e_operator_olm_subscription_manifest_path)
  local namespace
  namespace=$(e2e_operator_effective_namespace)
  local run_label
  run_label=$(e2e_operator_run_label_value)
  local catalog_name
  catalog_name=$(e2e_operator_olm_catalog_source_name)
  local operator_group_name
  operator_group_name=$(e2e_operator_olm_operator_group_name)
  local subscription_name
  subscription_name=$(e2e_operator_olm_subscription_name)
  local catalog_image
  catalog_image=$(e2e_operator_olm_catalog_image_ref)

  mkdir -p -- "$(dirname -- "${manifest_path}")" || return 1

  cat >"${catalog_manifest_path}" <<EOF_OLM_CATALOG
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
  labels:
    app.kubernetes.io/name: declarest-operator
    declarest.e2e/run-id: ${run_label}
---
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ${catalog_name}
  namespace: olm
  labels:
    declarest.e2e/run-id: ${run_label}
spec:
  sourceType: grpc
  image: ${catalog_image}
  displayName: DeclaREST Operator (E2E)
  publisher: DeclaREST E2E
EOF_OLM_CATALOG

  : >"${subscription_manifest_path}" || return 1
  if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE:-}" && -f "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}" ]]; then
    local secret_name
    secret_name=$(e2e_operator_managed_service_metadata_bundle_secret_name)
    local encoded
    encoded=$(base64 < "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}" | tr -d '\n')
    cat >"${subscription_manifest_path}" <<EOF_METADATA_SECRET
apiVersion: v1
kind: Secret
metadata:
  name: ${secret_name}
  namespace: ${namespace}
  labels:
    declarest.e2e/run-id: ${run_label}
type: Opaque
data:
  metadata-bundle.tar.gz: ${encoded}
---
EOF_METADATA_SECRET
  fi

  cat >>"${subscription_manifest_path}" <<EOF_OLM_SUBSCRIPTION
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: ${operator_group_name}
  namespace: ${namespace}
  labels:
    declarest.e2e/run-id: ${run_label}
spec:
  targetNamespaces:
    - ${namespace}
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ${subscription_name}
  namespace: ${namespace}
  labels:
    declarest.e2e/run-id: ${run_label}
spec:
  channel: alpha
  name: declarest-operator
  source: ${catalog_name}
  sourceNamespace: olm
  installPlanApproval: Automatic
EOF_OLM_SUBSCRIPTION

  {
    cat "${catalog_manifest_path}"
    printf '\n---\n'
    cat "${subscription_manifest_path}"
  } >"${manifest_path}" || return 1

  return 0
}

e2e_operator_olm_wait_for_catalog_source_ready() {
  local ready_timeout_seconds
  ready_timeout_seconds=$(e2e_operator_ready_timeout_seconds) || return 1
  local deadline=$(( $(date +%s) + ready_timeout_seconds ))
  local catalog_name
  catalog_name=$(e2e_operator_olm_catalog_source_name)
  e2e_info "operator profile waiting for OLM catalog READY catalog=${catalog_name} timeout=${ready_timeout_seconds}s"
  while (( $(date +%s) < deadline )); do
    local state
    state=$(
      kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm get "catalogsource/${catalog_name}" \
        -o jsonpath='{.status.connectionState.lastObservedState}' 2>/dev/null || true
    )
    if [[ "${state}" == 'READY' ]]; then
      return 0
    fi
    sleep 2
  done
  e2e_error "operator profile OLM catalog ${catalog_name} did not reach READY within ${ready_timeout_seconds}s"
  kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm describe "catalogsource/${catalog_name}" || true
  kubectl --kubeconfig "${E2E_KUBECONFIG}" -n olm get pods -o wide || true
  return 1
}

e2e_operator_olm_apply_install_manifest() {
  local manifest_path catalog_manifest_path subscription_manifest_path
  manifest_path=$(e2e_operator_olm_install_manifest_path)
  catalog_manifest_path=$(e2e_operator_olm_catalog_manifest_path)
  subscription_manifest_path=$(e2e_operator_olm_subscription_manifest_path)
  if [[ ! -f "${manifest_path}" ]]; then
    e2e_die "operator OLM install manifest is unavailable: ${manifest_path}"
    return 1
  fi
  if [[ ! -f "${catalog_manifest_path}" ]]; then
    e2e_die "operator OLM catalog manifest is unavailable: ${catalog_manifest_path}"
    return 1
  fi
  if [[ ! -f "${subscription_manifest_path}" ]]; then
    e2e_die "operator OLM subscription manifest is unavailable: ${subscription_manifest_path}"
    return 1
  fi

  e2e_info "operator profile applying OLM catalog manifest=${catalog_manifest_path}"
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${catalog_manifest_path}" || return 1
  e2e_operator_olm_wait_for_catalog_source_ready || return 1

  e2e_info "operator profile applying OLM subscription manifest=${subscription_manifest_path}"
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${subscription_manifest_path}" || return 1
  return 0
}

e2e_operator_olm_wait_for_csv_succeeded() {
  local namespace
  namespace=$(e2e_operator_effective_namespace)
  local subscription_name
  subscription_name=$(e2e_operator_olm_subscription_name)
  local ready_timeout_seconds
  ready_timeout_seconds=$(e2e_operator_ready_timeout_seconds) || return 1
  local start_ts
  start_ts=$(date +%s)
  local deadline=$((start_ts + ready_timeout_seconds))
  local csv_name=''

  e2e_info "operator profile waiting for CSV Succeeded namespace=${namespace} subscription=${subscription_name} timeout=${ready_timeout_seconds}s"
  while (( $(date +%s) < deadline )); do
    csv_name=$(kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" \
      get subscription "${subscription_name}" \
      -o jsonpath='{.status.installedCSV}' 2>/dev/null || true)
    if [[ -n "${csv_name}" ]]; then
      break
    fi
    sleep 2
  done

  if [[ -z "${csv_name}" ]]; then
    e2e_error "operator subscription ${subscription_name} did not resolve to an installed CSV within ${ready_timeout_seconds}s"
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" describe "subscription/${subscription_name}" || true
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" get installplans,csv || true
    return 1
  fi

  local remaining=$(( deadline - $(date +%s) ))
  if (( remaining <= 0 )); then
    remaining=1
  fi

  if ! kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" \
    wait --for=jsonpath='{.status.phase}=Succeeded' "csv/${csv_name}" \
    --timeout="${remaining}s"; then
    e2e_error "operator CSV ${csv_name} did not reach Succeeded within timeout"
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" describe "csv/${csv_name}" || true
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" describe "subscription/${subscription_name}" || true
    return 1
  fi

  E2E_OPERATOR_CSV_NAME="${csv_name}"
  export E2E_OPERATOR_CSV_NAME
  return 0
}

e2e_operator_olm_wait_for_manager_deployment_ready() {
  local namespace
  namespace=$(e2e_operator_effective_namespace)
  local ready_timeout_seconds
  ready_timeout_seconds=$(e2e_operator_ready_timeout_seconds) || return 1
  local deployment_name='declarest-operator'

  e2e_info "operator profile waiting for OLM-managed Deployment Available namespace=${namespace} deployment=${deployment_name} timeout=${ready_timeout_seconds}s"
  if ! e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" \
    wait --for=condition=Available "deployment/${deployment_name}" --timeout="${ready_timeout_seconds}s"; then
    e2e_error "operator Deployment ${deployment_name} did not become Available"
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" describe "deployment/${deployment_name}" || true
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" get pods -o wide || true
    return 1
  fi

  return 0
}

e2e_operator_olm_record_runtime_state() {
  local namespace
  namespace=$(e2e_operator_effective_namespace)
  local deployment_name='declarest-operator'
  local webhook_service_name
  webhook_service_name=$(e2e_operator_repository_webhook_service_name)
  local runtime_root='/var/lib/declarest'
  local repo_root="${runtime_root}/repos"
  local cache_root="${runtime_root}/cache"

  E2E_OPERATOR_MANAGER_DEPLOYMENT="${deployment_name}"
  E2E_OPERATOR_MANAGER_POD=$(
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" \
      get pod -l 'app.kubernetes.io/name=declarest-operator' \
      -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true
  )
  E2E_OPERATOR_MANAGER_PID=''
  E2E_OPERATOR_MANAGER_LOG_FILE="kubectl --kubeconfig ${E2E_KUBECONFIG} -n ${namespace} logs deployment/${deployment_name}"
  E2E_OPERATOR_NAMESPACE="${namespace}"
  export E2E_OPERATOR_MANAGER_DEPLOYMENT
  export E2E_OPERATOR_MANAGER_POD
  export E2E_OPERATOR_MANAGER_PID
  export E2E_OPERATOR_MANAGER_LOG_FILE
  export E2E_OPERATOR_NAMESPACE

  e2e_runtime_state_set 'OPERATOR_IMAGE' "${E2E_OPERATOR_IMAGE}" || return 1
  e2e_runtime_state_set 'OPERATOR_BUNDLE_IMAGE' "$(e2e_operator_olm_bundle_image)" || return 1
  e2e_runtime_state_set 'OPERATOR_CATALOG_IMAGE' "$(e2e_operator_olm_catalog_image)" || return 1
  e2e_runtime_state_set 'OPERATOR_NAMESPACE' "${namespace}" || return 1
  e2e_runtime_state_set 'OPERATOR_REPOSITORY_WEBHOOK_SERVICE_NAME' "${webhook_service_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_MANAGER_DEPLOYMENT' "${deployment_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_MANAGER_LOG_FILE' "${E2E_OPERATOR_MANAGER_LOG_FILE}" || return 1
  if [[ -n "${E2E_OPERATOR_CSV_NAME:-}" ]]; then
    e2e_runtime_state_set 'OPERATOR_CSV_NAME' "${E2E_OPERATOR_CSV_NAME}" || return 1
  fi
  e2e_runtime_state_set 'OPERATOR_CATALOG_SOURCE_NAME' "$(e2e_operator_olm_catalog_source_name)" || return 1
  e2e_runtime_state_set 'OPERATOR_OPERATOR_GROUP_NAME' "$(e2e_operator_olm_operator_group_name)" || return 1
  e2e_runtime_state_set 'OPERATOR_SUBSCRIPTION_NAME' "$(e2e_operator_olm_subscription_name)" || return 1
  e2e_runtime_state_set 'OPERATOR_OLM_INSTALL_MANIFEST' "$(e2e_operator_olm_install_manifest_path)" || return 1
  e2e_runtime_state_set 'OPERATOR_OLM_CATALOG_MANIFEST' "$(e2e_operator_olm_catalog_manifest_path)" || return 1
  e2e_runtime_state_set 'OPERATOR_OLM_SUBSCRIPTION_MANIFEST' "$(e2e_operator_olm_subscription_manifest_path)" || return 1
  e2e_runtime_state_set 'OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_SECRET_NAME' "$(e2e_operator_managed_service_metadata_bundle_secret_name)" || return 1
  if [[ -n "${E2E_OPERATOR_MANAGER_POD}" ]]; then
    e2e_runtime_state_set 'OPERATOR_MANAGER_POD' "${E2E_OPERATOR_MANAGER_POD}" || return 1
  fi
  if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE:-}" ]]; then
    e2e_runtime_state_set 'OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE' "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}" || return 1
  fi
  if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH:-}" ]]; then
    e2e_runtime_state_set 'OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH' "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH}" || return 1
  fi
  e2e_runtime_state_set 'OPERATOR_REPO_BASE_DIR' "${repo_root}" || return 1
  e2e_runtime_state_set 'OPERATOR_CACHE_BASE_DIR' "${cache_root}" || return 1
  return 0
}

e2e_operator_olm_cluster_reused_truthy() {
  local value=${1:-}
  case "${value,,}" in
    1|true|yes|on)
      return 0
      ;;
  esac
  return 1
}

e2e_operator_olm_delete_namespaced_resource() {
  local kubeconfig=$1
  local namespace=$2
  local resource=$3
  local name=$4

  [[ -n "${name}" ]] || return 0
  if ! kubectl --kubeconfig "${kubeconfig}" -n "${namespace}" \
    delete "${resource}/${name}" --ignore-not-found >/dev/null 2>&1; then
    e2e_warn "failed to delete ${resource}/${name} in namespace=${namespace}"
    return 1
  fi
  return 0
}

e2e_operator_olm_cleanup_resources() {
  local kubeconfig=$1
  local namespace=$2
  local catalog_name=$3
  local subscription_name=$4
  local operator_group_name=$5
  local csv_name=$6
  local metadata_secret_name=$7
  local subscription_manifest=$8
  local failed=0

  [[ -n "${kubeconfig}" && -f "${kubeconfig}" ]] || return 0
  if ! kubectl --kubeconfig "${kubeconfig}" get namespace olm >/dev/null 2>&1; then
    return 0
  fi

  if [[ -f "${subscription_manifest}" ]]; then
    e2e_info "cleanup deleting run-scoped operator OLM subscription resources manifest=${subscription_manifest}"
    kubectl --kubeconfig "${kubeconfig}" delete -f "${subscription_manifest}" --ignore-not-found >/dev/null 2>&1 || failed=1
  else
    e2e_operator_olm_delete_namespaced_resource "${kubeconfig}" "${namespace}" subscription "${subscription_name}" || failed=1
    e2e_operator_olm_delete_namespaced_resource "${kubeconfig}" "${namespace}" operatorgroup "${operator_group_name}" || failed=1
    e2e_operator_olm_delete_namespaced_resource "${kubeconfig}" "${namespace}" secret "${metadata_secret_name}" || failed=1
  fi

  e2e_operator_olm_delete_namespaced_resource "${kubeconfig}" "${namespace}" csv "${csv_name}" || failed=1
  e2e_operator_olm_delete_namespaced_resource "${kubeconfig}" olm catalogsource "${catalog_name}" || failed=1

  if ((failed == 1)); then
    return 1
  fi
  return 0
}

e2e_operator_olm_cleanup_current_run() {
  local cluster_reused=${E2E_KIND_CLUSTER_REUSED:-}
  if [[ -z "${cluster_reused}" && -n "${E2E_STATE_DIR:-}" ]]; then
    local runtime_state
    runtime_state=$(e2e_runtime_state_file)
    cluster_reused=$(e2e_state_get "${runtime_state}" 'KIND_CLUSTER_REUSED' || true)
  fi

  e2e_operator_olm_cluster_reused_truthy "${cluster_reused}" || return 0

  local namespace
  namespace=$(e2e_operator_effective_namespace)
  local csv_name=${E2E_OPERATOR_CSV_NAME:-}
  if [[ -z "${csv_name}" && -n "${E2E_STATE_DIR:-}" ]]; then
    local runtime_state
    runtime_state=$(e2e_runtime_state_file)
    csv_name=$(e2e_state_get "${runtime_state}" 'OPERATOR_CSV_NAME' || true)
  fi

  e2e_operator_olm_cleanup_resources \
    "${E2E_KUBECONFIG:-}" \
    "${namespace}" \
    "$(e2e_operator_olm_catalog_source_name)" \
    "$(e2e_operator_olm_subscription_name)" \
    "$(e2e_operator_olm_operator_group_name)" \
    "${csv_name}" \
    "$(e2e_operator_managed_service_metadata_bundle_secret_name)" \
    "$(e2e_operator_olm_subscription_manifest_path)" || return 1
  return 0
}

e2e_operator_cleanup_olm_for_run_id() {
  local run_id=$1
  local runtime_state="${E2E_RUNS_DIR}/${run_id}/state/runtime.env"
  [[ -f "${runtime_state}" ]] || return 0

  local cluster_reused
  cluster_reused=$(e2e_state_get "${runtime_state}" 'KIND_CLUSTER_REUSED' || true)
  e2e_operator_olm_cluster_reused_truthy "${cluster_reused}" || return 0

  local run_dir="${E2E_RUNS_DIR}/${run_id}"
  local kubeconfig namespace catalog_name subscription_name operator_group_name csv_name metadata_secret_name subscription_manifest
  kubeconfig=$(e2e_state_get "${runtime_state}" 'KUBECONFIG_PATH' || true)
  namespace=$(e2e_state_get "${runtime_state}" 'OPERATOR_NAMESPACE' || true)
  if [[ -z "${namespace}" ]]; then
    namespace=$(e2e_state_get "${runtime_state}" 'K8S_NAMESPACE' || true)
  fi
  catalog_name=$(e2e_state_get "${runtime_state}" 'OPERATOR_CATALOG_SOURCE_NAME' || true)
  subscription_name=$(e2e_state_get "${runtime_state}" 'OPERATOR_SUBSCRIPTION_NAME' || true)
  operator_group_name=$(e2e_state_get "${runtime_state}" 'OPERATOR_OPERATOR_GROUP_NAME' || true)
  csv_name=$(e2e_state_get "${runtime_state}" 'OPERATOR_CSV_NAME' || true)
  metadata_secret_name=$(e2e_state_get "${runtime_state}" 'OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_SECRET_NAME' || true)
  subscription_manifest=$(e2e_state_get "${runtime_state}" 'OPERATOR_OLM_SUBSCRIPTION_MANIFEST' || true)

  local previous_run_id=${E2E_RUN_ID:-}
  E2E_RUN_ID="${run_id}"
  if [[ -z "${namespace}" ]]; then
    namespace=$(e2e_k8s_namespace_for_run "${run_id}")
  fi
  if [[ -z "${catalog_name}" ]]; then
    catalog_name=$(e2e_operator_olm_catalog_source_name)
  fi
  if [[ -z "${subscription_name}" ]]; then
    subscription_name=$(e2e_operator_olm_subscription_name)
  fi
  if [[ -z "${operator_group_name}" ]]; then
    operator_group_name=$(e2e_operator_olm_operator_group_name)
  fi
  if [[ -z "${metadata_secret_name}" ]]; then
    metadata_secret_name=$(e2e_operator_managed_service_metadata_bundle_secret_name)
  fi
  if [[ -n "${previous_run_id}" ]]; then
    E2E_RUN_ID="${previous_run_id}"
  else
    unset E2E_RUN_ID
  fi
  if [[ -z "${subscription_manifest}" ]]; then
    subscription_manifest="${run_dir}/operator/olm/olm-subscription.yaml"
  fi

  e2e_operator_olm_cleanup_resources \
    "${kubeconfig}" \
    "${namespace}" \
    "${catalog_name}" \
    "${subscription_name}" \
    "${operator_group_name}" \
    "${csv_name}" \
    "${metadata_secret_name}" \
    "${subscription_manifest}" || return 1
  return 0
}

e2e_operator_install_via_olm() {
  if [[ -z "${E2E_OPERATOR_IMAGE:-}" ]]; then
    e2e_die "operator manager image is unavailable: ${E2E_OPERATOR_IMAGE:-<unset>}"
    return 1
  fi
  if [[ -z "${E2E_KIND_CLUSTER_NAME:-}" ]]; then
    e2e_die 'operator profile kind cluster metadata is missing'
    return 1
  fi

  e2e_operator_prepare_managed_service_metadata_bundle || return 1
  e2e_operator_olm_prepare_bundle_tree || return 1
  e2e_operator_olm_build_bundle_image || return 1
  e2e_operator_olm_prepare_catalog_tree || return 1
  e2e_operator_olm_build_catalog_image || return 1
  e2e_operator_olm_load_images_into_cluster || return 1
  e2e_operator_olm_install_core || return 1
  e2e_operator_olm_write_install_manifest || return 1
  e2e_operator_olm_apply_install_manifest || return 1
  e2e_operator_olm_wait_for_csv_succeeded || return 1
  e2e_operator_olm_wait_for_manager_deployment_ready || return 1
  e2e_operator_olm_record_runtime_state || return 1
  return 0
}

e2e_operator_collect_managed_service_config() {
  local state_file=$1
  local managed_service_key

  # shellcheck disable=SC1090
  source "${state_file}"

  managed_service_key=$(e2e_component_key 'managed-service' "${E2E_MANAGED_SERVICE}")

  E2E_OPERATOR_MANAGED_SERVICE_BASE_URL=${MANAGED_SERVICE_BASE_URL:-}
  E2E_OPERATOR_MANAGED_SERVICE_AUTH_KIND=${MANAGED_SERVICE_AUTH_KIND:-}
  E2E_OPERATOR_MANAGED_SERVICE_TOKEN_URL=${MANAGED_SERVICE_TOKEN_URL:-}
  E2E_OPERATOR_MANAGED_SERVICE_OAUTH_SCOPE=${MANAGED_SERVICE_OAUTH_SCOPE:-}
  E2E_OPERATOR_MANAGED_SERVICE_OAUTH_AUDIENCE=${MANAGED_SERVICE_OAUTH_AUDIENCE:-}
  E2E_OPERATOR_MANAGED_SERVICE_BASIC_USERNAME=${MANAGED_SERVICE_BASIC_USERNAME:-}
  E2E_OPERATOR_MANAGED_SERVICE_BASIC_PASSWORD=${MANAGED_SERVICE_BASIC_PASSWORD:-}
  E2E_OPERATOR_MANAGED_SERVICE_HEADER_NAME=${MANAGED_SERVICE_HEADER_NAME:-}
  E2E_OPERATOR_MANAGED_SERVICE_HEADER_PREFIX=${MANAGED_SERVICE_HEADER_PREFIX:-}
  E2E_OPERATOR_MANAGED_SERVICE_HEADER_VALUE=${MANAGED_SERVICE_HEADER_VALUE:-}
  E2E_OPERATOR_MANAGED_SERVICE_OAUTH_CLIENT_ID=${MANAGED_SERVICE_OAUTH_CLIENT_ID:-}
  E2E_OPERATOR_MANAGED_SERVICE_OAUTH_CLIENT_SECRET=${MANAGED_SERVICE_OAUTH_CLIENT_SECRET:-}

  if [[ "${E2E_PLATFORM:-}" == 'kubernetes' && "${E2E_MANAGED_SERVICE_CONNECTION:-}" == 'local' ]]; then
    E2E_OPERATOR_MANAGED_SERVICE_BASE_URL=$(
      e2e_operator_rewrite_local_url_to_component_service \
        "${E2E_OPERATOR_MANAGED_SERVICE_BASE_URL}" \
        "${managed_service_key}"
    )
    if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_TOKEN_URL}" ]]; then
      E2E_OPERATOR_MANAGED_SERVICE_TOKEN_URL=$(
        e2e_operator_rewrite_local_url_to_component_service \
          "${E2E_OPERATOR_MANAGED_SERVICE_TOKEN_URL}" \
          "${managed_service_key}"
      )
    fi
  fi

  if [[ -z "${E2E_OPERATOR_MANAGED_SERVICE_BASE_URL}" ]]; then
    e2e_die 'operator profile managed-service base URL is empty after component setup'
    return 1
  fi

  case "${E2E_OPERATOR_MANAGED_SERVICE_AUTH_KIND}" in
    oauth2)
      [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_TOKEN_URL}" ]] || {
        e2e_die 'operator profile managed-service oauth2 token URL is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_CLIENT_ID}" ]] || {
        e2e_die 'operator profile managed-service oauth2 client id is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_CLIENT_SECRET}" ]] || {
        e2e_die 'operator profile managed-service oauth2 client secret is empty'
        return 1
      }
      ;;
    basic)
      [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_BASIC_USERNAME}" ]] || {
        e2e_die 'operator profile managed-service basic username is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_BASIC_PASSWORD}" ]] || {
        e2e_die 'operator profile managed-service basic password is empty'
        return 1
      }
      ;;
    custom-header)
      [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_HEADER_NAME}" ]] || {
        e2e_die 'operator profile managed-service custom header name is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_HEADER_VALUE}" ]] || {
        e2e_die 'operator profile managed-service custom header value is empty'
        return 1
      }
      ;;
    *)
      e2e_die 'operator profile managed-service auth mode is unresolved'
      return 1
      ;;
  esac

  return 0
}

e2e_operator_write_manifests() {
  local manifest_dir
  manifest_dir=$(e2e_operator_manifest_dir)
  mkdir -p "${manifest_dir}" || return 1

  local namespace
  namespace=$(e2e_operator_effective_namespace)

  local repo_key
  repo_key=$(e2e_component_key 'repo-type' "${E2E_REPO_TYPE}")
  local repo_state_file
  repo_state_file=$(e2e_component_state_file "${repo_key}")

  local managed_service_key
  managed_service_key=$(e2e_component_key 'managed-service' "${E2E_MANAGED_SERVICE}")
  local managed_service_state_file
  managed_service_state_file=$(e2e_component_state_file "${managed_service_key}")

  local secret_store_key
  secret_store_key=$(e2e_component_key 'secret-provider' "${E2E_SECRET_PROVIDER}")
  local secret_store_state_file
  secret_store_state_file=$(e2e_component_state_file "${secret_store_key}")

  [[ -f "${repo_state_file}" ]] || {
    e2e_die "operator profile missing repository state: ${repo_state_file}"
    return 1
  }
  [[ -f "${managed_service_state_file}" ]] || {
    e2e_die "operator profile missing managed-service state: ${managed_service_state_file}"
    return 1
  }
  [[ -f "${secret_store_state_file}" ]] || {
    e2e_die "operator profile missing secret-store state: ${secret_store_state_file}"
    return 1
  }

  # shellcheck disable=SC1090
  source "${repo_state_file}"

  local repository_name
  repository_name=${E2E_OPERATOR_REPOSITORY_NAME:-$(e2e_operator_scoped_name 'declarest-e2e-repository')}
  local managed_service_name
  managed_service_name=$(e2e_operator_scoped_name 'declarest-e2e-managed-service')
  local secret_store_name
  secret_store_name=$(e2e_operator_scoped_name 'declarest-e2e-secret-store')
  local sync_policy_name
  sync_policy_name=$(e2e_operator_scoped_name 'declarest-e2e-sync-policy')

  local repo_secret_name
  repo_secret_name=$(e2e_operator_scoped_name 'declarest-e2e-repo-auth')
  local managed_service_secret_name
  managed_service_secret_name=$(e2e_operator_scoped_name 'declarest-e2e-managed-service-auth')
  local secret_store_secret_name
  secret_store_secret_name=$(e2e_operator_scoped_name 'declarest-e2e-secret-store-auth')

  local repo_url=${GIT_REMOTE_URL:-}
  local repo_branch=${GIT_REMOTE_BRANCH:-main}
  [[ -n "${repo_url}" ]] || {
    e2e_die 'operator profile repository URL is empty'
    return 1
  }
  repo_url=$(e2e_operator_rewrite_repo_url_for_cluster "${repo_url}") || return 1

  local repo_token=''
  case "${GIT_AUTH_MODE:-}" in
    basic)
      repo_url=$(e2e_operator_repo_url_with_username "${repo_url}" "${GIT_AUTH_USERNAME:-}")
      repo_token=${GIT_AUTH_PASSWORD:-}
      ;;
    access-key)
      repo_token=${GIT_AUTH_TOKEN:-}
      ;;
    '')
      repo_token='declarest-e2e-token-placeholder'
      ;;
    *)
      e2e_die "operator profile unsupported git auth mode: ${GIT_AUTH_MODE}"
      return 1
      ;;
  esac
  [[ -n "${repo_token}" ]] || {
    e2e_die 'operator profile repository token/password is empty'
    return 1
  }
  local repo_webhook_provider
  local repo_webhook_secret
  repo_webhook_provider=${E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER:-}
  repo_webhook_secret=${E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET:-}
  if [[ -n "${repo_webhook_provider}" && -z "${repo_webhook_secret}" ]]; then
    e2e_die 'operator profile repository webhook provider is set but webhook secret is empty'
    return 1
  fi

  cat >"${manifest_dir}/secret-repository-auth.yaml" <<EOF_REPO_SECRET
apiVersion: v1
kind: Secret
metadata:
  name: ${repo_secret_name}
  namespace: ${namespace}
type: Opaque
data:
  token: $(e2e_operator_b64 "${repo_token}")
EOF_REPO_SECRET

  if [[ -n "${repo_webhook_provider}" ]]; then
    cat >>"${manifest_dir}/secret-repository-auth.yaml" <<EOF_REPO_WEBHOOK_SECRET
  webhook-secret: $(e2e_operator_b64 "${repo_webhook_secret}")
EOF_REPO_WEBHOOK_SECRET
  fi

  cat >"${manifest_dir}/resource-repository.yaml" <<EOF_REPO_CR
apiVersion: declarest.io/v1alpha1
kind: ResourceRepository
metadata:
  name: ${repository_name}
  namespace: ${namespace}
spec:
  type: git
  pollInterval: 30s
  git:
    url: $(e2e_operator_yaml_quote "${repo_url}")
    branch: $(e2e_operator_yaml_quote "${repo_branch}")
    auth:
      tokenRef:
        name: ${repo_secret_name}
        key: token
EOF_REPO_CR

  if [[ -n "${repo_webhook_provider}" ]]; then
    cat >>"${manifest_dir}/resource-repository.yaml" <<EOF_REPO_WEBHOOK
    webhook:
      provider: ${repo_webhook_provider}
      secretRef:
        name: ${repo_secret_name}
        key: webhook-secret
EOF_REPO_WEBHOOK
  fi

  cat >>"${manifest_dir}/resource-repository.yaml" <<EOF_REPO_CR_FOOTER
  storage:
    pvc:
      accessModes:
        - ReadWriteOnce
      requests:
        storage: 1Gi
EOF_REPO_CR_FOOTER

  e2e_operator_collect_managed_service_config "${managed_service_state_file}" || return 1

  local managed_service_tls_enabled='false'
  local managed_service_tls_insecure_skip_verify='false'
  local tls_ca_file=''
  local tls_client_cert_file=''
  local tls_client_key_file=''
  local metadata_bundle_ref="${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH:-}"
  if [[ -z "${metadata_bundle_ref}" ]]; then
    metadata_bundle_ref="${E2E_METADATA_BUNDLE:-}"
  fi
  if [[ "${MANAGED_SERVICE_TLS_ENABLED:-false}" == 'true' ]]; then
    managed_service_tls_enabled='true'
    if [[ "${E2E_PLATFORM:-}" == 'kubernetes' && "${MANAGED_SERVICE_TLS_INSECURE_SKIP_VERIFY_FOR_CLUSTER:-false}" == 'true' ]]; then
      managed_service_tls_insecure_skip_verify='true'
    fi
    tls_ca_file=${MANAGED_SERVICE_TLS_CA_CERT_FILE_HOST:-}
    tls_client_cert_file=${MANAGED_SERVICE_TLS_CLIENT_CERT_FILE_HOST:-}
    tls_client_key_file=${MANAGED_SERVICE_TLS_CLIENT_KEY_FILE_HOST:-}
    [[ -f "${tls_ca_file}" ]] || {
      e2e_die "operator profile missing mTLS CA certificate file: ${tls_ca_file}"
      return 1
    }
    [[ -f "${tls_client_cert_file}" ]] || {
      e2e_die "operator profile missing mTLS client certificate file: ${tls_client_cert_file}"
      return 1
    }
    [[ -f "${tls_client_key_file}" ]] || {
      e2e_die "operator profile missing mTLS client key file: ${tls_client_key_file}"
      return 1
    }
  fi

  {
    printf 'apiVersion: v1\n'
    printf 'kind: Secret\n'
    printf 'metadata:\n'
    printf '  name: %s\n' "${managed_service_secret_name}"
    printf '  namespace: %s\n' "${namespace}"
    printf 'type: Opaque\n'
    printf 'data:\n'
    case "${E2E_OPERATOR_MANAGED_SERVICE_AUTH_KIND}" in
      oauth2)
        printf '  client-id: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_CLIENT_ID}")"
        printf '  client-secret: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_CLIENT_SECRET}")"
        ;;
      basic)
        printf '  username: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVICE_BASIC_USERNAME}")"
        printf '  password: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVICE_BASIC_PASSWORD}")"
        ;;
      custom-header)
        printf '  header-value: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVICE_HEADER_VALUE}")"
        ;;
    esac
    if [[ "${managed_service_tls_enabled}" == 'true' ]]; then
      printf '  ca-cert: %s\n' "$(e2e_operator_b64 "$(cat "${tls_ca_file}")")"
      printf '  client-cert: %s\n' "$(e2e_operator_b64 "$(cat "${tls_client_cert_file}")")"
      printf '  client-key: %s\n' "$(e2e_operator_b64 "$(cat "${tls_client_key_file}")")"
    fi
  } >"${manifest_dir}/secret-managed-service-auth.yaml"

  {
    printf 'apiVersion: declarest.io/v1alpha1\n'
    printf 'kind: ManagedService\n'
    printf 'metadata:\n'
    printf '  name: %s\n' "${managed_service_name}"
    printf '  namespace: %s\n' "${namespace}"
    printf 'spec:\n'
    printf '  http:\n'
    printf '    baseURL: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVICE_BASE_URL}")"
    printf '    auth:\n'
    case "${E2E_OPERATOR_MANAGED_SERVICE_AUTH_KIND}" in
      oauth2)
        printf '      oauth2:\n'
        printf '        tokenURL: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVICE_TOKEN_URL}")"
        printf '        grantType: client_credentials\n'
        printf '        clientIDRef:\n'
        printf '          name: %s\n' "${managed_service_secret_name}"
        printf '          key: client-id\n'
        printf '        clientSecretRef:\n'
        printf '          name: %s\n' "${managed_service_secret_name}"
        printf '          key: client-secret\n'
        if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_SCOPE}" ]]; then
          printf '        scope: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_SCOPE}")"
        fi
        if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_AUDIENCE}" ]]; then
          printf '        audience: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVICE_OAUTH_AUDIENCE}")"
        fi
        ;;
      basic)
        printf '      basicAuth:\n'
        printf '        usernameRef:\n'
        printf '          name: %s\n' "${managed_service_secret_name}"
        printf '          key: username\n'
        printf '        passwordRef:\n'
        printf '          name: %s\n' "${managed_service_secret_name}"
        printf '          key: password\n'
        ;;
      custom-header)
        printf '      customHeaders:\n'
        printf '        - header: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVICE_HEADER_NAME}")"
        if [[ -n "${E2E_OPERATOR_MANAGED_SERVICE_HEADER_PREFIX}" ]]; then
          printf '          prefix: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVICE_HEADER_PREFIX}")"
        fi
        printf '          valueRef:\n'
        printf '            name: %s\n' "${managed_service_secret_name}"
        printf '            key: header-value\n'
        ;;
    esac
    if [[ "${managed_service_tls_enabled}" == 'true' ]]; then
      printf '    tls:\n'
      printf '      caCertRef:\n'
      printf '        name: %s\n' "${managed_service_secret_name}"
      printf '        key: ca-cert\n'
      printf '      clientCertRef:\n'
      printf '        name: %s\n' "${managed_service_secret_name}"
      printf '        key: client-cert\n'
      printf '      clientKeyRef:\n'
      printf '        name: %s\n' "${managed_service_secret_name}"
      printf '        key: client-key\n'
      if [[ "${managed_service_tls_insecure_skip_verify}" == 'true' ]]; then
        printf '      insecureSkipVerify: true\n'
      fi
    fi
    if [[ -n "${metadata_bundle_ref}" ]]; then
      printf '  metadata:\n'
      printf '    bundle: %s\n' "$(e2e_operator_yaml_quote "${metadata_bundle_ref}")"
    fi
  } >"${manifest_dir}/managed-service.yaml"

  # shellcheck disable=SC1090
  source "${secret_store_state_file}"

  case "${E2E_SECRET_PROVIDER}" in
    file)
      local secret_file_path=${SECRET_FILE_PATH:-}
      if [[ "${E2E_PLATFORM:-}" == 'kubernetes' ]]; then
        secret_file_path='/var/lib/declarest/secrets/declarest-e2e-secrets.enc.json'
      fi

      [[ -n "${secret_file_path}" ]] || {
        e2e_die 'operator profile file secret-store path is empty'
        return 1
      }
      [[ -n "${SECRET_FILE_PASSPHRASE:-}" ]] || {
        e2e_die 'operator profile file secret-store passphrase is empty'
        return 1
      }
      cat >"${manifest_dir}/secret-secret-store-auth.yaml" <<EOF_FILE_SECRET
apiVersion: v1
kind: Secret
metadata:
  name: ${secret_store_secret_name}
  namespace: ${namespace}
type: Opaque
data:
  passphrase: $(e2e_operator_b64 "${SECRET_FILE_PASSPHRASE}")
EOF_FILE_SECRET

      cat >"${manifest_dir}/secret-store.yaml" <<EOF_FILE_STORE
apiVersion: declarest.io/v1alpha1
kind: SecretStore
metadata:
  name: ${secret_store_name}
  namespace: ${namespace}
spec:
  file:
    path: $(e2e_operator_yaml_quote "${secret_file_path}")
    storage:
      pvc:
        accessModes:
          - ReadWriteOnce
        requests:
          storage: 1Gi
    encryption:
      passphraseRef:
        name: ${secret_store_secret_name}
        key: passphrase
EOF_FILE_STORE
      ;;
    vault)
      local vault_address=${VAULT_ADDRESS:-}
      [[ -n "${vault_address}" ]] || {
        e2e_die 'operator profile vault secret-store address is empty'
        return 1
      }
      if [[ "${E2E_PLATFORM:-}" == 'kubernetes' && "${E2E_SECRET_PROVIDER_CONNECTION:-}" == 'local' ]]; then
        vault_address=$(e2e_operator_rewrite_local_url_to_service "${vault_address}" 'secret-provider-vault' '8200')
      fi
      {
        printf 'apiVersion: v1\n'
        printf 'kind: Secret\n'
        printf 'metadata:\n'
        printf '  name: %s\n' "${secret_store_secret_name}"
        printf '  namespace: %s\n' "${namespace}"
        printf 'type: Opaque\n'
        printf 'data:\n'
        case "${VAULT_AUTH_MODE:-token}" in
          token)
            [[ -n "${VAULT_TOKEN:-}" ]] || {
              e2e_die 'operator profile vault token auth selected but VAULT_TOKEN is empty'
              return 1
            }
            printf '  token: %s\n' "$(e2e_operator_b64 "${VAULT_TOKEN}")"
            ;;
          password)
            [[ -n "${VAULT_USERNAME:-}" && -n "${VAULT_PASSWORD:-}" ]] || {
              e2e_die 'operator profile vault password auth requires VAULT_USERNAME and VAULT_PASSWORD'
              return 1
            }
            printf '  username: %s\n' "$(e2e_operator_b64 "${VAULT_USERNAME}")"
            printf '  password: %s\n' "$(e2e_operator_b64 "${VAULT_PASSWORD}")"
            ;;
          approle)
            [[ -n "${VAULT_ROLE_ID:-}" && -n "${VAULT_SECRET_ID:-}" ]] || {
              e2e_die 'operator profile vault approle auth requires VAULT_ROLE_ID and VAULT_SECRET_ID'
              return 1
            }
            printf '  role-id: %s\n' "$(e2e_operator_b64 "${VAULT_ROLE_ID}")"
            printf '  secret-id: %s\n' "$(e2e_operator_b64 "${VAULT_SECRET_ID}")"
            ;;
          *)
            e2e_die "operator profile unsupported vault auth mode: ${VAULT_AUTH_MODE:-}"
            return 1
            ;;
        esac
      } >"${manifest_dir}/secret-secret-store-auth.yaml" || return 1

      {
        printf 'apiVersion: declarest.io/v1alpha1\n'
        printf 'kind: SecretStore\n'
        printf 'metadata:\n'
        printf '  name: %s\n' "${secret_store_name}"
        printf '  namespace: %s\n' "${namespace}"
        printf 'spec:\n'
        printf '  vault:\n'
        printf '    address: %s\n' "$(e2e_operator_yaml_quote "${vault_address}")"
        printf '    mount: %s\n' "$(e2e_operator_yaml_quote "${VAULT_MOUNT:-secret}")"
        printf '    pathPrefix: %s\n' "$(e2e_operator_yaml_quote "${VAULT_PATH_PREFIX:-declarest-e2e}")"
        printf '    kvVersion: %s\n' "${VAULT_KV_VERSION:-2}"
        printf '    auth:\n'
        case "${VAULT_AUTH_MODE:-token}" in
          token)
            printf '      token:\n'
            printf '        secretRef:\n'
            printf '          name: %s\n' "${secret_store_secret_name}"
            printf '          key: token\n'
            ;;
          password)
            printf '      userpass:\n'
            printf '        usernameRef:\n'
            printf '          name: %s\n' "${secret_store_secret_name}"
            printf '          key: username\n'
            printf '        passwordRef:\n'
            printf '          name: %s\n' "${secret_store_secret_name}"
            printf '          key: password\n'
            printf '        mount: %s\n' "$(e2e_operator_yaml_quote "${VAULT_AUTH_MOUNT:-userpass}")"
            ;;
          approle)
            printf '      appRole:\n'
            printf '        roleIDRef:\n'
            printf '          name: %s\n' "${secret_store_secret_name}"
            printf '          key: role-id\n'
            printf '        secretIDRef:\n'
            printf '          name: %s\n' "${secret_store_secret_name}"
            printf '          key: secret-id\n'
            printf '        mount: %s\n' "$(e2e_operator_yaml_quote "${VAULT_AUTH_MOUNT:-approle}")"
            ;;
        esac
      } >"${manifest_dir}/secret-store.yaml"
      ;;
    *)
      e2e_die "operator profile unsupported secret-provider: ${E2E_SECRET_PROVIDER}"
      return 1
      ;;
  esac

  cat >"${manifest_dir}/sync-policy.yaml" <<EOF_SYNC_POLICY
apiVersion: declarest.io/v1alpha1
kind: SyncPolicy
metadata:
  name: ${sync_policy_name}
  namespace: ${namespace}
spec:
  resourceRepositoryRef:
    name: ${repository_name}
  managedServiceRef:
    name: ${managed_service_name}
  secretStoreRef:
    name: ${secret_store_name}
  source:
    path: /
    recursive: true
  sync:
    prune: false
  suspend: false
EOF_SYNC_POLICY

  E2E_OPERATOR_NAMESPACE="${namespace}"
  E2E_OPERATOR_RESOURCE_REPOSITORY_NAME="${repository_name}"
  E2E_OPERATOR_MANAGED_SERVICE_NAME="${managed_service_name}"
  E2E_OPERATOR_SECRET_STORE_NAME="${secret_store_name}"
  E2E_OPERATOR_SYNC_POLICY_NAME="${sync_policy_name}"
  export E2E_OPERATOR_NAMESPACE
  export E2E_OPERATOR_RESOURCE_REPOSITORY_NAME
  export E2E_OPERATOR_MANAGED_SERVICE_NAME
  export E2E_OPERATOR_SECRET_STORE_NAME
  export E2E_OPERATOR_SYNC_POLICY_NAME

  e2e_runtime_state_set 'OPERATOR_NAMESPACE' "${namespace}" || return 1
  e2e_runtime_state_set 'OPERATOR_RESOURCE_REPOSITORY_NAME' "${repository_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_MANAGED_SERVICE_NAME' "${managed_service_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_SECRET_STORE_NAME' "${secret_store_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_SYNC_POLICY_NAME' "${sync_policy_name}" || return 1
  return 0
}

e2e_operator_wait_resource_ready() {
  local resource_type=$1
  local resource_name=$2
  local timeout=${3:-}

  if [[ -z "${timeout}" ]]; then
    local ready_timeout_seconds
    ready_timeout_seconds=$(e2e_operator_ready_timeout_seconds) || return 1
    timeout="${ready_timeout_seconds}s"
  fi

  e2e_info "operator profile waiting ready resource=${resource_type}/${resource_name} timeout=${timeout}"
  if ! kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${E2E_OPERATOR_NAMESPACE}" \
    wait --for=condition=Ready --timeout="${timeout}" "${resource_type}/${resource_name}" >/dev/null 2>&1; then
    e2e_error "operator profile ready wait failed resource=${resource_type}/${resource_name}"
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${E2E_OPERATOR_NAMESPACE}" get "${resource_type}/${resource_name}" -o yaml || true
    return 1
  fi
  return 0
}

e2e_operator_wait_resources_ready_parallel() {
  local -a resource_specs=("$@")
  local -a pids=()
  local spec
  local resource_type
  local resource_name

  for spec in "${resource_specs[@]}"; do
    resource_type=${spec%%:*}
    resource_name=${spec#*:}
    (
      e2e_operator_wait_resource_ready "${resource_type}" "${resource_name}"
    ) &
    pids+=("$!")
  done

  local failed=0
  local pid
  local rc
  for pid in "${pids[@]}"; do
    set +e
    wait "${pid}"
    rc=$?
    set -e
    if ((rc != 0)); then
      failed=1
    fi
  done

  ((failed == 0))
}

e2e_operator_apply_manifests() {
  local manifest_dir
  manifest_dir=$(e2e_operator_manifest_dir)

  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-repository-auth.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-managed-service-auth.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-secret-store-auth.yaml" || return 1

  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/resource-repository.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/managed-service.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-store.yaml" || return 1

  e2e_operator_wait_resources_ready_parallel \
    "resourcerepository.declarest.io:${E2E_OPERATOR_RESOURCE_REPOSITORY_NAME}" \
    "managedservice.declarest.io:${E2E_OPERATOR_MANAGED_SERVICE_NAME}" \
    "secretstore.declarest.io:${E2E_OPERATOR_SECRET_STORE_NAME}" || return 1

  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/sync-policy.yaml" || return 1
  e2e_operator_wait_resource_ready 'syncpolicy.declarest.io' "${E2E_OPERATOR_SYNC_POLICY_NAME}" || return 1
  return 0
}

e2e_operator_install_stack() {
  e2e_operator_profile_enabled || return 0
  e2e_operator_install_via_olm || return 1
  e2e_operator_write_manifests || return 1
  e2e_operator_apply_manifests || return 1
  return 0
}

e2e_operator_example_resource_path() {
  local managed_service_key
  managed_service_key=$(e2e_component_key 'managed-service' "${E2E_MANAGED_SERVICE:-}")
  if [[ -n "${E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH[${managed_service_key}]:-}" ]]; then
    printf '%s\n' "${E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH[${managed_service_key}]}"
    return 0
  fi

  printf '/operator-demo\n'
}

e2e_operator_example_resource_payload() {
  local managed_service_key
  managed_service_key=$(e2e_component_key 'managed-service' "${E2E_MANAGED_SERVICE:-}")
  if [[ -n "${E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD[${managed_service_key}]:-}" ]]; then
    printf '%s\n' "${E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD[${managed_service_key}]}"
    return 0
  fi

  printf '{"name":"operator-demo"}\n'
}

e2e_profile_operator_handoff() {
  local context_name=$1
  local setup_script
  local reset_script
  local resource_path
  local resource_payload
  local manager_deployment
  local sync_policy_name
  local repository_name
  local commit_message='operator demo resource'

  resource_path=$(e2e_operator_example_resource_path)
  resource_payload=$(e2e_operator_example_resource_payload)
  manager_deployment=${E2E_OPERATOR_MANAGER_DEPLOYMENT:-$(e2e_operator_scoped_name 'declarest-operator')}
  sync_policy_name=${E2E_OPERATOR_SYNC_POLICY_NAME:-$(e2e_operator_scoped_name 'declarest-e2e-sync-policy')}
  repository_name=${E2E_OPERATOR_RESOURCE_REPOSITORY_NAME:-${E2E_OPERATOR_REPOSITORY_NAME:-$(e2e_operator_scoped_name 'declarest-e2e-repository')}}

  e2e_manual_write_env_scripts "${context_name}" || return 1
  setup_script=$(e2e_manual_env_setup_script_path)
  reset_script=$(e2e_manual_env_reset_script_path)

  cat <<EOF_HANDOFF
Operator profile is ready.

Run ID:
  ${E2E_RUN_ID:-n/a}

Context name:
  ${context_name}

Context file:
  ${E2E_CONTEXT_FILE}

Operator runtime:
  manager-deployment: ${E2E_OPERATOR_MANAGER_DEPLOYMENT:-${manager_deployment}}
  manager-pod: ${E2E_OPERATOR_MANAGER_POD:-n/a}
  manager-image: ${E2E_OPERATOR_IMAGE:-n/a}
  manager-logs: ${E2E_OPERATOR_MANAGER_LOG_FILE:-n/a}
  namespace: ${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-n/a}}
  sync-policy: ${E2E_OPERATOR_SYNC_POLICY_NAME:-${sync_policy_name}}
  repository-webhook-url: ${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL:-n/a}

Shell scripts:
  setup: ${setup_script}
  reset: ${reset_script}

To use it in your current shell:
  source ${setup_script@Q}
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" get deploy "${E2E_OPERATOR_MANAGER_DEPLOYMENT:-${manager_deployment}}"
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" logs deployment/"${E2E_OPERATOR_MANAGER_DEPLOYMENT:-${manager_deployment}}" --tail=80
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" get resourcerepository,managedservice,secretstore,syncpolicy
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" get syncpolicy "${E2E_OPERATOR_SYNC_POLICY_NAME:-${sync_policy_name}}" -o yaml
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repository status
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" resource save ${resource_path@Q} --payload ${resource_payload@Q}
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repository commit -m ${commit_message@Q}
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repository push
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" resource get ${resource_path@Q} --source managed-service
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" get resourcerepository "${repository_name}" -o jsonpath='{.metadata.annotations.declarest\\.io/webhook-last-received-at}'
EOF_HANDOFF

  if [[ "${E2E_PLATFORM}" == 'kubernetes' && -n "${E2E_KIND_CLUSTER_NAME:-}" ]]; then
    printf '\n'
    e2e_profile_print_kubernetes_connection_help
  fi

  if [[ -n "${E2E_MANUAL_COMPONENT_ACCESS_OUTPUT:-}" ]]; then
    printf '\n'
    e2e_profile_print_manual_component_access_help
  fi

  if [[ "${E2E_REPO_TYPE:-}" == 'git' && -n "${E2E_GIT_PROVIDER:-}" ]]; then
    printf '\n'
    e2e_profile_print_repo_provider_access_help
  fi

  cat <<EOF_HANDOFF

To reset environment variables and alias:
  source ${reset_script@Q}

Runtime resources are kept for manual verification.
To stop and remove this execution:
  ./run-e2e.sh --clean ${E2E_RUN_ID:-<run-id>}
To stop and remove all executions:
  ./run-e2e.sh --clean-all
EOF_HANDOFF
}
