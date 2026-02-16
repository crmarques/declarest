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

e2e_usage() {
  cat <<'USAGE'
Usage: ./run-e2e.sh [flags]

Profiles:
  --profile <basic|full|manual>            default: basic

Component selection:
  --resource-server <keycloak|none>        default: keycloak
  --resource-server-connection <local|remote>
  --repo-type <filesystem|git>             default: filesystem
  --git-provider <git|gitlab|github>
  --git-provider-connection <local|remote>
  --secret-provider <file|vault|none>      default: file
  --secret-provider-connection <local|remote>

General:
  --list-components
  --keep-runtime
  --verbose
  -h, --help
USAGE
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
        e2e_usage
        exit 0
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

  case "${E2E_RESOURCE_SERVER}" in
    keycloak|none) ;;
    *)
      e2e_die "invalid resource server: ${E2E_RESOURCE_SERVER}"
      return 1
      ;;
  esac

  case "${E2E_RESOURCE_SERVER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid resource-server connection: ${E2E_RESOURCE_SERVER_CONNECTION}"
      return 1
      ;;
  esac

  case "${E2E_REPO_TYPE}" in
    filesystem|git) ;;
    *)
      e2e_die "invalid repo type: ${E2E_REPO_TYPE}"
      return 1
      ;;
  esac

  case "${E2E_GIT_PROVIDER}" in
    ''|git|gitlab|github) ;;
    *)
      e2e_die "invalid git provider: ${E2E_GIT_PROVIDER}"
      return 1
      ;;
  esac

  case "${E2E_GIT_PROVIDER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid git-provider connection: ${E2E_GIT_PROVIDER_CONNECTION}"
      return 1
      ;;
  esac

  case "${E2E_SECRET_PROVIDER}" in
    file|vault|none) ;;
    *)
      e2e_die "invalid secret provider: ${E2E_SECRET_PROVIDER}"
      return 1
      ;;
  esac

  case "${E2E_SECRET_PROVIDER_CONNECTION}" in
    local|remote) ;;
    *)
      e2e_die "invalid secret-provider connection: ${E2E_SECRET_PROVIDER_CONNECTION}"
      return 1
      ;;
  esac

  if [[ "${E2E_REPO_TYPE}" == 'git' && -z "${E2E_GIT_PROVIDER}" ]]; then
    e2e_die '--repo-type git requires --git-provider'
    return 1
  fi

  if [[ "${E2E_REPO_TYPE}" == 'filesystem' && -n "${E2E_GIT_PROVIDER}" ]]; then
    e2e_die '--git-provider is only valid when --repo-type git'
    return 1
  fi
}
