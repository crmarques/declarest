#!/usr/bin/env bash

e2e_apply_profile_defaults() {
  if [[ "${E2E_PROFILE}" != 'operator' ]]; then
    # Non-operator profiles share the same component defaults from args parsing.
    return 0
  fi

  if ! e2e_is_explicit 'platform'; then
    E2E_PLATFORM='kubernetes'
  fi

  if ! e2e_is_explicit 'repo-type'; then
    E2E_REPO_TYPE='git'
    E2E_SELECTED_BY_PROFILE_DEFAULT=1
  fi

  if [[ "${E2E_REPO_TYPE}" == 'git' ]] && ! e2e_is_explicit 'git-provider'; then
    if [[ -z "${E2E_GIT_PROVIDER}" || "${E2E_GIT_PROVIDER}" == 'git' ]]; then
      E2E_GIT_PROVIDER='gitea'
      E2E_SELECTED_BY_PROFILE_DEFAULT=1
    fi
  fi

  return 0
}

e2e_validate_profile_rules() {
  if [[ "${E2E_PROFILE}" == 'manual' ]]; then
    if [[ "${E2E_MANAGED_SERVER_CONNECTION}" != 'local' && "${E2E_MANAGED_SERVER}" != 'none' ]]; then
      e2e_die 'manual profile is local-instantiable only; managed-server connection must be local'
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

    return 0
  fi

  if [[ "${E2E_PROFILE}" != 'operator' ]]; then
    return 0
  fi

  if [[ "${E2E_PLATFORM}" != 'kubernetes' ]]; then
    e2e_die 'operator profile requires --platform kubernetes'
    return 1
  fi

  if [[ "${E2E_REPO_TYPE}" != 'git' ]]; then
    e2e_die 'operator profile requires --repo-type git'
    return 1
  fi

  if [[ -z "${E2E_GIT_PROVIDER}" ]]; then
    e2e_die 'operator profile requires --git-provider'
    return 1
  fi

  if [[ "${E2E_GIT_PROVIDER}" == 'git' ]]; then
    e2e_die 'operator profile does not support --git-provider git; choose gitea or gitlab'
    return 1
  fi

  if [[ "${E2E_MANAGED_SERVER_CONNECTION}" != 'local' ]]; then
    e2e_die 'operator profile is local-instantiable only; managed-server connection must be local'
    return 1
  fi

  if [[ "${E2E_GIT_PROVIDER_CONNECTION}" != 'local' ]]; then
    e2e_die 'operator profile is local-instantiable only; git-provider connection must be local'
    return 1
  fi

  if [[ "${E2E_SECRET_PROVIDER}" == 'none' ]]; then
    e2e_die 'operator profile requires a secret provider (file or vault)'
    return 1
  fi

  if [[ "${E2E_SECRET_PROVIDER_CONNECTION}" != 'local' ]]; then
    e2e_die 'operator profile is local-instantiable only; secret-provider connection must be local'
    return 1
  fi

  return 0
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
    operator)
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
export DECLAREST_E2E_PLATFORM=${E2E_PLATFORM@Q}
export DECLAREST_E2E_KUBECONFIG=${E2E_KUBECONFIG@Q}
export DECLAREST_E2E_KIND_CLUSTER=${E2E_KIND_CLUSTER_NAME@Q}
export DECLAREST_E2E_K8S_NAMESPACE=${E2E_K8S_NAMESPACE@Q}
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

if [[ "\${DECLAREST_E2E_PLATFORM:-}" == 'kubernetes' && -n "\${DECLAREST_E2E_KUBECONFIG:-}" ]]; then
  if [[ -z "\${DECLAREST_E2E_ORIGINAL_KUBECONFIG+x}" ]]; then
    export DECLAREST_E2E_ORIGINAL_KUBECONFIG="\${KUBECONFIG-}"
  fi
  export KUBECONFIG="\${DECLAREST_E2E_KUBECONFIG}"
