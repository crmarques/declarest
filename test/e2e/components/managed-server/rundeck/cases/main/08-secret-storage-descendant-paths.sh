#!/usr/bin/env bash

CASE_ID='rundeck-secret-storage-descendant-paths'
CASE_SCOPE='main'
CASE_REQUIRES='managed-server=rundeck has-secret-provider'

case_run() {
  local project_name="platform-desc-secret-${RANDOM}${RANDOM}"
  local project_path="/projects/${project_name}"
  local nested_dir='path/to'
  local secret_name='db-password'
  local secret_path="${project_path}/secrets/${nested_dir}/${secret_name}"
  local project_source_file
  local project_payload_file="${E2E_CASE_TMP_DIR}/project.json"
  local secret_metadata_file="${E2E_CASE_TMP_DIR}/secret-metadata.json"
  local secret_payload_file="${E2E_CASE_TMP_DIR}/secret.json"
  local secret_value='super-secret-descendant'

  case_run_declarest secret init
  case_expect_success

  project_source_file=$(case_repo_template_resource_file_for_path '/projects/platform' 'rundeck') || return 1
  jq \
    --arg name "${project_name}" \
    '
      .name = $name
      | .description = "Managed by declarest E2E descendant secret case"
      | .config["project.label"] = $name
      | .config["project.description"] = "Managed by declarest E2E descendant secret case"
    ' \
    "${project_source_file}" >"${project_payload_file}"

  case_run_declarest resource create "${project_path}" -f "${project_payload_file}" --content-type json
  case_expect_success

  case_write_json "${secret_metadata_file}" '{
    "resource": {
      "secretAttributes": ["/content"]
    }
  }'

  case_run_declarest resource metadata set "${secret_path}" -f "${secret_metadata_file}" --content-type json
  case_expect_success
  case_repo_commit_setup_changes_if_git

  jq \
    -n \
    --arg name "${secret_name}" \
    --arg content "${secret_value}" \
    '{
      name: $name,
      type: "password",
      contentType: "application/x-rundeck-data-password",
      content: $content
    }' >"${secret_payload_file}"

  case_run_declarest resource save "${secret_path}" -f "${secret_payload_file}" --content-type json
  case_expect_success

  case_run_declarest resource get "${secret_path}" --source repository -o json
  case_expect_success
  if ! jq -e \
    --arg name "${secret_name}" \
    '.name == $name and .type == "password" and .contentType == "application/x-rundeck-data-password" and .content == "{{secret .}}"' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected repository payload to mask nested secret content with a secret placeholder\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource get "${secret_path}" --source repository --show-secrets -o json
  case_expect_success
  if [[ "$(jq -r '.content' <<<"${CASE_LAST_STDOUT}")" != "${secret_value}" ]]; then
    printf 'expected --show-secrets to resolve nested secret content from the secret store\n' >&2
    return 1
  fi

  case_run_declarest secret get "${secret_path}:/content"
  case_expect_success
  if [[ "${CASE_LAST_STDOUT}" != "${secret_value}" ]]; then
    printf 'expected secret store lookup for nested secret path to return the original value\n' >&2
    return 1
  fi

  case_run_declarest resource get "${secret_path}" --source repository --show-metadata -o json
  case_expect_success
  if ! jq -e \
    --arg path "/storage/keys/project/${project_name}/path/to" \
    '.metadata.resource.remoteCollectionPath == $path' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected rendered metadata snapshot to keep descendant-aware remoteCollectionPath\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource apply "${secret_path}"
  case_expect_success

  case_run_declarest resource diff "${secret_path}" -o json
  case_expect_success
  if ! jq -e '. == []' <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected nested secret diff to stay idempotent after apply\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource get "${secret_path}" --source managed-server -o json
  case_expect_success
  if ! jq -e \
    --arg name "${secret_name}" \
    '.name == $name and .type == "password" and .contentType == "application/x-rundeck-data-password"' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected Rundeck managed-server output to describe the nested password secret\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource list "${project_path}/secrets/${nested_dir}" --source managed-server -o json
  case_expect_success
  if ! jq -e \
    --arg name "${secret_name}" \
    'map(select(.name == $name and .type == "password")) | length == 1' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected nested secrets collection listing to include the saved secret metadata\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi

  case_run_declarest resource list "${project_path}/secrets" --source managed-server -o json
  case_expect_success
  if ! jq -e \
    --arg name "${secret_name}" \
    'map(select(.name == $name)) | length == 0' \
    <<<"${CASE_LAST_STDOUT}" >/dev/null; then
    printf 'expected root secrets collection listing to exclude the nested child directly\n' >&2
    printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}
