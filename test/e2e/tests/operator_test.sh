#!/usr/bin/env bash
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/testkit.sh"

load_operator_libs() {
  source_e2e_libs common components profile operator
}

prepare_operator_handoff_env() {
  local tmp=$1

  export E2E_PROFILE='operator-manual'
  export E2E_RUN_ID='operator-handoff-test'
  export E2E_RUNS_DIR="${tmp}/runs"
  export E2E_RUN_DIR="${E2E_RUNS_DIR}/${E2E_RUN_ID}"
  export E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  export E2E_CONTEXT_FILE="${E2E_RUN_DIR}/contexts.yaml"
  export E2E_BIN="${E2E_RUN_DIR}/bin/declarest"
  export E2E_PLATFORM='kubernetes'
  export E2E_KUBECONFIG="${tmp}/kubeconfig"
  export E2E_KIND_CLUSTER_NAME='declarest-e2e-operator'
  export E2E_K8S_NAMESPACE='declarest-operator'
  export E2E_OPERATOR_NAMESPACE='declarest-operator'
  export E2E_OPERATOR_MANAGER_DEPLOYMENT='declarest-operator'
  export E2E_OPERATOR_MANAGER_POD='declarest-operator-77b8f6fcb9-l9j6k'
  export E2E_OPERATOR_IMAGE='localhost/declarest/e2e-operator-manager:operator-handoff-test'
  export E2E_OPERATOR_SYNC_POLICY_NAME='declarest-e2e-sync-policy'
  export E2E_OPERATOR_RESOURCE_REPOSITORY_NAME='declarest-e2e-repository'
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_URL='http://declarest-operator-repo-webhook.declarest-operator.svc.cluster.local:18082/webhooks/repository/declarest-operator/declarest-e2e-repository'
  export E2E_REPO_TYPE='git'
  export E2E_GIT_PROVIDER='gitea'
  export E2E_GIT_PROVIDER_CONNECTION='local'

  mkdir -p "${E2E_STATE_DIR}" "$(dirname -- "${E2E_BIN}")"
  : >"${E2E_CONTEXT_FILE}"
  : >"${E2E_KUBECONFIG}"
  cat >"${E2E_BIN}" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
  chmod +x "${E2E_BIN}"

  local provider_state="${E2E_STATE_DIR}/git-provider-gitea.env"
  : >"${provider_state}"
  e2e_write_state_value "${provider_state}" 'GIT_REMOTE_URL' 'http://127.0.0.1:3000/declarest-e2e/declarest-e2e.git'
  e2e_write_state_value "${provider_state}" 'REPO_PROVIDER_BASE_URL' 'http://127.0.0.1:3000'
  e2e_write_state_value "${provider_state}" 'GIT_AUTH_USERNAME' 'gitea-admin'
  e2e_write_state_value "${provider_state}" 'GIT_AUTH_PASSWORD' 'gitea-pass'
  E2E_COMPONENT_REPO_PROVIDER_LOGIN_PATH=()
  E2E_COMPONENT_REPO_PROVIDER_LOGIN_PATH['git-provider:gitea']='/user/login'
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH=()
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD=()
  E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER=()
  E2E_COMPONENT_SERVICE_PORT=()
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH['managed-service:simple-api-server']='/api/projects/operator-demo'
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD['managed-service:simple-api-server']='{"id":"operator-demo","name":"operator-demo","displayName":"Operator Demo","owner":"operator-e2e"}'
  E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER['git-provider:gitea']='gitea'
  E2E_COMPONENT_SERVICE_PORT['git-provider:gitea']='3000'
}