fi

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

if [[ -n "${DECLAREST_E2E_ORIGINAL_KUBECONFIG+x}" ]]; then
  if [[ -n "${DECLAREST_E2E_ORIGINAL_KUBECONFIG}" ]]; then
    export KUBECONFIG="${DECLAREST_E2E_ORIGINAL_KUBECONFIG}"
  else
    unset KUBECONFIG || true
  fi
fi
unset DECLAREST_E2E_ORIGINAL_KUBECONFIG

for state_var in ${DECLAREST_E2E_STATE_ENV_KEYS:-}; do
  unset "${state_var}"
done

unset DECLAREST_E2E_STATE_ENV_KEYS
unset DECLAREST_E2E_ENV_SETUP_SCRIPT
unset DECLAREST_E2E_ENV_RESET_SCRIPT
unset DECLAREST_E2E_K8S_NAMESPACE
unset DECLAREST_E2E_KIND_CLUSTER
unset DECLAREST_E2E_KUBECONFIG
unset DECLAREST_E2E_PLATFORM
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

e2e_profile_repo_provider_state_file() {
  [[ -n "${E2E_STATE_DIR:-}" ]] || return 1
  [[ -n "${E2E_GIT_PROVIDER:-}" ]] || return 1

  printf '%s/git-provider-%s.env\n' "${E2E_STATE_DIR}" "${E2E_GIT_PROVIDER}"
}

e2e_profile_repo_provider_state_get() {
  local key=$1
  local state_file

  state_file=$(e2e_profile_repo_provider_state_file) || return 1
  e2e_state_get "${state_file}" "${key}"
}

