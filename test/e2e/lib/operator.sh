#!/usr/bin/env bash

# Operator profile helpers for installing the manager as an in-cluster Deployment
# and applying generated CR instances based on selected component state.

e2e_operator_profile_enabled() {
  [[ "${E2E_PROFILE}" == 'operator' ]]
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
  local namespace=${E2E_K8S_NAMESPACE:-default}
  printf '%s.%s.svc.cluster.local\n' "${service_name}" "${namespace}"
}

e2e_operator_service_network_host() {
  local service_name=$1
  local namespace=${E2E_K8S_NAMESPACE:-default}

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

e2e_operator_rewrite_repo_url_for_cluster() {
  local repo_url=$1

  if [[ "${E2E_PLATFORM:-}" != 'kubernetes' || "${E2E_GIT_PROVIDER_CONNECTION:-}" != 'local' ]]; then
    printf '%s\n' "${repo_url}"
    return 0
  fi

  case "${E2E_GIT_PROVIDER:-}" in
    gitea)
      e2e_operator_rewrite_local_url_to_service "${repo_url}" 'git-provider-gitea' '3000'
      ;;
    gitlab)
      e2e_operator_rewrite_local_url_to_service "${repo_url}" 'git-provider-gitlab' '80'
      ;;
    *)
      printf '%s\n' "${repo_url}"
      ;;
  esac
}

e2e_operator_manager_manifest_path() {
  printf '%s/operator-manager.yaml\n' "$(e2e_operator_manifest_dir)"
}

e2e_operator_wait_pid_exit() {
  local pid=$1
  local loops=${2:-60}
  local idx

  for ((idx = 0; idx < loops; idx++)); do
    if ! kill -0 "${pid}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done

  return 1
}

e2e_operator_stop_manager() {
  local manager_manifest
  manager_manifest=$(e2e_operator_manager_manifest_path)
  if [[ -n "${E2E_KUBECONFIG:-}" && -f "${manager_manifest}" ]]; then
    e2e_info "stopping operator manager deployment manifest=${manager_manifest}"
    kubectl --kubeconfig "${E2E_KUBECONFIG}" delete -f "${manager_manifest}" --ignore-not-found >/dev/null 2>&1 || true
  fi

  local pid=${E2E_OPERATOR_MANAGER_PID:-}

  if [[ -z "${pid}" ]]; then
    local runtime_state
    runtime_state=$(e2e_runtime_state_file)
    pid=$(e2e_state_get "${runtime_state}" 'OPERATOR_MANAGER_PID' || true)
  fi

  if [[ -z "${pid}" || ! "${pid}" =~ ^[0-9]+$ ]]; then
    return 0
  fi
  if ! kill -0 "${pid}" >/dev/null 2>&1; then
    return 0
  fi

  e2e_info "stopping operator manager pid=${pid}"
  kill -TERM "${pid}" >/dev/null 2>&1 || true
  if e2e_operator_wait_pid_exit "${pid}" 80; then
    return 0
  fi

  kill -KILL "${pid}" >/dev/null 2>&1 || true
  if ! e2e_operator_wait_pid_exit "${pid}" 20; then
    e2e_warn "failed to stop operator manager pid=${pid}"
    return 1
  fi
  return 0
}

e2e_operator_install_crds() {
  local crd_dir="${E2E_ROOT_DIR}/config/crd/bases"
  if [[ ! -d "${crd_dir}" ]]; then
    e2e_die "operator profile missing CRD manifests: ${crd_dir}"
    return 1
  fi

  e2e_info "operator profile installing CRDs from ${crd_dir}"
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${crd_dir}" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" wait --for=condition=Established --timeout=120s \
    crd/resourcerepositories.declarest.io \
    crd/managedservers.declarest.io \
    crd/secretstores.declarest.io \
    crd/syncpolicies.declarest.io || return 1
  return 0
}