test_operator_example_resource_mapping() {
  load_operator_libs

  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH=()
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD=()
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH['managed-service:simple-api-server']='/api/projects/operator-demo'
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD['managed-service:simple-api-server']='{"id":"operator-demo","name":"operator-demo","displayName":"Operator Demo","owner":"operator-e2e"}'
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PATH['managed-service:keycloak']='/admin/realms/operator-demo'
  E2E_COMPONENT_OPERATOR_EXAMPLE_RESOURCE_PAYLOAD['managed-service:keycloak']='{"realm":"operator-demo","enabled":true,"displayName":"Operator Demo Realm"}'
  E2E_MANAGED_SERVICE='simple-api-server'
  assert_eq "$(e2e_operator_example_resource_path)" "/api/projects/operator-demo"
  assert_contains "$(e2e_operator_example_resource_payload)" "\"id\":\"operator-demo\""

  E2E_MANAGED_SERVICE='keycloak'
  assert_eq "$(e2e_operator_example_resource_path)" "/admin/realms/operator-demo"
  assert_contains "$(e2e_operator_example_resource_payload)" "\"realm\":\"operator-demo\""
}

test_operator_scoped_names_are_run_specific() {
  load_operator_libs

  E2E_RUN_ID='operator-run-alpha'
  local alpha_name
  alpha_name=$(e2e_operator_scoped_name 'declarest-e2e-sync-policy')
  assert_contains "${alpha_name}" 'operator-run-alpha'

  E2E_RUN_ID='operator-run-beta'
  local beta_name
  beta_name=$(e2e_operator_scoped_name 'declarest-e2e-sync-policy')
  assert_contains "${beta_name}" 'operator-run-beta'

  if [[ "${alpha_name}" == "${beta_name}" ]]; then
    fail "expected run-scoped names to differ, got ${alpha_name}"
  fi
  if ((${#alpha_name} > 63 || ${#beta_name} > 63)); then
    fail "expected run-scoped names to stay within DNS-1123 limits: ${alpha_name}, ${beta_name}"
  fi

  E2E_RUN_ID='operator-run-with-very-very-very-very-very-very-long-identifier'
  local long_name
  long_name=$(e2e_operator_scoped_name 'declarest-e2e-managed-service-auth')
  if ((${#long_name} > 63)); then
    fail "expected truncated run-scoped name <= 63 chars, got ${#long_name}: ${long_name}"
  fi
}

test_operator_handoff_prints_managed_service_specific_commands() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  prepare_operator_handoff_env "${tmp}"
  E2E_MANAGED_SERVICE='simple-api-server'
  E2E_MANUAL_COMPONENT_ACCESS_OUTPUT=$'managed-service:simple-api-server\n  Base URL: http://127.0.0.1:20890/api\n  Auth Mode: oauth2'

  local output
  output=$(e2e_profile_operator_handoff 'e2e-operator')

  assert_contains "${output}" "resource save '/api/projects/operator-demo' --payload"
  assert_contains "${output}" "resource get '/api/projects/operator-demo' --source managed-service"
  assert_contains "${output}" "manager-deployment: declarest-operator"
  assert_contains "${output}" "repository-webhook-url: ${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL}"
  assert_contains "${output}" "kubectl --kubeconfig \"${E2E_KUBECONFIG}\" -n \"${E2E_OPERATOR_NAMESPACE}\" logs deployment/\"${E2E_OPERATOR_MANAGER_DEPLOYMENT}\" --tail=80"
  assert_contains "${output}" "How to connect kubectl to this kind cluster:"
  assert_contains "${output}" "Manual Component Access:"
  assert_contains "${output}" "managed-service:simple-api-server"
  assert_contains "${output}" "Base URL: http://127.0.0.1:20890/api"
  assert_contains "${output}" "Repository provider access:"
  assert_contains "${output}" "web login: http://127.0.0.1:3000/user/login"
  assert_not_contains "${output}" "/customers/demo"

  local manual_line repo_line
  manual_line=$(printf '%s\n' "${output}" | grep -n 'Manual Component Access:' | head -n 1 | cut -d: -f1 || true)
  repo_line=$(printf '%s\n' "${output}" | grep -n 'Repository provider access:' | head -n 1 | cut -d: -f1 || true)
  if [[ -z "${manual_line}" || -z "${repo_line}" ]] || ((manual_line >= repo_line)); then
    fail 'expected Manual Component Access section before Repository provider access'
  fi

  local setup_script reset_script
  setup_script=$(e2e_manual_env_setup_script_path)
  reset_script=$(e2e_manual_env_reset_script_path)
  assert_path_exists "${setup_script}"
  assert_path_exists "${reset_script}"
}

test_operator_prepare_repository_webhook_builds_scoped_url() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_PROFILE='operator-manual'
  export E2E_RUN_ID='operator-webhook-test'
  export E2E_STATE_DIR="${tmp}/state"
  export E2E_K8S_NAMESPACE='declarest-operator'
  export E2E_REPO_TYPE='git'
  export E2E_GIT_PROVIDER='gitea'
  E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER=()
  E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER['git-provider:gitea']='gitea'
  mkdir -p "${E2E_STATE_DIR}"

  e2e_operator_prepare_repository_webhook

  assert_eq "${E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER}" "gitea"
  assert_contains "${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL}" "/webhooks/repository/declarest-operator/"
  assert_contains "${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL}" "declarest-e2e-repository-operator-webhook-test"
  assert_contains "${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL}" "$(e2e_operator_repository_webhook_service_name)"
  if [[ -z "${E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET:-}" ]]; then
    fail "expected operator repository webhook secret to be generated"
  fi
}

test_operator_prepare_repository_webhook_derives_namespace_when_unset() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_PROFILE='operator-manual'
  export E2E_PLATFORM='kubernetes'
  export E2E_RUN_ID='operator-webhook-autons'
  export E2E_STATE_DIR="${tmp}/state"
  unset E2E_K8S_NAMESPACE
  export E2E_REPO_TYPE='git'
  export E2E_GIT_PROVIDER='gitea'
  E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER=()
  E2E_COMPONENT_REPOSITORY_WEBHOOK_PROVIDER['git-provider:gitea']='gitea'
  mkdir -p "${E2E_STATE_DIR}"

  e2e_operator_prepare_repository_webhook

  local expected_namespace='declarest-operator-webhook-autons'
  assert_contains "${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL}" ".${expected_namespace}.svc.cluster.local:18082/"
  assert_contains "${E2E_OPERATOR_REPOSITORY_WEBHOOK_URL}" "/webhooks/repository/${expected_namespace}/"
}

test_operator_repository_webhook_registration_deferred_only_for_operator_profiles() {
  load_operator_libs

  E2E_PROFILE='operator-full'
  E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER='gitlab'
  if ! e2e_operator_should_defer_repository_webhook_registration; then
    fail 'expected operator profile webhook registration to be deferred'
  fi

  E2E_PROFILE='cli-full'
  if e2e_operator_should_defer_repository_webhook_registration; then
    fail 'expected non-operator profile webhook registration not to be deferred'
  fi

  E2E_PROFILE='operator-full'
  unset E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER
  if e2e_operator_should_defer_repository_webhook_registration; then
    fail 'expected empty webhook provider not to trigger deferred registration'
  fi
}

test_operator_configure_repository_webhook_if_needed_runs_git_provider_hook() {
  load_operator_libs

  local recorded=''
  e2e_component_key() {
    printf '%s:%s\n' "$1" "$2"
  }
  e2e_components_run_hook_for_keys() {
    recorded="$*"
  }

  E2E_PROFILE='operator-full'
  E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER='gitlab'
  E2E_REPO_TYPE='git'
  E2E_GIT_PROVIDER='gitlab'

  e2e_operator_configure_repository_webhook_if_needed

  assert_eq "${recorded}" "configure-auth false git-provider:gitlab"
}

test_operator_rewrites_local_urls_for_cluster_services() {
  load_operator_libs

  E2E_PLATFORM='kubernetes'
  E2E_K8S_NAMESPACE='declarest-test'
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_GIT_PROVIDER='gitea'
  E2E_COMPONENT_SERVICE_PORT=()
  E2E_COMPONENT_SERVICE_PORT['git-provider:gitea']='3000'

  local rewritten
  rewritten=$(e2e_operator_rewrite_local_url_to_service 'http://127.0.0.1:3000/root/repo.git' 'git-provider-gitea' '3000')
  assert_eq "${rewritten}" "http://git-provider-gitea.declarest-test.svc.cluster.local:3000/root/repo.git"

  rewritten=$(e2e_operator_rewrite_repo_url_for_cluster 'http://localhost:3000/root/repo.git')
  assert_eq "${rewritten}" "http://git-provider-gitea.declarest-test.svc.cluster.local:3000/root/repo.git"

  rewritten=$(e2e_operator_rewrite_local_url_to_service 'https://example.com/api' 'managed-service-keycloak' '8080')
  assert_eq "${rewritten}" "https://example.com/api"
}

test_operator_ready_timeout_validation_and_cap() {
  load_operator_libs

  E2E_OPERATOR_READY_TIMEOUT_SECONDS=120
  assert_eq "$(e2e_operator_ready_timeout_seconds)" "120"

  E2E_OPERATOR_READY_TIMEOUT_SECONDS=999
  assert_eq "$(e2e_operator_ready_timeout_seconds)" "600"

  local output status
  E2E_OPERATOR_READY_TIMEOUT_SECONDS='0'
  set +e
  output=$(e2e_operator_ready_timeout_seconds 2>&1)
  status=$?
  set -e

  assert_status "${status}" "1"
  assert_contains "${output}" "invalid operator readiness timeout"
}

assert_text_order() {
  local haystack=$1
  local earlier=$2
  local later=$3
  local earlier_line later_line

  earlier_line=$(grep -nF -- "${earlier}" <<<"${haystack}" | head -n 1 | cut -d: -f1 || true)
  later_line=$(grep -nF -- "${later}" <<<"${haystack}" | head -n 1 | cut -d: -f1 || true)
  [[ -n "${earlier_line}" ]] || fail "expected text to contain ${earlier@Q}"
  [[ -n "${later_line}" ]] || fail "expected text to contain ${later@Q}"
  if ((earlier_line >= later_line)); then
    fail "expected ${earlier@Q} before ${later@Q}"
  fi
}

test_operator_olm_install_core_applies_vendored_yaml() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_KUBECONFIG="${tmp}/kubeconfig"
  export E2E_OPERATOR_READY_TIMEOUT_SECONDS=5
  : >"${E2E_KUBECONFIG}"

  local calls="${tmp}/kubectl.calls"
  kubectl() {
    printf '%s\n' "$*" >>"${calls}"
    case "$*" in
      *'get namespace olm'*)
        return 1
        ;;
    esac
    return 0
  }

  e2e_operator_olm_install_core

  local output
  output=$(<"${calls}")
  assert_contains "${output}" "apply --server-side=true -f ${REPO_ROOT}/test/e2e/olm/v0.42.0/crds.yaml"
  assert_contains "${output}" "apply -f ${REPO_ROOT}/test/e2e/olm/v0.42.0/olm.yaml"
  assert_contains "${output}" "wait --for=condition=Established crd/catalogsources.operators.coreos.com"
  assert_contains "${output}" "-n olm wait --for=condition=Available deployment/olm-operator"
  assert_contains "${output}" "-n olm wait --for=condition=Available deployment/catalog-operator"
  assert_contains "${output}" "-n olm wait --for=condition=Available deployment/packageserver"
  assert_contains "${output}" "-n olm delete catalogsource/operatorhubio-catalog --ignore-not-found"
  assert_not_contains "$(declare -f e2e_operator_olm_install_core)" "operator-sdk"
}

test_operator_olm_patch_csv_preserves_webhooks_and_enables_admission() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_RUN_ID='operator-csv-patch'
  export E2E_RUN_DIR="${tmp}/run"
  export E2E_K8S_NAMESPACE='declarest-operator-csv'
  export E2E_OPERATOR_IMAGE='localhost/declarest/e2e-operator-manager:csv-patch'
  export E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE="${tmp}/metadata-bundle.tar.gz"
  : >"${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}"

  local csv_file="${tmp}/declarest-operator.clusterserviceversion.yaml"
  cat >"${csv_file}" <<'EOF'
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: declarest-operator.v0.0.1
spec:
  webhookdefinitions:
    - type: ValidatingAdmissionWebhook
      deploymentName: declarest-operator-webhook
      webhookPath: /validate-declarest-io-v1alpha1-managedservice
  install:
    strategy: deployment
    spec:
      deployments:
        - name: declarest-operator
          spec:
            template:
              spec:
                containers:
                  - name: manager
                    image: ghcr.io/crmarques/declarest-operator:old
                    args:
                      - --health-probe-bind-address=:8081
                      - --enable-admission-webhooks=false
                      - --watch-namespace=old
                      - --repository-webhook-bind-address=:18082
                      - --leader-elect
                    volumeMounts:
                      - name: state
                        mountPath: /var/lib/declarest
                volumes:
                  - name: state
                    persistentVolumeClaim:
                      claimName: declarest-operator-state
                  - name: tmp
                    emptyDir: {}
EOF

  e2e_operator_olm_patch_csv "${csv_file}"

  assert_file_contains "${csv_file}" "webhookdefinitions:"
  assert_file_contains "${csv_file}" "deploymentName: declarest-operator"
  assert_file_contains "${csv_file}" "containerPort: 9443"
  assert_file_contains "${csv_file}" "targetPort: 9443"
  assert_file_contains "${csv_file}" "image: localhost/declarest/e2e-operator-manager:csv-patch"
  assert_file_contains "${csv_file}" "--enable-admission-webhooks=true"
  assert_file_contains "${csv_file}" "--watch-namespace=declarest-operator-csv"
  assert_file_contains "${csv_file}" "--repository-webhook-bind-address=:8082"
  assert_file_contains "${csv_file}" "emptyDir: {}"
  assert_file_contains "${csv_file}" "secretName: $(e2e_operator_managed_service_metadata_bundle_secret_name)"
  assert_file_contains "${csv_file}" "mountPath: $(e2e_operator_managed_service_metadata_bundle_mount_dir)"
  assert_not_contains "$(<"${csv_file}")" "--enable-admission-webhooks=false"
  assert_not_contains "$(<"${csv_file}")" "--watch-namespace=old"
  assert_not_contains "$(<"${csv_file}")" "persistentVolumeClaim"
}

