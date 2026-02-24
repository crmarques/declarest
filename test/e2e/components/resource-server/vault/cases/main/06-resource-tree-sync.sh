#!/usr/bin/env bash

CASE_ID='vault-resource-tree-sync'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=vault'

case_run() {
  case_repo_template_sync_tree 'vault' 'vault-rev' 'vault-sync'
}