e2e_operator_start_manager() {
  if [[ -z "${E2E_OPERATOR_IMAGE:-}" ]]; then
    e2e_die "operator manager image is unavailable: ${E2E_OPERATOR_IMAGE:-<unset>}"
    return 1
  fi

  if [[ -z "${E2E_KIND_CLUSTER_NAME:-}" ]]; then
    e2e_die 'operator profile kind cluster metadata is missing'
    return 1
  fi

  local namespace="${E2E_K8S_NAMESPACE:-default}"
  local deployment_name='declarest-operator'
  local role_name='declarest-operator'
  local role_binding_name='declarest-operator'
  local service_account_name='declarest-operator'
  local runtime_root='/var/lib/declarest'
  local repo_root="${runtime_root}/repos"
  local cache_root="${runtime_root}/cache"
  local manifest_dir
  manifest_dir=$(e2e_operator_manifest_dir)
  mkdir -p "${manifest_dir}" || return 1

  local manager_manifest
  manager_manifest=$(e2e_operator_manager_manifest_path)
  cat >"${manager_manifest}" <<EOF_MANAGER
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${service_account_name}
  namespace: ${namespace}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ${role_name}
  namespace: ${namespace}
rules:
  - apiGroups: ["declarest.io"]
    resources: ["resourcerepositories", "managedservers", "secretstores", "syncpolicies"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["declarest.io"]
    resources: ["resourcerepositories/status", "managedservers/status", "secretstores/status", "syncpolicies/status"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["declarest.io"]
    resources: ["resourcerepositories/finalizers", "managedservers/finalizers", "secretstores/finalizers", "syncpolicies/finalizers"]
    verbs: ["update"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${role_binding_name}
  namespace: ${namespace}
subjects:
  - kind: ServiceAccount
    name: ${service_account_name}
    namespace: ${namespace}
roleRef:
  kind: Role
  name: ${role_name}
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${deployment_name}
  namespace: ${namespace}
  labels:
    app.kubernetes.io/name: declarest-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: declarest-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: declarest-operator
    spec:
      serviceAccountName: ${service_account_name}
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: manager
          image: ${E2E_OPERATOR_IMAGE}
          imagePullPolicy: IfNotPresent
          args:
            - --leader-elect=false
            - --enable-webhooks=false
            - --watch-namespace=${namespace}
            - --health-probe-bind-address=:18081
            - --metrics-bind-address=:18080
          env:
            - name: DECLAREST_OPERATOR_REPO_BASE_DIR
              value: ${repo_root}
            - name: DECLAREST_OPERATOR_CACHE_BASE_DIR
              value: ${cache_root}
            - name: KUBERNETES_SERVICE_HOST
              value: "127.0.0.1"
            - name: KUBERNETES_SERVICE_PORT
              value: "6443"
          ports:
            - containerPort: 18080
              name: metrics
            - containerPort: 18081
              name: probes
          readinessProbe:
            httpGet:
              path: /readyz
              port: probes
          livenessProbe:
            httpGet:
              path: /healthz
              port: probes
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
          volumeMounts:
            - name: state
              mountPath: ${runtime_root}
      volumes:
        - name: state
          emptyDir: {}
EOF_MANAGER

  local image_archive="${E2E_RUN_DIR}/operator/manager-image.tar"
  mkdir -p -- "$(dirname -- "${image_archive}")" || return 1
  e2e_info "operator profile exporting manager image archive image=${E2E_OPERATOR_IMAGE} archive=${image_archive}"
  e2e_run_cmd "${E2E_CONTAINER_ENGINE}" save -o "${image_archive}" "${E2E_OPERATOR_IMAGE}" || return 1

  e2e_info "operator profile loading manager image archive into kind cluster name=${E2E_KIND_CLUSTER_NAME} archive=${image_archive}"
  e2e_kind_cmd load image-archive "${image_archive}" --name "${E2E_KIND_CLUSTER_NAME}" || return 1

  e2e_info "operator profile installing manager deployment namespace=${namespace}"
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manager_manifest}" || return 1
  if ! e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" rollout status "deployment/${deployment_name}" --timeout=180s; then
    e2e_error "operator manager deployment failed rollout deployment=${deployment_name} namespace=${namespace}"
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" describe "deployment/${deployment_name}" || true
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${namespace}" logs "deployment/${deployment_name}" --tail=80 || true
    return 1
  fi

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
  e2e_runtime_state_set 'OPERATOR_IMAGE_ARCHIVE' "${image_archive}" || return 1
  e2e_runtime_state_set 'OPERATOR_NAMESPACE' "${namespace}" || return 1
  e2e_runtime_state_set 'OPERATOR_MANAGER_DEPLOYMENT' "${deployment_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_MANAGER_LOG_FILE' "${E2E_OPERATOR_MANAGER_LOG_FILE}" || return 1
  if [[ -n "${E2E_OPERATOR_MANAGER_POD}" ]]; then
    e2e_runtime_state_set 'OPERATOR_MANAGER_POD' "${E2E_OPERATOR_MANAGER_POD}" || return 1
  fi
  e2e_runtime_state_set 'OPERATOR_REPO_BASE_DIR' "${repo_root}" || return 1
  e2e_runtime_state_set 'OPERATOR_CACHE_BASE_DIR' "${cache_root}" || return 1
  return 0
}

e2e_operator_collect_managed_server_config() {
  local state_file=$1

  # shellcheck disable=SC1090
  source "${state_file}"

  E2E_OPERATOR_MANAGED_SERVER_BASE_URL=''
  E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND=''
  E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL=''
  E2E_OPERATOR_MANAGED_SERVER_OAUTH_SCOPE=''
  E2E_OPERATOR_MANAGED_SERVER_OAUTH_AUDIENCE=''
  E2E_OPERATOR_MANAGED_SERVER_BASIC_USERNAME=''
  E2E_OPERATOR_MANAGED_SERVER_BASIC_PASSWORD=''
  E2E_OPERATOR_MANAGED_SERVER_HEADER_NAME=''
  E2E_OPERATOR_MANAGED_SERVER_HEADER_PREFIX=''
  E2E_OPERATOR_MANAGED_SERVER_HEADER_VALUE=''
  E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_ID=''
  E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_SECRET=''

  case "${E2E_MANAGED_SERVER}" in
    simple-api-server)
      E2E_OPERATOR_MANAGED_SERVER_BASE_URL=${SIMPLE_API_SERVER_BASE_URL:-${MANAGED_SERVER_BASE_URL:-}}
      case "${E2E_MANAGED_SERVER_AUTH_TYPE}" in
        oauth2)
          E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND='oauth2'
          E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL=${SIMPLE_API_SERVER_TOKEN_URL:-}
          E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_ID=${SIMPLE_API_SERVER_CLIENT_ID:-}
          E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_SECRET=${SIMPLE_API_SERVER_CLIENT_SECRET:-}
          E2E_OPERATOR_MANAGED_SERVER_OAUTH_SCOPE=${SIMPLE_API_SERVER_SCOPE:-}
          E2E_OPERATOR_MANAGED_SERVER_OAUTH_AUDIENCE=${SIMPLE_API_SERVER_AUDIENCE:-}
          ;;
        basic)
          E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND='basic'
          E2E_OPERATOR_MANAGED_SERVER_BASIC_USERNAME=${SIMPLE_API_SERVER_BASIC_AUTH_USERNAME:-}
          E2E_OPERATOR_MANAGED_SERVER_BASIC_PASSWORD=${SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD:-}
          ;;
        none)
          E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND='custom-header'
          E2E_OPERATOR_MANAGED_SERVER_HEADER_NAME='Authorization'
          E2E_OPERATOR_MANAGED_SERVER_HEADER_PREFIX='Bearer'
          E2E_OPERATOR_MANAGED_SERVER_HEADER_VALUE='simple-api-oauth2-disabled'
          ;;
        *)
          e2e_die "operator profile does not support managed-server auth-type ${E2E_MANAGED_SERVER_AUTH_TYPE} for simple-api-server"
          return 1
          ;;
      esac
      ;;
    keycloak)
      [[ "${E2E_MANAGED_SERVER_AUTH_TYPE}" == 'oauth2' ]] || {
        e2e_die "operator profile keycloak requires --managed-server-auth-type oauth2"
        return 1
      }
      E2E_OPERATOR_MANAGED_SERVER_BASE_URL=${KEYCLOAK_BASE_URL:-${MANAGED_SERVER_BASE_URL:-}}
      E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND='oauth2'
      E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL=${KEYCLOAK_TOKEN_URL:-}
      E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_ID=${KEYCLOAK_CLIENT_ID:-}
      E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_SECRET=${KEYCLOAK_CLIENT_SECRET:-}
      E2E_OPERATOR_MANAGED_SERVER_OAUTH_SCOPE=${KEYCLOAK_SCOPE:-}
      E2E_OPERATOR_MANAGED_SERVER_OAUTH_AUDIENCE=${KEYCLOAK_AUDIENCE:-}
      ;;
    rundeck)
      E2E_OPERATOR_MANAGED_SERVER_BASE_URL="${RUNDECK_BASE_URL%/}/api/${RUNDECK_API_VERSION:-45}"
      case "${E2E_MANAGED_SERVER_AUTH_TYPE}" in
        custom-header)
          E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND='custom-header'
          E2E_OPERATOR_MANAGED_SERVER_HEADER_NAME=${RUNDECK_AUTH_HEADER:-X-Rundeck-Auth-Token}
          E2E_OPERATOR_MANAGED_SERVER_HEADER_VALUE=${RUNDECK_API_TOKEN:-}
          ;;
        basic)
          E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND='basic'
          E2E_OPERATOR_MANAGED_SERVER_BASIC_USERNAME=${RUNDECK_ADMIN_USER:-}
          E2E_OPERATOR_MANAGED_SERVER_BASIC_PASSWORD=${RUNDECK_ADMIN_PASSWORD:-}
          ;;
        *)
          e2e_die "operator profile rundeck supports auth-type basic or custom-header"
          return 1
          ;;
      esac
      ;;
    vault)
      [[ "${E2E_MANAGED_SERVER_AUTH_TYPE}" == 'custom-header' ]] || {
        e2e_die "operator profile vault requires --managed-server-auth-type custom-header"
        return 1
      }
      E2E_OPERATOR_MANAGED_SERVER_BASE_URL=${VAULT_ADDRESS:-}
      E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND='custom-header'
      E2E_OPERATOR_MANAGED_SERVER_HEADER_NAME='X-Vault-Token'
      E2E_OPERATOR_MANAGED_SERVER_HEADER_VALUE=${VAULT_TOKEN:-}
      ;;
    *)
      e2e_die "operator profile unsupported managed-server: ${E2E_MANAGED_SERVER}"
      return 1
      ;;
  esac

  if [[ "${E2E_PLATFORM:-}" == 'kubernetes' && "${E2E_MANAGED_SERVER_CONNECTION:-}" == 'local' ]]; then
    local service_name=''
    local service_port=''

    case "${E2E_MANAGED_SERVER}" in
      simple-api-server)
        service_name='managed-server-simple-api-server'
        service_port='8080'
        ;;
      keycloak)
        service_name='managed-server-keycloak'
        service_port='8080'
        ;;
      rundeck)
        service_name='managed-server-rundeck'
        service_port='4440'
        ;;
      vault)
        service_name='managed-server-vault'
        service_port='8200'
        ;;
    esac

    if [[ -n "${service_name}" && -n "${service_port}" ]]; then
      E2E_OPERATOR_MANAGED_SERVER_BASE_URL=$(
        e2e_operator_rewrite_local_url_to_service \
          "${E2E_OPERATOR_MANAGED_SERVER_BASE_URL}" \
          "${service_name}" \
          "${service_port}"
      )
      if [[ -n "${E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL}" ]]; then
        E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL=$(
          e2e_operator_rewrite_local_url_to_service \
            "${E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL}" \
            "${service_name}" \
            "${service_port}"
        )
      fi
    fi
  fi

  if [[ -z "${E2E_OPERATOR_MANAGED_SERVER_BASE_URL}" ]]; then
    e2e_die 'operator profile managed-server base URL is empty after component setup'
    return 1
  fi

  case "${E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND}" in
    oauth2)
      [[ -n "${E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL}" ]] || {
        e2e_die 'operator profile managed-server oauth2 token URL is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_ID}" ]] || {
        e2e_die 'operator profile managed-server oauth2 client id is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_SECRET}" ]] || {
        e2e_die 'operator profile managed-server oauth2 client secret is empty'
        return 1
      }
      ;;
    basic)
      [[ -n "${E2E_OPERATOR_MANAGED_SERVER_BASIC_USERNAME}" ]] || {
        e2e_die 'operator profile managed-server basic username is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVER_BASIC_PASSWORD}" ]] || {
        e2e_die 'operator profile managed-server basic password is empty'
        return 1
      }
      ;;
    custom-header)
      [[ -n "${E2E_OPERATOR_MANAGED_SERVER_HEADER_NAME}" ]] || {
        e2e_die 'operator profile managed-server custom header name is empty'
        return 1
      }
      [[ -n "${E2E_OPERATOR_MANAGED_SERVER_HEADER_VALUE}" ]] || {
        e2e_die 'operator profile managed-server custom header value is empty'
        return 1
      }
      ;;
    *)
      e2e_die 'operator profile managed-server auth mode is unresolved'
      return 1
      ;;
  esac

  return 0
}

