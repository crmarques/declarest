# Purpose and Glossary

## Purpose
DeclaREST is a Go tool that keeps a Git repo (desired state) in sync with remote REST APIs (actual state).

Source of truth:
- Git repo is the source of truth for apply/update/delete.
- Remote server is the source of truth for refresh/get/list (unless stated otherwise).

## Core value
- Declare resources as files in Git.
- Fetch resources from servers into Git deterministically.
- Diff desired vs actual.
- Reconcile safely and repeatably.

## Glossary
- Logical Path: normalized path-like identifier (example: `/fruits/apples/apple-01`).
- Resource: single remote object stored at `<logical-path>/resource.json`.
- Collection: directory representing a group of resources.
- Generic Metadata: applies to a collection subtree at `<collection>/_/metadata.json`.
- Resource Metadata: resource-specific metadata at `<logical-path>/metadata.json`.
- Desired State: repo content.
- Actual State: server content.
- Managers/Providers: stable interfaces (see specs/05-architecture.md).
