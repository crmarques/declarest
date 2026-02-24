#!/usr/bin/env bash

CASE_ID='metadata-expansion-multi-wildcard'
CASE_SCOPE='corner'
CASE_REQUIRES='has-resource-server'

metadata_path_has_intermediary_placeholder() {
  local logical_path=$1
  local without_last_marker

  without_last_marker=${logical_path%/_}
  [[ "${without_last_marker}" == *"/_/"* ]]
}

case_run() {
  local metadata_file
  local logical_path
  local target_path
  local -a target_paths=()
  local -a second_pass_targets=()
  local matched=0
  local expanded=0

  while IFS= read -r metadata_file; do
    [[ -n "${metadata_file}" ]] || continue
    logical_path=$(case_repo_template_metadata_logical_path "${metadata_file}") || return 1
    if ! metadata_path_has_intermediary_placeholder "${logical_path}"; then
      continue
    fi
    matched=1

    target_paths=()
    while IFS= read -r target_path; do
      [[ -n "${target_path}" ]] || continue
      target_paths+=("${target_path}")
    done < <(case_repo_template_metadata_target_paths "${logical_path}")

    if ((${#target_paths[@]} == 0)); then
      printf 'metadata path has no concrete fixture targets (skipping): %s\n' "${logical_path}" >&2
      continue
    fi
    expanded=1

    second_pass_targets=()
    while IFS= read -r target_path; do
      [[ -n "${target_path}" ]] || continue
      second_pass_targets+=("${target_path}")
    done < <(case_repo_template_metadata_target_paths "${logical_path}")

    if [[ "${target_paths[*]}" != "${second_pass_targets[*]}" ]]; then
      printf 'metadata expansion is not deterministic for %s\n' "${logical_path}" >&2
      printf 'first:  %s\n' "${target_paths[*]}" >&2
      printf 'second: %s\n' "${second_pass_targets[*]}" >&2
      return 1
    fi

    for target_path in "${target_paths[@]}"; do
      if [[ "${target_path}" != */_ ]]; then
        printf 'expanded metadata target must end with /_: %s\n' "${target_path}" >&2
        return 1
      fi
      if [[ "${target_path}" == *"/_/"* ]]; then
        printf 'expanded metadata target still contains intermediary placeholder: %s\n' "${target_path}" >&2
        return 1
      fi
    done
  done < <(case_repo_template_metadata_files)

  if ((matched == 0)); then
    printf 'no metadata paths with intermediary placeholders found for resource-server=%s\n' "${E2E_RESOURCE_SERVER}" >&2
    return 1
  fi

  if ((expanded == 0)); then
    printf 'no metadata paths with intermediary placeholders expanded to concrete targets for resource-server=%s\n' "${E2E_RESOURCE_SERVER}" >&2
    return 1
  fi
}
