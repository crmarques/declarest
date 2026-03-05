package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

type syncMode string

const (
	syncModeFull        syncMode = "full"
	syncModeIncremental syncMode = "incremental"
)

type syncApplyTarget struct {
	Path      string
	Recursive bool
}

type syncExecutionPlan struct {
	Mode         syncMode
	ApplyTargets []syncApplyTarget
	PruneTargets []string
}

func fullSyncExecutionPlan(path string, recursive bool) syncExecutionPlan {
	return syncExecutionPlan{
		Mode: syncModeFull,
		ApplyTargets: []syncApplyTarget{
			{Path: path, Recursive: recursive},
		},
	}
}

func buildSyncExecutionPlan(
	ctx context.Context,
	syncPolicy *declarestv1alpha1.SyncPolicy,
	repositoryPath string,
	targetRevision string,
	secretHashChanged bool,
	fullResyncDue bool,
) (syncExecutionPlan, error) {
	recursive := true
	if syncPolicy.Spec.Source.Recursive != nil {
		recursive = *syncPolicy.Spec.Source.Recursive
	}

	fullPlan := fullSyncExecutionPlan(syncPolicy.Spec.Source.Path, recursive)

	// Safety-first fallback to full scope for changes that can impact all resources.
	if fullResyncDue || secretHashChanged {
		return fullPlan, nil
	}
	if syncPolicy.Status.ObservedGeneration != syncPolicy.Generation {
		return fullPlan, nil
	}
	baseRevision := strings.TrimSpace(syncPolicy.Status.LastAppliedRepoRevision)
	newRevision := strings.TrimSpace(targetRevision)
	if baseRevision == "" || newRevision == "" {
		return fullPlan, nil
	}
	if baseRevision == newRevision {
		return syncExecutionPlan{Mode: syncModeIncremental}, nil
	}
	if !recursive {
		return fullPlan, nil
	}

	incremental, err := buildIncrementalPlanFromRepositoryDiff(
		ctx,
		repositoryPath,
		baseRevision,
		newRevision,
		syncPolicy.Spec.Source.Path,
	)
	if err != nil {
		return syncExecutionPlan{}, err
	}
	if incremental.requiresFull {
		return fullPlan, nil
	}

	targets := normalizeSyncApplyTargets(incremental.applyTargets)
	pruneTargets := stringSet(incremental.pruneTargets)
	return syncExecutionPlan{
		Mode:         syncModeIncremental,
		ApplyTargets: targets,
		PruneTargets: pruneTargets,
	}, nil
}

type incrementalSyncPlan struct {
	applyTargets []syncApplyTarget
	pruneTargets []string
	requiresFull bool
}

func buildIncrementalPlanFromRepositoryDiff(
	ctx context.Context,
	repositoryPath string,
	baseRevision string,
	targetRevision string,
	sourcePath string,
) (incrementalSyncPlan, error) {
	changes, err := repositoryFileChangesBetweenRevisions(ctx, repositoryPath, baseRevision, targetRevision)
	if err != nil {
		return incrementalSyncPlan{}, err
	}

	normalizedSource := normalizeOverlapPath(sourcePath)
	plan := incrementalSyncPlan{}
	for _, change := range changes {
		action, actionErr := change.Action()
		if actionErr != nil {
			return incrementalSyncPlan{}, actionErr
		}
		switch action {
		case merkletrie.Insert:
			accumulateAddedPath(&plan, change.To.Name, normalizedSource)
		case merkletrie.Delete:
			accumulateRemovedPath(&plan, change.From.Name, normalizedSource)
		case merkletrie.Modify:
			accumulateAddedPath(&plan, change.To.Name, normalizedSource)
			if change.From.Name != change.To.Name {
				accumulateRemovedPath(&plan, change.From.Name, normalizedSource)
			}
		default:
			plan.requiresFull = true
		}
		if plan.requiresFull {
			return plan, nil
		}
	}

	return plan, nil
}

type repositoryPathKind int

const (
	repositoryPathKindNone repositoryPathKind = iota
	repositoryPathKindPayload
	repositoryPathKindResourceMetadata
	repositoryPathKindCollectionMetadata
	repositoryPathKindUnknownConfig
)

func classifyRepositoryPath(raw string) (string, repositoryPathKind) {
	value := strings.Trim(strings.TrimSpace(filepath.ToSlash(raw)), "/")
	if value == "" {
		return "", repositoryPathKindNone
	}

	ext := strings.ToLower(filepath.Ext(value))
	switch ext {
	case ".json", ".yaml", ".yml":
	default:
		return "", repositoryPathKindNone
	}

	base := strings.TrimSuffix(filepath.Base(value), ext)
	dir := strings.Trim(filepath.ToSlash(filepath.Dir(value)), "/")
	if dir == "." {
		dir = ""
	}

	// Reserved metadata namespace directories must not be interpreted as payload.
	if strings.Contains("/"+value, "/_/") && base != "metadata" {
		return "", repositoryPathKindUnknownConfig
	}

	switch base {
	case "resource":
		if dir == "" {
			return "", repositoryPathKindUnknownConfig
		}
		return "/" + dir, repositoryPathKindPayload
	case "metadata":
		if filepath.Base(dir) == "_" {
			collectionDir := strings.Trim(filepath.ToSlash(filepath.Dir(dir)), "/")
			if collectionDir == "." || collectionDir == "" {
				return "/", repositoryPathKindCollectionMetadata
			}
			return "/" + collectionDir, repositoryPathKindCollectionMetadata
		}
		if dir == "" {
			return "", repositoryPathKindUnknownConfig
		}
		return "/" + dir, repositoryPathKindResourceMetadata
	default:
		return "/" + strings.Trim(strings.TrimSuffix(value, ext), "/"), repositoryPathKindPayload
	}
}

