#!/usr/bin/env bash

CASE_ID='repo-status-baseline'
CASE_SCOPE='main'
CASE_REQUIRES=''

case_run() {
  case_run_declarest repo init
  case_expect_success

  case_run_declarest repo status -o json
  case_expect_success
  case_expect_output_contains '"state"'
}
