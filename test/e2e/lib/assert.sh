#!/usr/bin/env bash

CASE_LAST_OUTPUT=''
CASE_LAST_STATUS=0

case_run_declarest() {
  local output

  set +e
  output=$(DECLAREST_CONTEXTS_FILE="${E2E_CONTEXT_FILE}" "${E2E_BIN}" --context "${E2E_CONTEXT_NAME}" "$@" 2>&1)
  CASE_LAST_STATUS=$?
  set -e

  CASE_LAST_OUTPUT="${output}"
  return 0
}

case_expect_status() {
  local expected=$1
  if ((CASE_LAST_STATUS != expected)); then
    printf 'expected exit status %d but got %d\n' "${expected}" "${CASE_LAST_STATUS}" >&2
    printf 'command output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_expect_success() {
  case_expect_status 0
}

case_expect_failure() {
  if ((CASE_LAST_STATUS == 0)); then
    printf 'expected command failure but got success\n' >&2
    printf 'command output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_expect_output_contains() {
  local expected=$1
  if ! grep -Fq -- "${expected}" <<<"${CASE_LAST_OUTPUT}"; then
    printf 'expected output to contain: %s\n' "${expected}" >&2
    printf 'actual output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_expect_output_not_contains() {
  local forbidden=$1
  if grep -Fq -- "${forbidden}" <<<"${CASE_LAST_OUTPUT}"; then
    printf 'expected output not to contain: %s\n' "${forbidden}" >&2
    printf 'actual output:\n%s\n' "${CASE_LAST_OUTPUT}" >&2
    return 1
  fi
}