test_operator_olm_apply_waits_for_catalog_before_subscription() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_RUN_ID='operator-olm-order'
  export E2E_RUN_DIR="${tmp}/run"
  export E2E_K8S_NAMESPACE='declarest-operator-order'
  export E2E_KUBECONFIG="${tmp}/kubeconfig"
  export E2E_OPERATOR_READY_TIMEOUT_SECONDS=5
  : >"${E2E_KUBECONFIG}"

  local calls="${tmp}/kubectl.calls"
  kubectl() {
    printf '%s\n' "$*" >>"${calls}"
    case "$*" in
      *'get catalogsource/'*)
        printf 'READY'
        ;;
    esac
    return 0
  }

  e2e_operator_olm_write_install_manifest
  e2e_operator_olm_apply_install_manifest

  local output catalog_manifest subscription_manifest catalog_name
  output=$(<"${calls}")
  catalog_manifest=$(e2e_operator_olm_catalog_manifest_path)
  subscription_manifest=$(e2e_operator_olm_subscription_manifest_path)
  catalog_name=$(e2e_operator_olm_catalog_source_name)

  assert_text_order "${output}" "apply -f ${catalog_manifest}" "get catalogsource/${catalog_name}"
  assert_text_order "${output}" "get catalogsource/${catalog_name}" "apply -f ${subscription_manifest}"
}