e2e_profile_repo_provider_web_url_from_remote() {
  local remote_url=$1
  local host
  local path

  case "${remote_url}" in
    http://*|https://*)
      if [[ "${remote_url}" =~ ^([a-zA-Z][a-zA-Z0-9+.-]*://)([^/@]+@)(.+)$ ]]; then
        remote_url="${BASH_REMATCH[1]}${BASH_REMATCH[3]}"
      fi
      printf '%s\n' "${remote_url%.git}"
      return 0
      ;;
    git@*:* )
      host=${remote_url#git@}
      host=${host%%:*}
      path=${remote_url#*:}
      printf 'https://%s/%s\n' "${host}" "${path%.git}"
      return 0
      ;;
    ssh://git@* )
      remote_url=${remote_url#ssh://}
      remote_url=${remote_url#*@}
      host=${remote_url%%/*}
      path=${remote_url#*/}
      printf 'https://%s/%s\n' "${host}" "${path%.git}"
      return 0
      ;;
  esac

  return 1
}

e2e_profile_print_kubernetes_connection_help() {
  [[ "${E2E_PLATFORM:-}" == 'kubernetes' ]] || return 0
  [[ -n "${E2E_KIND_CLUSTER_NAME:-}" ]] || return 0

  cat <<EOFK8S
How to connect kubectl to this kind cluster:
  export KUBECONFIG="${E2E_KUBECONFIG:-}"
  kubectl config current-context
  kubectl cluster-info
  kubectl -n "${E2E_K8S_NAMESPACE:-default}" get pods,svc

Kubernetes access:
  cluster: ${E2E_KIND_CLUSTER_NAME}
  namespace: ${E2E_K8S_NAMESPACE:-default}
  kubeconfig: ${E2E_KUBECONFIG:-n/a}
EOFK8S
}

e2e_profile_print_repo_provider_access_help() {
  local provider=${E2E_GIT_PROVIDER:-}
  local connection=${E2E_GIT_PROVIDER_CONNECTION:-local}
  local remote_url=''
  local web_url=''
  local login_url=''
  local username=''
  local password=''

  [[ "${E2E_REPO_TYPE:-}" == 'git' ]] || return 0
  [[ -n "${provider}" ]] || return 0

  remote_url=$(e2e_profile_repo_provider_state_get 'GIT_REMOTE_URL' || true)

  case "${provider}" in
    gitea)
      web_url=$(e2e_profile_repo_provider_state_get 'GITEA_BASE_URL' || true)
      if [[ -z "${web_url}" && -n "${remote_url}" ]]; then
        web_url=$(e2e_profile_repo_provider_web_url_from_remote "${remote_url}" || true)
      fi
      if [[ -n "${web_url}" ]]; then
        login_url="${web_url%/}/user/login"
      fi
      username=$(e2e_profile_repo_provider_state_get 'GITEA_ADMIN_USERNAME' || true)
      password=$(e2e_profile_repo_provider_state_get 'GITEA_ADMIN_PASSWORD' || true)
      ;;
    gitlab)
      web_url=$(e2e_profile_repo_provider_state_get 'GITLAB_BASE_URL' || true)
      if [[ -z "${web_url}" && -n "${remote_url}" ]]; then
        web_url=$(e2e_profile_repo_provider_web_url_from_remote "${remote_url}" || true)
      fi
      if [[ -n "${web_url}" ]]; then
        login_url="${web_url%/}/users/sign_in"
      fi
      username=$(e2e_profile_repo_provider_state_get 'GIT_AUTH_USERNAME' || true)
      password=$(e2e_profile_repo_provider_state_get 'GITLAB_ROOT_PASSWORD' || true)
      ;;
    github)
      if [[ -n "${remote_url}" ]]; then
        web_url=$(e2e_profile_repo_provider_web_url_from_remote "${remote_url}" || true)
      fi
      if [[ -n "${web_url}" ]]; then
        login_url='https://github.com/login'
      fi
      username=$(e2e_profile_repo_provider_state_get 'GIT_AUTH_USERNAME' || true)
      ;;
    git)
      ;;
    *)
      if [[ -n "${remote_url}" ]]; then
        web_url=$(e2e_profile_repo_provider_web_url_from_remote "${remote_url}" || true)
      fi
      login_url="${web_url}"
      username=$(e2e_profile_repo_provider_state_get 'GIT_AUTH_USERNAME' || true)
      ;;
  esac

  cat <<EOFREPO
Repository provider access:
  provider: ${provider} (${connection})
  git remote: ${remote_url:-n/a}
EOFREPO

  if [[ -n "${login_url}" ]]; then
    cat <<EOFREPOLOGIN
  web login: ${login_url}
  open in browser:
    xdg-open "${login_url}"
    open "${login_url}"  # macOS
EOFREPOLOGIN
  fi

  if [[ -n "${username}" ]]; then
    printf '  username: %s\n' "${username}"
  fi
  if [[ -n "${password}" ]]; then
    printf '  password: %s\n' "${password}"
  fi

  case "${provider}" in
    github)
      printf '  auth note: use your configured GitHub token for git operations if prompted.\n'
      ;;
    git)
      printf '  auth note: built-in git provider uses local file:// repository URLs (no web login).\n'
      ;;
  esac
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

Platform:
  ${E2E_PLATFORM:-n/a}

Shell scripts:
  setup: ${setup_script}
  reset: ${reset_script}

To use it in your current shell:
  source ${setup_script@Q}
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" config show
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" repository status -o json
  declarest-e2e --context "\${DECLAREST_E2E_CONTEXT}" resource list / --repository -o json
EOFH

  if [[ "${E2E_PLATFORM}" == 'kubernetes' && -n "${E2E_KIND_CLUSTER_NAME:-}" ]]; then
    printf '\n'
    e2e_profile_print_kubernetes_connection_help
  fi

  if [[ "${E2E_REPO_TYPE:-}" == 'git' && -n "${E2E_GIT_PROVIDER:-}" ]]; then
    printf '\n'
    e2e_profile_print_repo_provider_access_help
  fi

  cat <<EOFH

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
