#!/usr/bin/env bash

CASE_ID='simple-api-server-resource-tree-sync'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=simple-api-server'

case_run() {
  case_repo_template_sync_tree 'simple-api-server' 'simple-api-rev' 'simple-api-sync'
}