test_operator_install_waits_for_olm_before_declarest_crs() {
  load_operator_libs

  local install_via_olm install_stack
  install_via_olm=$(declare -f e2e_operator_install_via_olm)
  install_stack=$(declare -f e2e_operator_install_stack)

  assert_text_order "${install_via_olm}" "e2e_operator_olm_apply_install_manifest" "e2e_operator_olm_wait_for_csv_succeeded"
  assert_text_order "${install_via_olm}" "e2e_operator_olm_wait_for_csv_succeeded" "e2e_operator_olm_wait_for_manager_deployment_ready"
  assert_text_order "${install_stack}" "e2e_operator_install_via_olm" "e2e_operator_write_manifests"
}

test_operator_write_manifests_prefers_prepared_keycloak_metadata_bundle_mount_path() {
  source_e2e_libs common profile operator components

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_RUN_ID='operator-keycloak-bundle-test'
  export E2E_RUN_DIR="${tmp}/run"
  export E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  export E2E_PLATFORM='compose'
  export E2E_REPO_TYPE='git'
  export E2E_GIT_PROVIDER='gitea'
  export E2E_GIT_PROVIDER_CONNECTION='remote'
  export E2E_SECRET_PROVIDER='file'
  export E2E_SECRET_PROVIDER_CONNECTION='local'
  export E2E_MANAGED_SERVICE='keycloak'
  export E2E_MANAGED_SERVICE_CONNECTION='remote'
  export E2E_MANAGED_SERVICE_AUTH_TYPE='oauth2'
  export E2E_MANAGED_SERVICE_MTLS='false'
  export E2E_METADATA_BUNDLE='keycloak-bundle:0.0.1'
  export HOME="${tmp}/home"
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER=''
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET=''
  export E2E_OPERATOR_REPOSITORY_NAME='declarest-e2e-repository'

  mkdir -p "${E2E_STATE_DIR}"
  mkdir -p "${HOME}/.declarest/metadata-bundles/keycloak-bundle-0.0.1/metadata/admin/realms/_"
  cat >"${HOME}/.declarest/metadata-bundles/keycloak-bundle-0.0.1/bundle.yaml" <<'EOF'
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.1
description: E2E metadata bundle for keycloak.
declarest:
  metadataRoot: metadata
EOF
  cat >"${HOME}/.declarest/metadata-bundles/keycloak-bundle-0.0.1/metadata/admin/realms/_/metadata.yaml" <<'EOF'
{"resource":{"id":"{{/realm}}","alias":"{{/realm}}"}}
EOF

  local repo_state managed_state secret_state
  repo_state=$(e2e_component_state_file "$(e2e_component_key 'repo-type' 'git')")
  managed_state=$(e2e_component_state_file "$(e2e_component_key 'managed-service' 'keycloak')")
  secret_state=$(e2e_component_state_file "$(e2e_component_key 'secret-provider' 'file')")

  cat >"${repo_state}" <<'EOF'
GIT_REMOTE_URL=https://example.com/acme/declarest-e2e.git
GIT_REMOTE_BRANCH=main
GIT_AUTH_MODE=access-key
GIT_AUTH_TOKEN=test-token
EOF

  cat >"${managed_state}" <<'EOF'
MANAGED_SERVICE_BASE_URL=https://keycloak.example.com
MANAGED_SERVICE_AUTH_KIND=oauth2
MANAGED_SERVICE_TOKEN_URL=https://keycloak.example.com/realms/master/protocol/openid-connect/token
MANAGED_SERVICE_OAUTH_CLIENT_ID=declarest-e2e-client
MANAGED_SERVICE_OAUTH_CLIENT_SECRET=declarest-e2e-secret
EOF

  cat >"${secret_state}" <<'EOF'
SECRET_FILE_PATH=/tmp/declarest-e2e-secrets.enc.json
SECRET_FILE_PASSPHRASE=test-passphrase
EOF

  e2e_operator_prepare_managed_service_metadata_bundle
  e2e_operator_write_manifests

  local managed_service_manifest
  managed_service_manifest="$(e2e_operator_manifest_dir)/managed-service.yaml"
  assert_file_contains "${managed_service_manifest}" "metadata:"
  assert_file_contains \
    "${managed_service_manifest}" \
    "bundle: '$(e2e_operator_managed_service_metadata_bundle_mount_path)'"
}

