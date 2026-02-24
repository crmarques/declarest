#!/usr/bin/env bash
set -euo pipefail

TESTS_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

pass_count=0
fail_count=0

while IFS= read -r test_file; do
  [[ -n "${test_file}" ]] || continue
  test_name=$(basename "${test_file}")

  set +e
  output=$(bash "${test_file}" 2>&1)
  status=$?
  set -e

  if ((status == 0)); then
    printf '[PASS] %s\n' "${test_name}"
    ((pass_count += 1))
    continue
  fi

  printf '[FAIL] %s\n' "${test_name}"
  printf '%s\n' "${output}" | sed 's/^/  | /'
  ((fail_count += 1))
done < <(find "${TESTS_DIR}" -maxdepth 1 -type f -name '*_test.sh' ! -name 'testkit.sh' | sort)

printf '\n'
printf 'Bash E2E Contract Tests\n'
printf '  passed=%d failed=%d\n' "${pass_count}" "${fail_count}"

if ((fail_count > 0)); then
  exit 1
fi
