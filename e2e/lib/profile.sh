#!/usr/bin/env bash

e2e_apply_profile_defaults() {
  if [[ "${E2E_PROFILE}" != 'manual' ]]; then
    return 0
  fi

  if e2e_is_explicit 'resource-server' || \
    e2e_is_explicit 'resource-server-connection' || \
    e2e_is_explicit 'repo-type' || \
    e2e_is_explicit 'git-provider' || \
    e2e_is_explicit 'git-provider-connection' || \
    e2e_is_explicit 'secret-provider' || \
    e2e_is_explicit 'secret-provider-connection'; then
    return 0
  fi

  E2E_SELECTED_BY_PROFILE_DEFAULT=1

  # Manual profile default: maximal local-instantiable stack.
  E2E_RESOURCE_SERVER='keycloak'
  E2E_RESOURCE_SERVER_CONNECTION='local'
  E2E_REPO_TYPE='git'
  E2E_GIT_PROVIDER='gitlab'
  E2E_GIT_PROVIDER_CONNECTION='local'
  E2E_SECRET_PROVIDER='vault'
  E2E_SECRET_PROVIDER_CONNECTION='local'
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

e2e_profile_manual_handoff() {
  local context_name=$1

  cat <<EOFH
Manual profile is ready.

Context file:
  ${E2E_CONTEXT_FILE}

To use it in another shell:
  export DECLAREST_CONTEXTS_FILE=${E2E_CONTEXT_FILE@Q}
  ${E2E_BIN} --context ${context_name} config show
  ${E2E_BIN} --context ${context_name} repo status -o json
  ${E2E_BIN} --context ${context_name} resource list / --source local -o json

You can edit the context file above while components are running.
EOFH

  if [[ -t 0 ]]; then
    printf '\nPress ENTER to finish manual session (or Ctrl+C).\n'
    read -r _
  else
    e2e_warn 'manual profile running in non-interactive mode; waiting for interruption'
    while true; do
      sleep 3600
    done
  fi
}