test_operator_prepare_managed_service_metadata_bundle_from_metadata_dir() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_RUN_DIR="${tmp}/run"
  export E2E_MANAGED_SERVICE='rundeck'
  export E2E_METADATA_DIR="${tmp}/metadata"
  unset E2E_METADATA_BUNDLE

  mkdir -p "${E2E_RUN_DIR}" "${E2E_METADATA_DIR}/projects/_"
  cat >"${E2E_METADATA_DIR}/projects/_/metadata.yaml" <<'EOF'
{"resource":{"id":"{{/name}}","alias":"{{/name}}"}}
EOF

  e2e_operator_prepare_managed_service_metadata_bundle

  assert_path_exists "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}"
  assert_eq \
    "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_MOUNT_PATH}" \
    "$(e2e_operator_managed_service_metadata_bundle_mount_path)"

  local bundle_manifest archive_listing
  bundle_manifest=$(tar -xOf "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}" bundle.yaml)
  archive_listing=$(tar -tzf "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}")

  assert_contains "${bundle_manifest}" "name: e2e-rundeck-bundle"
  assert_contains "${bundle_manifest}" "metadataRoot: metadata"
  assert_contains "${archive_listing}" "bundle.yaml"
  assert_contains "${archive_listing}" "metadata/projects/_/metadata.yaml"
}