e2e_operator_write_manifests() {
  local manifest_dir
  manifest_dir=$(e2e_operator_manifest_dir)
  mkdir -p "${manifest_dir}" || return 1

  local namespace="${E2E_K8S_NAMESPACE:-default}"

  local repo_key
  repo_key=$(e2e_component_key 'repo-type' "${E2E_REPO_TYPE}")
  local repo_state_file
  repo_state_file=$(e2e_component_state_file "${repo_key}")

  local managed_server_key
  managed_server_key=$(e2e_component_key 'managed-server' "${E2E_MANAGED_SERVER}")
  local managed_server_state_file
  managed_server_state_file=$(e2e_component_state_file "${managed_server_key}")

  local secret_store_key
  secret_store_key=$(e2e_component_key 'secret-provider' "${E2E_SECRET_PROVIDER}")
  local secret_store_state_file
  secret_store_state_file=$(e2e_component_state_file "${secret_store_key}")

  [[ -f "${repo_state_file}" ]] || {
    e2e_die "operator profile missing repository state: ${repo_state_file}"
    return 1
  }
  [[ -f "${managed_server_state_file}" ]] || {
    e2e_die "operator profile missing managed-server state: ${managed_server_state_file}"
    return 1
  }
  [[ -f "${secret_store_state_file}" ]] || {
    e2e_die "operator profile missing secret-store state: ${secret_store_state_file}"
    return 1
  }

  # shellcheck disable=SC1090
  source "${repo_state_file}"

  local repository_name='declarest-e2e-repository'
  local managed_server_name='declarest-e2e-managed-server'
  local secret_store_name='declarest-e2e-secret-store'
  local sync_policy_name='declarest-e2e-sync-policy'

  local repo_secret_name='declarest-e2e-repo-auth'
  local managed_server_secret_name='declarest-e2e-managed-server-auth'
  local secret_store_secret_name='declarest-e2e-secret-store-auth'

  local repo_url=${GIT_REMOTE_URL:-}
  local repo_branch=${GIT_REMOTE_BRANCH:-main}
  local repo_format=${REPO_RESOURCE_FORMAT:-json}
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

  cat >"${manifest_dir}/resource-repository.yaml" <<EOF_REPO_CR
apiVersion: declarest.io/v1alpha1
kind: ResourceRepository
metadata:
  name: ${repository_name}
  namespace: ${namespace}
spec:
  type: git
  pollInterval: 30s
  resourceFormat: ${repo_format}
  git:
    url: $(e2e_operator_yaml_quote "${repo_url}")
    branch: $(e2e_operator_yaml_quote "${repo_branch}")
    auth:
      tokenSecretRef:
        name: ${repo_secret_name}
        key: token
  storage:
    pvc:
      accessModes:
        - ReadWriteOnce
      requests:
        storage: 1Gi
EOF_REPO_CR

  e2e_operator_collect_managed_server_config "${managed_server_state_file}" || return 1

  local managed_server_tls_enabled='false'
  local managed_server_tls_insecure_skip_verify='false'
  local tls_ca_file=''
  local tls_client_cert_file=''
  local tls_client_key_file=''
  if [[ "${E2E_MANAGED_SERVER}" == 'simple-api-server' && "${E2E_MANAGED_SERVER_MTLS}" == 'true' ]]; then
    managed_server_tls_enabled='true'
    if [[ "${E2E_PLATFORM:-}" == 'kubernetes' ]]; then
      # simple-api-server test certs are issued for localhost; operator traffic uses cluster service DNS.
      managed_server_tls_insecure_skip_verify='true'
    fi
    tls_ca_file=${SIMPLE_API_SERVER_TLS_CA_CERT_FILE_HOST:-}
    tls_client_cert_file=${SIMPLE_API_SERVER_TLS_CLIENT_CERT_FILE_HOST:-}
    tls_client_key_file=${SIMPLE_API_SERVER_TLS_CLIENT_KEY_FILE_HOST:-}
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
    printf '  name: %s\n' "${managed_server_secret_name}"
    printf '  namespace: %s\n' "${namespace}"
    printf 'type: Opaque\n'
    printf 'data:\n'
    case "${E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND}" in
      oauth2)
        printf '  client-id: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_ID}")"
        printf '  client-secret: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_CLIENT_SECRET}")"
        ;;
      basic)
        printf '  username: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVER_BASIC_USERNAME}")"
        printf '  password: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVER_BASIC_PASSWORD}")"
        ;;
      custom-header)
        printf '  header-value: %s\n' "$(e2e_operator_b64 "${E2E_OPERATOR_MANAGED_SERVER_HEADER_VALUE}")"
        ;;
    esac
    if [[ "${managed_server_tls_enabled}" == 'true' ]]; then
      printf '  ca-cert: %s\n' "$(e2e_operator_b64 "$(cat "${tls_ca_file}")")"
      printf '  client-cert: %s\n' "$(e2e_operator_b64 "$(cat "${tls_client_cert_file}")")"
      printf '  client-key: %s\n' "$(e2e_operator_b64 "$(cat "${tls_client_key_file}")")"
    fi
  } >"${manifest_dir}/secret-managed-server-auth.yaml"

  {
    printf 'apiVersion: declarest.io/v1alpha1\n'
    printf 'kind: ManagedServer\n'
    printf 'metadata:\n'
    printf '  name: %s\n' "${managed_server_name}"
    printf '  namespace: %s\n' "${namespace}"
    printf 'spec:\n'
    printf '  http:\n'
    printf '    baseURL: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVER_BASE_URL}")"
    printf '    auth:\n'
    case "${E2E_OPERATOR_MANAGED_SERVER_AUTH_KIND}" in
      oauth2)
        printf '      oauth2:\n'
        printf '        tokenURL: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVER_TOKEN_URL}")"
        printf '        grantType: client_credentials\n'
        printf '        clientIDRef:\n'
        printf '          name: %s\n' "${managed_server_secret_name}"
        printf '          key: client-id\n'
        printf '        clientSecretRef:\n'
        printf '          name: %s\n' "${managed_server_secret_name}"
        printf '          key: client-secret\n'
        if [[ -n "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_SCOPE}" ]]; then
          printf '        scope: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_SCOPE}")"
        fi
        if [[ -n "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_AUDIENCE}" ]]; then
          printf '        audience: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVER_OAUTH_AUDIENCE}")"
        fi
        ;;
      basic)
        printf '      basicAuth:\n'
        printf '        usernameRef:\n'
        printf '          name: %s\n' "${managed_server_secret_name}"
        printf '          key: username\n'
        printf '        passwordRef:\n'
        printf '          name: %s\n' "${managed_server_secret_name}"
        printf '          key: password\n'
        ;;
      custom-header)
        printf '      customHeaders:\n'
        printf '        - header: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVER_HEADER_NAME}")"
        if [[ -n "${E2E_OPERATOR_MANAGED_SERVER_HEADER_PREFIX}" ]]; then
          printf '          prefix: %s\n' "$(e2e_operator_yaml_quote "${E2E_OPERATOR_MANAGED_SERVER_HEADER_PREFIX}")"
        fi
        printf '          valueRef:\n'
        printf '            name: %s\n' "${managed_server_secret_name}"
        printf '            key: header-value\n'
        ;;
    esac
    if [[ "${managed_server_tls_enabled}" == 'true' ]]; then
      printf '    tls:\n'
      printf '      caCertRef:\n'
      printf '        name: %s\n' "${managed_server_secret_name}"
      printf '        key: ca-cert\n'
      printf '      clientCertRef:\n'
      printf '        name: %s\n' "${managed_server_secret_name}"
      printf '        key: client-cert\n'
      printf '      clientKeyRef:\n'
      printf '        name: %s\n' "${managed_server_secret_name}"
      printf '        key: client-key\n'
      if [[ "${managed_server_tls_insecure_skip_verify}" == 'true' ]]; then
        printf '      insecureSkipVerify: true\n'
      fi
    fi
  } >"${manifest_dir}/managed-server.yaml"

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
  provider: file
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
        printf '  provider: vault\n'
        printf '  vault:\n'
        printf '    address: %s\n' "$(e2e_operator_yaml_quote "${vault_address}")"
        printf '    mount: %s\n' "$(e2e_operator_yaml_quote "${VAULT_MOUNT:-secret}")"
        printf '    pathPrefix: %s\n' "$(e2e_operator_yaml_quote "${VAULT_PATH_PREFIX:-declarest-e2e}")"
        printf '    kvVersion: %s\n' "${VAULT_KV_VERSION:-2}"
        printf '    auth:\n'
        case "${VAULT_AUTH_MODE:-token}" in
          token)
            printf '      tokenRef:\n'
            printf '        name: %s\n' "${secret_store_secret_name}"
            printf '        key: token\n'
            ;;
          password)
            printf '      usernameRef:\n'
            printf '        name: %s\n' "${secret_store_secret_name}"
            printf '        key: username\n'
            printf '      passwordRef:\n'
            printf '        name: %s\n' "${secret_store_secret_name}"
            printf '        key: password\n'
            printf '      userpassMount: %s\n' "$(e2e_operator_yaml_quote "${VAULT_AUTH_MOUNT:-userpass}")"
            ;;
          approle)
            printf '      appRoleRoleIDRef:\n'
            printf '        name: %s\n' "${secret_store_secret_name}"
            printf '        key: role-id\n'
            printf '      appRoleSecretIDRef:\n'
            printf '        name: %s\n' "${secret_store_secret_name}"
            printf '        key: secret-id\n'
            printf '      appRoleMount: %s\n' "$(e2e_operator_yaml_quote "${VAULT_AUTH_MOUNT:-approle}")"
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
  managedServerRef:
    name: ${managed_server_name}
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
  E2E_OPERATOR_MANAGED_SERVER_NAME="${managed_server_name}"
  E2E_OPERATOR_SECRET_STORE_NAME="${secret_store_name}"
  E2E_OPERATOR_SYNC_POLICY_NAME="${sync_policy_name}"
  export E2E_OPERATOR_NAMESPACE
  export E2E_OPERATOR_RESOURCE_REPOSITORY_NAME
  export E2E_OPERATOR_MANAGED_SERVER_NAME
  export E2E_OPERATOR_SECRET_STORE_NAME
  export E2E_OPERATOR_SYNC_POLICY_NAME

  e2e_runtime_state_set 'OPERATOR_NAMESPACE' "${namespace}" || return 1
  e2e_runtime_state_set 'OPERATOR_RESOURCE_REPOSITORY_NAME' "${repository_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_MANAGED_SERVER_NAME' "${managed_server_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_SECRET_STORE_NAME' "${secret_store_name}" || return 1
  e2e_runtime_state_set 'OPERATOR_SYNC_POLICY_NAME' "${sync_policy_name}" || return 1
  return 0
}

