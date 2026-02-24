#!/usr/bin/env bash

CASE_ID='keycloak-resource-tree-sync'
CASE_SCOPE='main'
CASE_REQUIRES='resource-server=keycloak'

case_run() {
  case_repo_template_sync_tree 'keycloak' 'keycloak-rev' 'keycloak-sync'
}
