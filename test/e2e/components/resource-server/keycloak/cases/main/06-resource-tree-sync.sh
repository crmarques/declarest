#!/usr/bin/env bash

CASE_ID='keycloak-resource-tree-sync'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=keycloak'

case_run() {
  local metadata_file="${E2E_DIR}/components/resource-server/keycloak/repo-template/admin/realms/_/metadata.json"
  local collection_path='/admin/realms'
  local -a realm_paths=(
    '/admin/realms/payments'
    '/admin/realms/platform'
  )

  case_run_declarest metadata set '/admin/realms/_' -f "${metadata_file}" -i json
  case_expect_success

  local realm_path
  local resource_file
  local update_payload_file
  local revision_index=0
  local reverse_index

  for realm_path in "${realm_paths[@]}"; do
    resource_file=$(case_repo_template_resource_file_for_path "${realm_path}" 'keycloak') || return 1

    case_run_declarest resource delete "${realm_path}" -y
    if ((CASE_LAST_STATUS != 0)) && ! grep -qi 'not found' <<<"${CASE_LAST_OUTPUT}"; then
      printf 'keycloak-sync pre-clean delete failed for %s\n' "${realm_path}" >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi

    case_run_declarest resource create "${realm_path}" -f "${resource_file}" -i json
    case_expect_success

    case_run_declarest resource list "${collection_path}" --remote-server -o json
    case_expect_success
    if ! jq -e 'map(.LogicalPath) as $paths | $paths == ($paths | sort)' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
      printf 'keycloak-sync expected deterministic sorted remote list for %s\n' "${collection_path}" >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi

    case_run_declarest resource save "${realm_path}" -f "${resource_file}" -i json
    case_expect_success

    case_run_declarest resource apply "${realm_path}"
    case_expect_success

    revision_index=$((revision_index + 1))
    update_payload_file="${E2E_CASE_TMP_DIR}/realm-${revision_index}.json"
    jq --arg tag "keycloak-rev-${revision_index}" '. + {"e2eRevision": $tag}' "${resource_file}" >"${update_payload_file}"
    case_repo_template_update_resource_path "${realm_path}" "${update_payload_file}"

    case_run_declarest resource diff "${realm_path}" -o json
    case_expect_success
    case_expect_output_contains '[]'
  done

  for ((reverse_index = ${#realm_paths[@]} - 1; reverse_index >= 0; reverse_index--)); do
    realm_path=${realm_paths[${reverse_index}]}

    case_run_declarest resource delete "${realm_path}" -y
    case_expect_success

    case_run_declarest resource list "${collection_path}" --remote-server -o json
    case_expect_success
    if ! jq -e 'map(.LogicalPath) as $paths | $paths == ($paths | sort)' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
      printf 'keycloak-sync expected deterministic sorted remote list after delete for %s\n' "${collection_path}" >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi
  done
}
