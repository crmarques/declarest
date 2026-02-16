#!/usr/bin/env bash

CASE_ID='cli-path-conflict'
CASE_SCOPE='corner'
CASE_REQUIRES=''

case_run() {
  case_run_declarest resource get /customers/acme --path /customers/other
  case_expect_failure
  case_expect_output_contains 'path mismatch between positional argument and --path'
}