test_operator_prepare_rundeck_component_metadata_bundle_omits_case_only_fixtures() {
  load_operator_libs

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_RUN_DIR="${tmp}/run"
  export E2E_MANAGED_SERVICE='rundeck'
  export E2E_METADATA_DIR="${REPO_ROOT}/test/e2e/components/managed-service/rundeck/metadata"
  unset E2E_METADATA_BUNDLE

  mkdir -p "${E2E_RUN_DIR}"

  e2e_operator_prepare_managed_service_metadata_bundle

  local archive_listing
  archive_listing=$(tar -tzf "${E2E_OPERATOR_MANAGED_SERVICE_METADATA_BUNDLE_ARCHIVE}")

  assert_contains "${archive_listing}" "metadata/projects/_/jobs/_/metadata.yaml"
  assert_not_contains "${archive_listing}" "metadata/projects/platform/jobs/_/metadata.yaml"
  assert_not_contains "${archive_listing}" "metadata/save-input-modes-items/_/metadata.yaml"
  assert_not_contains "${archive_listing}" "metadata/save-secret-guard/metadata/metadata.yaml"
  assert_not_contains "${archive_listing}" "metadata/secret-detect-fix/acme/metadata.yaml"
}

test_operator_write_manifests_uses_prepared_metadata_bundle_mount_path() {
  source_e2e_libs common profile operator components

  local tmp
  tmp=$(new_temp_dir)
  trap 'rm -rf "${tmp}"' RETURN

  export E2E_RUN_ID='operator-rundeck-metadata-test'
  export E2E_RUN_DIR="${tmp}/run"
  export E2E_STATE_DIR="${E2E_RUN_DIR}/state"
  export E2E_PLATFORM='kubernetes'
  export E2E_REPO_TYPE='git'
  export E2E_GIT_PROVIDER='gitea'
  export E2E_GIT_PROVIDER_CONNECTION='remote'
  export E2E_SECRET_PROVIDER='file'
  export E2E_SECRET_PROVIDER_CONNECTION='local'
  export E2E_MANAGED_SERVICE='rundeck'
  export E2E_MANAGED_SERVICE_CONNECTION='remote'
  export E2E_MANAGED_SERVICE_AUTH_TYPE='custom-header'
  export E2E_MANAGED_SERVICE_MTLS='false'
  export E2E_METADATA_DIR="${tmp}/metadata"
  unset E2E_METADATA_BUNDLE
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_PROVIDER=''
  export E2E_OPERATOR_REPOSITORY_WEBHOOK_SECRET=''
  export E2E_OPERATOR_REPOSITORY_NAME='declarest-e2e-repository'

  mkdir -p "${E2E_STATE_DIR}" "${E2E_METADATA_DIR}/projects/_"
  cat >"${E2E_METADATA_DIR}/projects/_/metadata.yaml" <<'EOF'
{"resource":{"id":"{{/name}}","alias":"{{/name}}"}}
EOF

  local repo_state managed_state secret_state
  repo_state=$(e2e_component_state_file "$(e2e_component_key 'repo-type' 'git')")
  managed_state=$(e2e_component_state_file "$(e2e_component_key 'managed-service' 'rundeck')")
  secret_state=$(e2e_component_state_file "$(e2e_component_key 'secret-provider' 'file')")

  cat >"${repo_state}" <<'EOF'
GIT_REMOTE_URL=https://example.com/acme/declarest-e2e.git
GIT_REMOTE_BRANCH=main
GIT_AUTH_MODE=access-key
GIT_AUTH_TOKEN=test-token
EOF

  cat >"${managed_state}" <<'EOF'
MANAGED_SERVICE_BASE_URL=https://rundeck.example.com
MANAGED_SERVICE_AUTH_KIND=custom-header
MANAGED_SERVICE_HEADER_NAME=X-Rundeck-Auth-Token
MANAGED_SERVICE_HEADER_VALUE=test-token
EOF

  cat >"${secret_state}" <<'EOF'
SECRET_FILE_PATH=/tmp/declarest-e2e-secrets.enc.json
SECRET_FILE_PASSPHRASE=test-passphrase
EOF

  e2e_operator_prepare_managed_service_metadata_bundle
  e2e_operator_write_manifests

  local managed_service_manifest
  managed_service_manifest="$(e2e_operator_manifest_dir)/managed-service.yaml"
  assert_file_contains "${managed_service_manifest}" "metadata:"
  assert_file_contains \
    "${managed_service_manifest}" \
    "bundle: '$(e2e_operator_managed_service_metadata_bundle_mount_path)'"
}

