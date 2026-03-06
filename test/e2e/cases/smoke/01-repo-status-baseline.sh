#!/usr/bin/env bash

CASE_ID='repo-status-baseline'
CASE_SCOPE='smoke'
CASE_PROFILES='cli operator'
CASE_REQUIRES=''

case_run() {
  case_run_declarest repository init
  case_expect_success

  case_run_declarest repository status -o json
  case_expect_success
  case_expect_output_contains '"state"'
}
