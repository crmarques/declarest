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

e2e_manual_handoff_print() {
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

This execution finished and runtime resources were kept.
To stop and remove this execution:
  ./run-e2e.sh --clean ${E2E_RUN_ID:-<run-id>}
To stop and remove all executions:
  ./run-e2e.sh --clean-all
EOFH
}

e2e_profile_manual_handoff() {
  local context_name=$1
  e2e_manual_handoff_print "${context_name}"
}