func accumulateAddedPath(plan *incrementalSyncPlan, changedPath string, sourcePath string) {
	logicalPath, kind := classifyRepositoryPath(changedPath)
	switch kind {
	case repositoryPathKindNone:
		return
	case repositoryPathKindUnknownConfig:
		if isPathRelevantToScope(logicalPath, changedPath, sourcePath) {
			plan.requiresFull = true
		}
		return
	case repositoryPathKindPayload, repositoryPathKindResourceMetadata:
		if !hasPathOverlap(logicalPath, sourcePath) {
			return
		}
		plan.applyTargets = append(plan.applyTargets, syncApplyTarget{Path: logicalPath, Recursive: false})
	case repositoryPathKindCollectionMetadata:
		targetPath, ok := scopedCollectionMetadataTarget(logicalPath, sourcePath)
		if !ok {
			return
		}
		plan.applyTargets = append(plan.applyTargets, syncApplyTarget{Path: targetPath, Recursive: true})
	}
}

func accumulateRemovedPath(plan *incrementalSyncPlan, changedPath string, sourcePath string) {
	logicalPath, kind := classifyRepositoryPath(changedPath)
	switch kind {
	case repositoryPathKindNone:
		return
	case repositoryPathKindUnknownConfig:
		if isPathRelevantToScope(logicalPath, changedPath, sourcePath) {
			plan.requiresFull = true
		}
		return
	case repositoryPathKindPayload:
		if hasPathOverlap(logicalPath, sourcePath) {
			plan.pruneTargets = append(plan.pruneTargets, logicalPath)
		}
	case repositoryPathKindResourceMetadata, repositoryPathKindCollectionMetadata:
		accumulateAddedPath(plan, changedPath, sourcePath)
	}
}

func scopedCollectionMetadataTarget(collectionPath string, sourcePath string) (string, bool) {
	if !hasPathOverlap(collectionPath, sourcePath) {
		return "", false
	}
	normalizedCollection := normalizeOverlapPath(collectionPath)
	normalizedSource := normalizeOverlapPath(sourcePath)
	if pathHasPrefix(normalizedSource, normalizedCollection) {
		return normalizedSource, true
	}
	return normalizedCollection, true
}

func isPathRelevantToScope(logicalPath string, rawRepositoryPath string, sourcePath string) bool {
	if logicalPath != "" {
		return hasPathOverlap(logicalPath, sourcePath)
	}
	sourcePrefix := strings.Trim(strings.TrimPrefix(normalizeOverlapPath(sourcePath), "/"), "/")
	repoPath := strings.Trim(strings.TrimSpace(filepath.ToSlash(rawRepositoryPath)), "/")
	if sourcePrefix == "" {
		return repoPath != ""
	}
	if repoPath == sourcePrefix {
		return true
	}
	return strings.HasPrefix(repoPath, sourcePrefix+"/")
}

func normalizeSyncApplyTargets(targets []syncApplyTarget) []syncApplyTarget {
	if len(targets) == 0 {
		return nil
	}

	pathToRecursive := make(map[string]bool, len(targets))
	for _, target := range targets {
		path := normalizeOverlapPath(target.Path)
		existing, ok := pathToRecursive[path]
		if !ok {
			pathToRecursive[path] = target.Recursive
			continue
		}
		pathToRecursive[path] = existing || target.Recursive
	}

	normalized := make([]syncApplyTarget, 0, len(pathToRecursive))
	for path, recursive := range pathToRecursive {
		normalized = append(normalized, syncApplyTarget{Path: path, Recursive: recursive})
	}
	sort.Slice(normalized, func(i int, j int) bool {
		if len(normalized[i].Path) == len(normalized[j].Path) {
			if normalized[i].Recursive == normalized[j].Recursive {
				return normalized[i].Path < normalized[j].Path
			}
			return normalized[i].Recursive
		}
		return len(normalized[i].Path) < len(normalized[j].Path)
	})

	filtered := make([]syncApplyTarget, 0, len(normalized))
	for _, candidate := range normalized {
		covered := false
		for _, kept := range filtered {
			if !kept.Recursive {
				continue
			}
			if pathHasPrefix(candidate.Path, kept.Path) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		filtered = append(filtered, candidate)
	}

	return filtered
}

func pathHasPrefix(path string, prefix string) bool {
	normalizedPath := normalizeOverlapPath(path)
	normalizedPrefix := normalizeOverlapPath(prefix)
	if normalizedPrefix == "/" {
		return true
	}
	if normalizedPath == normalizedPrefix {
		return true
	}
	return strings.HasPrefix(normalizedPath, normalizedPrefix+"/")
}

func repositoryFileChangesBetweenRevisions(
	ctx context.Context,
	repositoryPath string,
	baseRevision string,
	targetRevision string,
) (object.Changes, error) {
	repo, err := gogit.PlainOpen(strings.TrimSpace(repositoryPath))
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	baseCommit, err := repo.CommitObject(plumbing.NewHash(strings.TrimSpace(baseRevision)))
	if err != nil {
		return nil, fmt.Errorf("load base revision %q: %w", baseRevision, err)
	}
	targetCommit, err := repo.CommitObject(plumbing.NewHash(strings.TrimSpace(targetRevision)))
	if err != nil {
		return nil, fmt.Errorf("load target revision %q: %w", targetRevision, err)
	}

	baseTree, err := baseCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("load base tree: %w", err)
	}
	targetTree, err := targetCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("load target tree: %w", err)
	}

	changes, err := baseTree.DiffContext(ctx, targetTree)
	if err != nil {
		return nil, fmt.Errorf("diff revisions: %w", err)
	}
	return changes, nil
}
