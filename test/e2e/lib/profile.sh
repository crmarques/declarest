#!/usr/bin/env bash

e2e_apply_profile_defaults() {
  # Profiles share the same component defaults from args parsing.
  # Manual mode only changes workload behavior (handoff vs automated cases).
  return 0
}

e2e_validate_profile_rules() {
  if [[ "${E2E_PROFILE}" != 'manual' ]]; then
    return 0
  fi

  if [[ "${E2E_RESOURCE_SERVER_CONNECTION}" != 'local' && "${E2E_RESOURCE_SERVER}" != 'none' ]]; then
    e2e_die 'manual profile is local-instantiable only; resource-server connection must be local'
    return 1
  fi

  if [[ "${E2E_GIT_PROVIDER_CONNECTION}" != 'local' && -n "${E2E_GIT_PROVIDER}" ]]; then
    e2e_die 'manual profile is local-instantiable only; git-provider connection must be local'
    return 1
  fi

  if [[ "${E2E_SECRET_PROVIDER_CONNECTION}" != 'local' && "${E2E_SECRET_PROVIDER}" != 'none' ]]; then
    e2e_die 'manual profile is local-instantiable only; secret-provider connection must be local'
    return 1
  fi
}

e2e_profile_scopes() {
  case "${E2E_PROFILE}" in
    basic)
      printf 'main\n'
      ;;
    full)
      printf 'main\ncorner\n'
      ;;
    manual)
      ;;
  esac
}

e2e_manual_env_setup_script_path() {
  printf '%s/%s\n' "${E2E_RUN_DIR}" 'declarest-e2e-env.sh'
}

e2e_manual_env_reset_script_path() {
  printf '%s/%s\n' "${E2E_RUN_DIR}" 'declarest-e2e-env-reset.sh'
}

