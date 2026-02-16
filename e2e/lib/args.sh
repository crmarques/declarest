#!/usr/bin/env bash

declare -Ag E2E_EXPLICIT

E2E_RESOURCE_SERVER='keycloak'
E2E_RESOURCE_SERVER_CONNECTION='local'
E2E_REPO_TYPE='filesystem'
E2E_GIT_PROVIDER=''
E2E_GIT_PROVIDER_CONNECTION='local'
E2E_SECRET_PROVIDER='file'
E2E_SECRET_PROVIDER_CONNECTION='local'
E2E_PROFILE='basic'
E2E_LIST_COMPONENTS=0
E2E_KEEP_RUNTIME=0
E2E_VERBOSE=0
E2E_CLEAN_RUN_ID=''
E2E_CLEAN_ALL=0

# shellcheck disable=SC2034
E2E_SELECTED_BY_PROFILE_DEFAULT=0

e2e_mark_explicit() {
  local key=$1
  E2E_EXPLICIT["${key}"]=1
}

e2e_is_explicit() {
  local key=$1
  [[ "${E2E_EXPLICIT[${key}]:-0}" == '1' ]]
}

e2e_has_help_flag() {
  local arg
  for arg in "$@"; do
    case "${arg}" in
      -h|--help)
        return 0
        ;;
    esac
  done
  return 1
}

