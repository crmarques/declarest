#!/usr/bin/env bash

CASE_ID='rundeck-resource-tree-sync'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=rundeck'

case_run() {
  case_repo_template_sync_tree 'rundeck' 'rundeck-rev' 'rundeck-sync'
}