test_secretstore_crd_does_not_require_legacy_provider_field() {
  local crd_file="${REPO_ROOT}/config/crd/bases/declarest.io_secretstores.yaml"
  local content
  content=$(<"${crd_file}")

  assert_not_contains "${content}" "- provider"
}

test_operator_example_resource_mapping
test_operator_scoped_names_are_run_specific
test_operator_handoff_prints_managed_service_specific_commands
test_operator_prepare_repository_webhook_builds_scoped_url
test_operator_prepare_repository_webhook_derives_namespace_when_unset
test_operator_repository_webhook_registration_deferred_only_for_operator_profiles
test_operator_configure_repository_webhook_if_needed_runs_git_provider_hook
test_operator_rewrites_local_urls_for_cluster_services
test_operator_ready_timeout_validation_and_cap
test_operator_olm_install_core_applies_vendored_yaml
test_operator_olm_patch_csv_preserves_webhooks_and_enables_admission
test_operator_olm_apply_waits_for_catalog_before_subscription
test_operator_install_waits_for_olm_before_declarest_crs
test_operator_write_manifests_prefers_prepared_keycloak_metadata_bundle_mount_path
test_operator_prepare_managed_service_metadata_bundle_from_metadata_dir
test_operator_prepare_rundeck_component_metadata_bundle_omits_case_only_fixtures
test_operator_write_manifests_uses_prepared_metadata_bundle_mount_path
test_secretstore_crd_does_not_require_legacy_provider_field