e2e_operator_wait_resource_ready() {
  local resource_type=$1
  local resource_name=$2
  local timeout=${3:-300s}

  e2e_info "operator profile waiting ready resource=${resource_type}/${resource_name} timeout=${timeout}"
  if ! kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${E2E_OPERATOR_NAMESPACE}" \
    wait --for=condition=Ready --timeout="${timeout}" "${resource_type}/${resource_name}" >/dev/null 2>&1; then
    e2e_error "operator profile ready wait failed resource=${resource_type}/${resource_name}"
    kubectl --kubeconfig "${E2E_KUBECONFIG}" -n "${E2E_OPERATOR_NAMESPACE}" get "${resource_type}/${resource_name}" -o yaml || true
    return 1
  fi
  return 0
}

e2e_operator_apply_manifests() {
  local manifest_dir
  manifest_dir=$(e2e_operator_manifest_dir)

  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-repository-auth.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-managed-server-auth.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-secret-store-auth.yaml" || return 1

  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/resource-repository.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/managed-server.yaml" || return 1
  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/secret-store.yaml" || return 1

  e2e_operator_wait_resource_ready 'resourcerepository.declarest.io' "${E2E_OPERATOR_RESOURCE_REPOSITORY_NAME}" || return 1
  e2e_operator_wait_resource_ready 'managedserver.declarest.io' "${E2E_OPERATOR_MANAGED_SERVER_NAME}" || return 1
  e2e_operator_wait_resource_ready 'secretstore.declarest.io' "${E2E_OPERATOR_SECRET_STORE_NAME}" || return 1

  e2e_kubectl_cmd --kubeconfig "${E2E_KUBECONFIG}" apply -f "${manifest_dir}/sync-policy.yaml" || return 1
  e2e_operator_wait_resource_ready 'syncpolicy.declarest.io' "${E2E_OPERATOR_SYNC_POLICY_NAME}" '360s' || return 1
  return 0
}