e2e_profile_from_cli_args() {
  local profile='basic'

  while (($# > 0)); do
    case "$1" in
      --profile)
        [[ $# -ge 2 ]] || break
        profile=$2
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done

  printf '%s\n' "${profile}"
}

e2e_valid_component_name() {
  local value=$1
  [[ "${value}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]
}

e2e_validate_component_arg() {
  local flag=$1
  local value=$2
  local allow_none=${3:-false}
  local allow_empty=${4:-false}

  if [[ -z "${value}" ]]; then
    if [[ "${allow_empty}" == 'true' ]]; then
      return 0
    fi
    e2e_die "invalid ${flag} value: empty"
    return 1
  fi

  if [[ "${allow_none}" == 'true' && "${value}" == 'none' ]]; then
    return 0
  fi

  if ! e2e_valid_component_name "${value}"; then
    if [[ "${allow_none}" == 'true' ]]; then
      e2e_die "invalid ${flag} value: ${value} (expected component name or none)"
    else
      e2e_die "invalid ${flag} value: ${value} (expected component name)"
    fi
    return 1
  fi

  return 0
}

e2e_usage() {
  cat <<'USAGE'
Usage: ./run-e2e.sh [flags]

Objective:
  Run Declarest end-to-end workloads against a selected component stack.

Profiles:
  --profile <basic|full|manual>                  default: basic
    basic   Run compatible main cases only.
    full    Run compatible main and corner cases.
    manual  Start local components, print context access info, and exit.

Component selection:
  --resource-server <name|none>                  default: keycloak
  --resource-server-connection <local|remote>
  --repo-type <name>                             default: filesystem
  --git-provider <name>
  --git-provider-connection <local|remote>
  --secret-provider <name|none>                  default: file
  --secret-provider-connection <local|remote>

Runtime controls:
  --list-components   List discovered components and exit.
  --keep-runtime      Skip teardown to keep runtime resources available.
  --verbose           Print extra per-step log details.
  --clean <run-id>    Stop referenced run process and remove its containers/files.
  --clean-all         Stop all run processes and remove all executions under e2e/.runs.
  -h, --help          Show this help and exit.

Environment:
  DECLAREST_E2E_CONTAINER_ENGINE=<podman|docker> default: podman
  DECLAREST_E2E_EXECUTION_LOG=<path>             optional execution log path

Examples:
  ./run-e2e.sh --profile basic --repo-type filesystem --resource-server none --secret-provider none
  ./run-e2e.sh --profile full --repo-type git --git-provider gitlab --resource-server keycloak
  ./run-e2e.sh --profile manual --keep-runtime
  ./run-e2e.sh --clean 20260216-141148-216353
  ./run-e2e.sh --clean-all
USAGE
}

e2e_parse_cleanup_args() {
  local has_cleanup_flag=0
  local has_workload_flag=0

  E2E_CLEAN_RUN_ID=''
  E2E_CLEAN_ALL=0

  while (($# > 0)); do
    case "$1" in
      --clean)
        has_cleanup_flag=1
        [[ $# -ge 2 ]] || {
          e2e_die '--clean requires a run-id value'
          return 2
        }
        [[ "${2}" != -* ]] || {
          e2e_die '--clean requires a run-id value'
          return 2
        }
        [[ -z "${E2E_CLEAN_RUN_ID}" ]] || {
          e2e_die '--clean may only be provided once'
          return 2
        }
        E2E_CLEAN_RUN_ID=$2
        shift 2
        ;;
      --clean-all)
        has_cleanup_flag=1
        ((E2E_CLEAN_ALL == 0)) || {
          e2e_die '--clean-all may only be provided once'
          return 2
        }
        E2E_CLEAN_ALL=1
        shift
        ;;
      -h|--help)
        shift
        ;;
      --verbose)
        E2E_VERBOSE=1
        shift
        ;;
      --profile|--resource-server|--resource-server-connection|--repo-type|--git-provider|--git-provider-connection|--secret-provider|--secret-provider-connection)
        has_workload_flag=1
        shift
        [[ $# -gt 0 ]] && shift || true
        ;;
      --list-components|--keep-runtime)
        has_workload_flag=1
        shift
        ;;
      *)
        has_workload_flag=1
        shift
        ;;
    esac
  done

  if ((has_cleanup_flag == 0)); then
    return 1
  fi

  if ((has_workload_flag == 1)); then
    e2e_die '--clean/--clean-all cannot be combined with workload flags'
    return 2
  fi

  if [[ -n "${E2E_CLEAN_RUN_ID}" && ${E2E_CLEAN_ALL} -eq 1 ]]; then
    e2e_die '--clean and --clean-all are mutually exclusive'
    return 2
  fi

  if [[ -z "${E2E_CLEAN_RUN_ID}" && ${E2E_CLEAN_ALL} -eq 0 ]]; then
    e2e_die 'cleanup mode requires --clean <run-id> or --clean-all'
    return 2
  fi

  return 0
}

e2e_parse_args() {
  while (($# > 0)); do
    case "$1" in
      --profile)
        [[ $# -ge 2 ]] || {
          e2e_die '--profile requires a value'
          return 1
        }
        E2E_PROFILE=$2
        e2e_mark_explicit 'profile'
        shift 2
        ;;
      --resource-server)
        [[ $# -ge 2 ]] || {
          e2e_die '--resource-server requires a value'
          return 1
        }
        E2E_RESOURCE_SERVER=$2
        e2e_mark_explicit 'resource-server'
        shift 2
        ;;
      --resource-server-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--resource-server-connection requires a value'
          return 1
        }
        E2E_RESOURCE_SERVER_CONNECTION=$2
        e2e_mark_explicit 'resource-server-connection'
        shift 2
        ;;
      --repo-type)
        [[ $# -ge 2 ]] || {
          e2e_die '--repo-type requires a value'
          return 1
        }
        E2E_REPO_TYPE=$2
        e2e_mark_explicit 'repo-type'
        shift 2
        ;;
      --git-provider)
        [[ $# -ge 2 ]] || {
          e2e_die '--git-provider requires a value'
          return 1
        }
        E2E_GIT_PROVIDER=$2
        e2e_mark_explicit 'git-provider'
        shift 2
        ;;
      --git-provider-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--git-provider-connection requires a value'
          return 1
        }
        E2E_GIT_PROVIDER_CONNECTION=$2
        e2e_mark_explicit 'git-provider-connection'
        shift 2
        ;;
      --secret-provider)
        [[ $# -ge 2 ]] || {
          e2e_die '--secret-provider requires a value'
          return 1
        }
        E2E_SECRET_PROVIDER=$2
        e2e_mark_explicit 'secret-provider'
        shift 2
        ;;
      --secret-provider-connection)
        [[ $# -ge 2 ]] || {
          e2e_die '--secret-provider-connection requires a value'
          return 1
        }
        E2E_SECRET_PROVIDER_CONNECTION=$2
        e2e_mark_explicit 'secret-provider-connection'
        shift 2
        ;;
      --list-components)
        E2E_LIST_COMPONENTS=1
        shift
        ;;
      --keep-runtime)
        E2E_KEEP_RUNTIME=1
        shift
        ;;
      --verbose)
        E2E_VERBOSE=1
        shift
        ;;
      -h|--help)
        # main handles help before runtime setup; keep parser behavior non-exiting.
        e2e_usage
        return 0
        ;;
      *)
        e2e_die "unknown argument: $1"
        return 1
        ;;
    esac
  done

  case "${E2E_PROFILE}" in
    basic|full|manual) ;;
    *)
      e2e_die "invalid profile: ${E2E_PROFILE} (allowed: basic, full, manual)"
      return 1
      ;;
  esac

  e2e_validate_component_arg '--resource-server' "${E2E_RESOURCE_SERVER}" 'true' || return 1

  case "${E2E_RESOURCE_SERVER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid resource-server connection: ${E2E_RESOURCE_SERVER_CONNECTION}"
      return 1
      ;;
  esac

  e2e_validate_component_arg '--repo-type' "${E2E_REPO_TYPE}" || return 1
  e2e_validate_component_arg '--git-provider' "${E2E_GIT_PROVIDER}" 'false' 'true' || return 1

  case "${E2E_GIT_PROVIDER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid git-provider connection: ${E2E_GIT_PROVIDER_CONNECTION}"
      return 1
      ;;
  esac

  e2e_validate_component_arg '--secret-provider' "${E2E_SECRET_PROVIDER}" 'true' || return 1

  case "${E2E_SECRET_PROVIDER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid secret-provider connection: ${E2E_SECRET_PROVIDER_CONNECTION}"
      return 1
      ;;
  esac

  return 0
}