case_wait_until() {
  local timeout_seconds=$1
  local interval_seconds=$2
  local description=$3
  shift 3

  if (($# == 0)); then
    printf 'case_wait_until requires a command to execute\n' >&2
    return 1
  fi

  local start_epoch
  local now_epoch
  local elapsed
  local rc
  local had_errexit=0
  local last_output=''
  local output_file

  if [[ $- == *e* ]]; then
    had_errexit=1
  fi

  output_file=$(mktemp /tmp/declarest-e2e-case-wait.XXXXXX) || {
    printf 'failed to allocate temp file for case_wait_until output capture\n' >&2
    return 1
  }

  start_epoch=$(date +%s)
  while true; do
    set +e
    "$@" >"${output_file}" 2>&1
    rc=$?
    if ((had_errexit == 1)); then
      set -e
    fi
    last_output=$(cat "${output_file}" 2>/dev/null || true)

    if ((rc == 0)); then
      rm -f "${output_file}" || true
      return 0
    fi

    now_epoch=$(date +%s)
    elapsed=$((now_epoch - start_epoch))
    if ((elapsed >= timeout_seconds)); then
      printf 'timed out waiting for %s after %ss\n' "${description}" "${timeout_seconds}" >&2
      if [[ -n "${last_output}" ]]; then
        printf 'last attempt output:\n%s\n' "${last_output}" >&2
      fi
      rm -f "${output_file}" || true
      return 1
    fi

    sleep "${interval_seconds}"
  done
}

case_write_json() {
  local path=$1
  local payload=$2
  printf '%s\n' "${payload}" >"${path}"
}

case_jq_value() {
  local jq_expr=$1
  jq -r "${jq_expr}" <<<"${CASE_LAST_OUTPUT}"
}

case_repo_template_root() {
  local component_name=${1:-${E2E_RESOURCE_SERVER:-}}
  if [[ -z "${component_name}" || "${component_name}" == 'none' ]]; then
    printf 'repo-template requested but no resource-server component is selected\n' >&2
    return 1
  fi

  local template_root="${E2E_DIR}/components/resource-server/${component_name}/repo-template"
  if [[ ! -d "${template_root}" ]]; then
    printf 'resource-server repo-template not found: %s\n' "${template_root}" >&2
    return 1
  fi

  printf '%s\n' "${template_root}"
}

case_component_metadata_root() {
  local component_name=${1:-${E2E_RESOURCE_SERVER:-}}
  if [[ -z "${component_name}" || "${component_name}" == 'none' ]]; then
    printf 'metadata requested but no resource-server component is selected\n' >&2
    return 1
  fi

  local metadata_root="${E2E_DIR}/components/resource-server/${component_name}/metadata"
  if [[ -d "${metadata_root}" ]]; then
    printf '%s\n' "${metadata_root}"
    return 0
  fi

  case_repo_template_root "${component_name}"
}

case_repo_template_metadata_logical_path() {
  local metadata_file=$1
  local component_name=${2:-}
  local metadata_root
  local rel_path
  local logical_path

  metadata_root=$(case_component_metadata_root "${component_name}") || return 1
  rel_path=${metadata_file#${metadata_root}/}
  logical_path=/${rel_path%/metadata.json}
  logical_path=${logical_path%/}

  if [[ ! "${logical_path}" =~ /_($|/) ]]; then
    printf 'metadata logical path must include collection marker (_): %s\n' "${logical_path}" >&2
    return 1
  fi

  printf '%s\n' "${logical_path}"
}

case_repo_template_metadata_target_paths() {
  local metadata_logical_path=$1
  local component_name=${2:-}
  local -a segments=()
  local -a pattern=()
  local -a resource_files=()
  local -a resource_paths=()
  local -a matches=()
  local segment
  local resource_file
  local logical_path
  local index
  local has_intermediary_placeholder=0

  IFS='/' read -r -a segments <<<"${metadata_logical_path#/}"
  if ((${#segments[@]} == 0)); then
    printf 'invalid metadata logical path: %s\n' "${metadata_logical_path}" >&2
    return 1
  fi
  if [[ "${segments[${#segments[@]}-1]}" != '_' ]]; then
    printf 'metadata logical path must end with collection marker (_): %s\n' "${metadata_logical_path}" >&2
    return 1
  fi

  local last_index
  last_index=$((${#segments[@]} - 1))
  for index in "${!segments[@]}"; do
    segment=${segments[${index}]}
    [[ -n "${segment}" ]] || continue
    if ((index < last_index)) && [[ "${segment}" == '_' ]]; then
      has_intermediary_placeholder=1
    fi
    if ((index < last_index)); then
      pattern+=("${segment}")
    fi
  done

  if ((has_intermediary_placeholder == 0)); then
    printf '%s\n' "${metadata_logical_path}"
    return 0
  fi

  while IFS= read -r resource_file; do
    [[ -n "${resource_file}" ]] || continue
    resource_files+=("${resource_file}")
  done < <(case_repo_template_resource_files "${component_name}")

  for resource_file in "${resource_files[@]}"; do
    logical_path=$(case_repo_template_resource_logical_path "${resource_file}" "${component_name}") || return 1
    resource_paths+=("${logical_path}")
  done

  local -A seen=()
  local resource_path
  for resource_path in "${resource_paths[@]}"; do
    local -a resource_segments=()
    IFS='/' read -r -a resource_segments <<<"${resource_path#/}"

    if ((${#resource_segments[@]} < ${#pattern[@]})); then
      continue
    fi

    local -a concrete=()
    local match=1
    for index in "${!pattern[@]}"; do
      local expected=${pattern[${index}]}
      local actual=${resource_segments[${index}]}

      if [[ -z "${actual}" ]]; then
        match=0
        break
      fi

      if [[ "${expected}" == '_' ]]; then
        concrete+=("${actual}")
        continue
      fi

      if [[ "${expected}" != "${actual}" ]]; then
        match=0
        break
      fi
      concrete+=("${actual}")
    done

    if ((match == 0)); then
      continue
    fi

    local target
    target="/$(IFS=/; printf '%s' "${concrete[*]}")/_"
    seen["${target}"]=1
  done

  if ((${#seen[@]} == 0)); then
    # Some component metadata paths are intentionally ahead of the sample repo-template fixtures.
    # Return an empty result so generic metadata loaders can skip them without per-component logic.
    return 0
  fi

  local target
  for target in "${!seen[@]}"; do
    printf '%s\n' "${target}"
  done | sort
}

case_repo_template_resource_logical_path() {
  local resource_file=$1
  local component_name=${2:-}
  local template_root
  local rel_path
  local logical_path

  template_root=$(case_repo_template_root "${component_name}") || return 1
  rel_path=${resource_file#${template_root}/}

  if [[ "$(basename -- "${rel_path}")" != 'resource.json' ]]; then
    printf 'resource file must be named resource.json in repo-template: %s\n' "${rel_path}" >&2
    return 1
  fi

  logical_path=/${rel_path%/resource.json}
  logical_path=${logical_path%/}
  if [[ -z "${logical_path}" ]]; then
    logical_path='/'
  fi

  if [[ "${logical_path}" == *"/_/"* || "${logical_path}" == */_ ]]; then
    printf 'resource file resolved to invalid logical path containing metadata marker: %s\n' "${logical_path}" >&2
    return 1
  fi

  printf '%s\n' "${logical_path}"
}

case_repo_template_resource_file_for_path() {
  local logical_path=$1
  local component_name=${2:-}
  local template_root
  local resource_file

  template_root=$(case_repo_template_root "${component_name}") || return 1
  if [[ "${logical_path}" == '/' ]]; then
    resource_file="${template_root}/resource.json"
  else
    resource_file="${template_root}/${logical_path#/}/resource.json"
  fi

  if [[ ! -f "${resource_file}" ]]; then
    printf 'repo-template resource file not found for %s: %s\n' "${logical_path}" "${resource_file}" >&2
    return 1
  fi

  printf '%s\n' "${resource_file}"
}

case_repo_template_collection_path_for_resource() {
  local logical_resource_path=$1
  local collection_path

  collection_path=${logical_resource_path%/*}
  if [[ -z "${collection_path}" ]]; then
    collection_path='/'
  fi
  printf '%s\n' "${collection_path}"
}

case_repo_template_metadata_files() {
  local component_name=${1:-}
  local metadata_root

  metadata_root=$(case_component_metadata_root "${component_name}") || return 1
  find "${metadata_root}" -type f -path '*/_/metadata.json' | sort
}

case_repo_template_resource_files() {
  local component_name=${1:-}
  local template_root
  local resource_file

  template_root=$(case_repo_template_root "${component_name}") || return 1

  while IFS= read -r resource_file; do
    [[ -n "${resource_file}" ]] || continue
    local rel_path
    local depth
    rel_path=${resource_file#${template_root}/}
    depth=$(tr -cd '/' <<<"${rel_path}" | wc -c)
    printf '%04d\t%s\n' "${depth}" "${resource_file}"
  done < <(find "${template_root}" -type f -name 'resource.json' | sort) \
    | sort -k1,1n -k2,2 \
    | cut -f2-
}

case_repo_template_first_resource_path() {
  local component_name=${1:-}
  local resource_file

  resource_file=$(case_repo_template_resource_files "${component_name}" | head -n 1)
  [[ -n "${resource_file}" ]] || {
    printf 'repo-template has no resource fixtures\n' >&2
    return 1
  }

  case_repo_template_resource_logical_path "${resource_file}" "${component_name}"
}

case_repo_template_collection_resource_paths() {
  local collection_path=$1
  local component_name=${2:-}
  local resource_file
  local logical_path

  while IFS= read -r resource_file; do
    [[ -n "${resource_file}" ]] || continue
    logical_path=$(case_repo_template_resource_logical_path "${resource_file}" "${component_name}") || return 1
    if [[ "$(case_repo_template_collection_path_for_resource "${logical_path}")" == "${collection_path}" ]]; then
      printf '%s\n' "${logical_path}"
    fi
  done < <(case_repo_template_resource_files "${component_name}")
}

case_repo_template_apply_all_metadata() {
  local component_name=${1:-}
  local metadata_file
  local logical_path

  while IFS= read -r metadata_file; do
    [[ -n "${metadata_file}" ]] || continue
    logical_path=$(case_repo_template_metadata_logical_path "${metadata_file}" "${component_name}") || return 1

    local target_path
    while IFS= read -r target_path; do
      [[ -n "${target_path}" ]] || continue
      case_run_declarest metadata set "${target_path}" -f "${metadata_file}" -i json
      case_expect_success
    done < <(case_repo_template_metadata_target_paths "${logical_path}" "${component_name}")
  done < <(case_repo_template_metadata_files "${component_name}")
}

case_repo_template_sync_tree() {
  local component_name=${1:-}
  local revision_prefix=${2:-rev}
  local case_label=${3:-repo-template}

  case_repo_template_apply_all_metadata "${component_name}"

  local -a template_files=()
  local -a logical_paths=()
  local resource_file
  while IFS= read -r resource_file; do
    [[ -n "${resource_file}" ]] || continue
    template_files+=("${resource_file}")
    logical_paths+=("$(case_repo_template_resource_logical_path "${resource_file}" "${component_name}")")
  done < <(case_repo_template_resource_files "${component_name}")

  local index
  for ((index = ${#logical_paths[@]} - 1; index >= 0; index--)); do
    local logical_path=${logical_paths[${index}]}
    case_run_declarest resource delete "${logical_path}" -y
    if ((CASE_LAST_STATUS != 0)) && ! grep -qi 'not found' <<<"${CASE_LAST_OUTPUT}"; then
      printf '%s pre-clean delete failed for %s\n' "${case_label}" "${logical_path}" >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi
  done

  local iteration=0
  local -a created_paths=()
  for index in "${!template_files[@]}"; do
    resource_file=${template_files[${index}]}
    iteration=$((iteration + 1))

    local logical_path
    local collection_path
    local update_payload_file
    local update_tag
    local safe_name

    logical_path=${logical_paths[${index}]}
    collection_path=$(case_repo_template_collection_path_for_resource "${logical_path}") || return 1
    created_paths+=("${logical_path}")
    safe_name=${logical_path#/}
    safe_name=${safe_name//\//__}
    update_payload_file="${E2E_CASE_TMP_DIR}/update-${safe_name}.json"
    update_tag="${revision_prefix}-${iteration}"

    case_run_declarest resource create "${logical_path}" -f "${resource_file}" -i json
    case_expect_success

    case_run_declarest resource list "${collection_path}" --remote-server -o json
    case_expect_success
    if ! jq -e 'type == "array" and (map(tojson) as $items | $items == ($items | sort))' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
      printf '%s expected deterministic sorted remote list for %s\n' "${case_label}" "${collection_path}" >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi

    case_run_declarest resource save "${logical_path}" -f "${resource_file}" -i json
    case_expect_success

    case_run_declarest resource apply "${logical_path}"
    case_expect_success

    jq --arg tag "${update_tag}" '. + {"e2eRevision": $tag}' "${resource_file}" >"${update_payload_file}"
    case_repo_template_update_resource_path "${logical_path}" "${update_payload_file}"

    case_run_declarest resource diff "${logical_path}" -o json
    case_expect_success
    case_expect_output_contains '[]'
  done

  for ((index = ${#created_paths[@]} - 1; index >= 0; index--)); do
    local logical_path=${created_paths[${index}]}
    local collection_path

    collection_path=$(case_repo_template_collection_path_for_resource "${logical_path}") || return 1

    case_run_declarest resource delete "${logical_path}" -y
    case_expect_success

    case_run_declarest resource list "${collection_path}" --remote-server -o json
    if ((CASE_LAST_STATUS != 0)); then
      if grep -qi 'status 404' <<<"${CASE_LAST_OUTPUT}"; then
        continue
      fi
      case_expect_success
    fi
    if ! jq -e 'type == "array" and (map(tojson) as $items | $items == ($items | sort))' <<<"${CASE_LAST_OUTPUT}" >/dev/null; then
      printf '%s expected deterministic sorted remote list after delete for %s\n' "${case_label}" "${collection_path}" >&2
      printf 'output: %s\n' "${CASE_LAST_OUTPUT}" >&2
      return 1
    fi
  done
}

case_repo_template_save_resource_path() {
  local logical_resource_path=$1
  local component_name=${2:-}
  local resource_file

  resource_file=$(case_repo_template_resource_file_for_path "${logical_resource_path}" "${component_name}") || return 1
  case_run_declarest resource save "${logical_resource_path}" -f "${resource_file}" -i json
  case_expect_success
}

case_repo_template_create_resource_path() {
  local logical_resource_path=$1
  local component_name=${2:-}
  local resource_file

  resource_file=$(case_repo_template_resource_file_for_path "${logical_resource_path}" "${component_name}") || return 1
  case_run_declarest resource create "${logical_resource_path}" -f "${resource_file}" -i json
  case_expect_success
}

case_repo_template_update_resource_path() {
  local logical_resource_path=$1
  local payload_file=$2
  case_run_declarest resource save "${logical_resource_path}" -f "${payload_file}" -i json
  case_expect_success
  case_run_declarest resource update "${logical_resource_path}"
  case_expect_success
}