e2e_operator_install_stack() {
  e2e_operator_profile_enabled || return 0
  e2e_operator_install_crds || return 1
  e2e_operator_start_manager || return 1
  e2e_operator_write_manifests || return 1
  e2e_operator_apply_manifests || return 1
  return 0
}

e2e_operator_example_resource_path() {
  case "${E2E_MANAGED_SERVER:-}" in
    simple-api-server)
      printf '/api/projects/operator-demo\n'
      ;;
    keycloak)
      printf '/admin/realms/operator-demo\n'
      ;;
    rundeck)
      printf '/project/operator-demo\n'
      ;;
    vault)
      printf '/v1/secret/data/declarest/operator-demo\n'
      ;;
    *)
      printf '/operator-demo\n'
      ;;
  esac
}

e2e_operator_example_resource_payload() {
  case "${E2E_MANAGED_SERVER:-}" in
    simple-api-server)
      printf '{"id":"operator-demo","name":"operator-demo","displayName":"Operator Demo","owner":"operator-e2e"}\n'
      ;;
    keycloak)
      printf '{"realm":"operator-demo","enabled":true,"displayName":"Operator Demo Realm"}\n'
      ;;
    rundeck)
      printf '{"name":"operator-demo","description":"Operator Demo Project"}\n'
      ;;
    vault)
      printf '{"path":"declarest/operator-demo","data":{"token":"operator-demo-token","owner":"operator-e2e"}}\n'
      ;;
    *)
      printf '{"name":"operator-demo"}\n'
      ;;
  esac
}