e2e_manual_collect_state_env_keys() {
  local state_file
  local key

  while IFS= read -r state_file; do
    [[ -f "${state_file}" ]] || continue

    while IFS='=' read -r key _; do
      [[ -n "${key}" ]] || continue
      [[ "${key}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
      printf '%s\n' "${key}"
    done <"${state_file}"
  done < <(find "${E2E_STATE_DIR}" -maxdepth 1 -type f -name '*.env' | sort)
}

e2e_manual_write_env_setup_script() {
  local context_name=$1
  local setup_script=$2
  local reset_script=$3
  local state_key_list=$4
  local state_file
  local has_state_files=0

  cat >"${setup_script}" <<EOF
#!/usr/bin/env bash

if [[ "\${BASH_SOURCE[0]}" == "\${0}" ]]; then
  printf '%s\n' "this script must be sourced: source ${setup_script}" >&2
  exit 1
fi

export DECLAREST_E2E_RUN_ID=${E2E_RUN_ID@Q}
export DECLAREST_E2E_RUN_DIR=${E2E_RUN_DIR@Q}
export DECLAREST_E2E_RUNS_DIR=${E2E_RUNS_DIR@Q}
export DECLAREST_CONTEXTS_FILE=${E2E_CONTEXT_FILE@Q}
export DECLAREST_E2E_CONTEXT=${context_name@Q}
export DECLAREST_E2E_BIN=${E2E_BIN@Q}
export DECLAREST_E2E_ENV_SETUP_SCRIPT=${setup_script@Q}
export DECLAREST_E2E_ENV_RESET_SCRIPT=${reset_script@Q}
export DECLAREST_E2E_STATE_ENV_KEYS=${state_key_list@Q}

if [[ -z "\${DECLAREST_E2E_ORIGINAL_PATH+x}" ]]; then
  export DECLAREST_E2E_ORIGINAL_PATH="\${PATH}"
fi

case ":\${PATH}:" in
  *":\${DECLAREST_E2E_RUN_DIR}/bin:"*) ;;
  *) export PATH="\${DECLAREST_E2E_RUN_DIR}/bin:\${PATH}" ;;
esac

__declarest_e2e_prune_deleted_run_bins_from_path() {
  local runs_dir="\${DECLAREST_E2E_RUNS_DIR:-}"
  [[ -n "\${runs_dir}" ]] || return 0

  local path_value="\${PATH:-}"
  local -a path_entries=()
  local -a kept=()
  local entry
  local removed=0
  local IFS=':'

  read -ra path_entries <<< "\${path_value}"
  for entry in "\${path_entries[@]}"; do
    if [[ "\${entry}" == "\${runs_dir}/"*"/bin" && ! -d "\${entry}" ]]; then
      removed=1
      continue
    fi
    kept+=("\${entry}")
  done

  if ((removed == 1)); then
    if ((\${#kept[@]} == 0)); then
      export PATH=''
    else
      local last_index=\$(( \${#kept[@]} - 1 ))
      local last_entry=\${kept[\${last_index}]}
      local new_path

      printf -v new_path '%s:' "\${kept[@]}"
      if [[ -z "\${last_entry}" ]]; then
        export PATH="\${new_path}"
      else
        export PATH="\${new_path%:}"
      fi
    fi
  fi

  if [[ -n "\${DECLAREST_E2E_BIN:-}" && ! -x "\${DECLAREST_E2E_BIN}" ]]; then
    unalias declarest-e2e >/dev/null 2>&1 || true
  fi
}

if [[ -z "\${DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET+x}" ]]; then
  if [[ -n "\${PROMPT_COMMAND+x}" ]]; then
    export DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND="\${PROMPT_COMMAND}"
    export DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET='1'
  else
    export DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND=''
    export DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET='0'
  fi
fi

case ";\${PROMPT_COMMAND:-};" in
  *";__declarest_e2e_prune_deleted_run_bins_from_path;"*) ;;
  *)
    if [[ -n "\${PROMPT_COMMAND:-}" ]]; then
      export PROMPT_COMMAND="__declarest_e2e_prune_deleted_run_bins_from_path; \${PROMPT_COMMAND}"
    else
      export PROMPT_COMMAND='__declarest_e2e_prune_deleted_run_bins_from_path'
    fi
    ;;
esac

__declarest_e2e_prune_deleted_run_bins_from_path

EOF

  while IFS= read -r state_file; do
    [[ -f "${state_file}" ]] || continue

    if ((has_state_files == 0)); then
      cat >>"${setup_script}" <<'EOF'
# Export component runtime state values captured during this run.
set -a
EOF
      has_state_files=1
    fi

    printf '# shellcheck disable=SC1090\nsource %q\n' "${state_file}" >>"${setup_script}"
  done < <(find "${E2E_STATE_DIR}" -maxdepth 1 -type f -name '*.env' | sort)

  if ((has_state_files == 1)); then
    cat >>"${setup_script}" <<'EOF'
set +a

EOF
  fi

  cat >>"${setup_script}" <<'EOF'
alias declarest-e2e="${DECLAREST_E2E_BIN}"

printf '%s\n' 'declarest e2e shell environment is active.'
printf '%s\n' 'run commands with: declarest-e2e --context "${DECLAREST_E2E_CONTEXT}" <command>'
printf '%s\n' 'reset with: source "${DECLAREST_E2E_ENV_RESET_SCRIPT}"'
EOF
}

e2e_manual_write_env_reset_script() {
  local reset_script=$1

  cat >"${reset_script}" <<'EOF'
#!/usr/bin/env bash

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  printf '%s\n' "this script must be sourced: source ${BASH_SOURCE[0]}" >&2
  exit 1
fi

unalias declarest-e2e >/dev/null 2>&1 || true
unset -f __declarest_e2e_prune_deleted_run_bins_from_path >/dev/null 2>&1 || true

if [[ "${DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET:-}" == '1' ]]; then
  export PROMPT_COMMAND="${DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND}"
elif [[ "${DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET:-}" == '0' ]]; then
  unset PROMPT_COMMAND || true
fi
unset DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND
unset DECLAREST_E2E_ORIGINAL_PROMPT_COMMAND_SET

if [[ -n "${DECLAREST_E2E_ORIGINAL_PATH+x}" ]]; then
  export PATH="${DECLAREST_E2E_ORIGINAL_PATH}"
fi
unset DECLAREST_E2E_ORIGINAL_PATH

for state_var in ${DECLAREST_E2E_STATE_ENV_KEYS:-}; do
  unset "${state_var}"
done

unset DECLAREST_E2E_STATE_ENV_KEYS
unset DECLAREST_E2E_ENV_SETUP_SCRIPT
unset DECLAREST_E2E_ENV_RESET_SCRIPT
unset DECLAREST_E2E_BIN
unset DECLAREST_E2E_CONTEXT
unset DECLAREST_E2E_RUNS_DIR
unset DECLAREST_CONTEXTS_FILE
unset DECLAREST_E2E_RUN_DIR
unset DECLAREST_E2E_RUN_ID

printf '%s\n' 'declarest e2e shell environment was reset.'
EOF
}

e2e_manual_write_env_scripts() {
  local context_name=$1
  local setup_script
  local reset_script
  local state_key_list

  setup_script=$(e2e_manual_env_setup_script_path)
  reset_script=$(e2e_manual_env_reset_script_path)

  mkdir -p -- "$(dirname -- "${setup_script}")" || return 1

  state_key_list=$(e2e_manual_collect_state_env_keys | sort -u | tr '\n' ' ' | sed 's/[[:space:]]\+$//')

  e2e_manual_write_env_setup_script "${context_name}" "${setup_script}" "${reset_script}" "${state_key_list}" || return 1
  e2e_manual_write_env_reset_script "${reset_script}" || return 1
  chmod +x "${setup_script}" "${reset_script}" || return 1
}

e2e_manual_handoff_print() {
  local context_name=$1
  local setup_script
  local reset_script

  setup_script=$(e2e_manual_env_setup_script_path)
  reset_script=$(e2e_manual_env_reset_script_path)

  cat <<EOFH
Manual profile is ready.

Run ID:
  ${E2E_RUN_ID:-n/a}

Context name:
  ${context_name}

Context file:
  ${E2E_CONTEXT_FILE}

Runtime binary alias:
  declarest-e2e -> ${E2E_BIN}

Shell scripts:
  setup: ${setup_script}
  reset: ${reset_script}

To use it in your current shell:
  source ${setup_script@Q}
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" config show
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repo status -o json
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" resource list / --repository -o json

To reset environment variables and alias:
  source ${reset_script@Q}

This execution finished and runtime resources were kept.
To stop and remove this execution:
  ./run-e2e.sh --clean ${E2E_RUN_ID:-<run-id>}
To stop and remove all executions:
  ./run-e2e.sh --clean-all
EOFH
}

e2e_profile_manual_handoff() {
  local context_name=$1
  e2e_manual_write_env_scripts "${context_name}" || return 1
  e2e_manual_handoff_print "${context_name}"
}
