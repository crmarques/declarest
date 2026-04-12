#!/usr/bin/env bash
set -euo pipefail

repo_base_dir=${E2E_REPO_BASE_DIR:-}
[[ -n "${repo_base_dir}" ]] || exit 0
[[ "${E2E_PROFILE:-}" == operator-* ]] || exit 0

realm_root="${repo_base_dir}/admin/realms/acme"
[[ -d "${realm_root}" ]] || exit 0

pruned=0
for child in authentication clients organizations user-registry; do
  if [[ -e "${realm_root}/${child}" ]]; then
    rm -rf "${realm_root:?}/${child}"
    ((pruned += 1))
  fi
done

if ((pruned > 0)); then
  printf 'repo-template prepare pruned non-idempotent paths count=%d root=%s\n' "${pruned}" "${realm_root}" >&2
fi