e2e_profile_operator_handoff() {
  local context_name=$1
  local setup_script
  local reset_script
  local resource_path
  local resource_payload
  local commit_message='operator demo resource'

  resource_path=$(e2e_operator_example_resource_path)
  resource_payload=$(e2e_operator_example_resource_payload)

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
  manager-deployment: ${E2E_OPERATOR_MANAGER_DEPLOYMENT:-n/a}
  manager-pod: ${E2E_OPERATOR_MANAGER_POD:-n/a}
  manager-image: ${E2E_OPERATOR_IMAGE:-n/a}
  manager-logs: ${E2E_OPERATOR_MANAGER_LOG_FILE:-n/a}
  namespace: ${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-n/a}}
  sync-policy: ${E2E_OPERATOR_SYNC_POLICY_NAME:-n/a}

Shell scripts:
  setup: ${setup_script}
  reset: ${reset_script}

To use it in your current shell:
  source ${setup_script@Q}
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" get deploy "${E2E_OPERATOR_MANAGER_DEPLOYMENT:-declarest-operator}"
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" logs deployment/"${E2E_OPERATOR_MANAGER_DEPLOYMENT:-declarest-operator}" --tail=80
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" get resourcerepository,managedserver,secretstore,syncpolicy
  kubectl --kubeconfig "${E2E_KUBECONFIG:-<kubeconfig>}" -n "${E2E_OPERATOR_NAMESPACE:-${E2E_K8S_NAMESPACE:-default}}" get syncpolicy "${E2E_OPERATOR_SYNC_POLICY_NAME:-declarest-e2e-sync-policy}" -o yaml
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repository status
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" resource save ${resource_path@Q} --payload ${resource_payload@Q}
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repository commit -m ${commit_message@Q}
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repository push
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" resource get ${resource_path@Q} --source remote-server
EOF_HANDOFF

  if [[ "${E2E_PLATFORM}" == 'kubernetes' && -n "${E2E_KIND_CLUSTER_NAME:-}" ]]; then
    printf '\n'
    e2e_profile_print_kubernetes_connection_help
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
