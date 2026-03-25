// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/faults"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/internal/bootstrap"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// secretRefCache maps SyncPolicy NamespacedName to the set of Kubernetes Secret
// names referenced by the policy's dependency CRDs. Updated during reconciliation,
// read by the Secret watch mapper to avoid O(n*3) API calls per Secret event.
type secretRefCache struct {
	mu    sync.RWMutex
	index map[types.NamespacedName]sets.Set[string]
}

func (c *secretRefCache) update(key types.NamespacedName, secretNames []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.index == nil {
		c.index = make(map[types.NamespacedName]sets.Set[string])
	}
	if len(secretNames) == 0 {
		delete(c.index, key)
		return
	}
	c.index[key] = sets.New(secretNames...)
}

func (c *secretRefCache) remove(key types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.index, key)
}

func (c *secretRefCache) policiesForSecret(namespace string, secretName string) []reconcile.Request {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var requests []reconcile.Request
	for key, names := range c.index {
		if key.Namespace == namespace && names.Has(secretName) {
			requests = append(requests, reconcile.Request{NamespacedName: key})
		}
	}
	return requests
}

// SyncPolicyReconciler reconciles SyncPolicy resources.
type SyncPolicyReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
	secretRefs              secretRefCache
}

const (
	syncPolicyIndexResourceRepositoryRef = "spec.resourceRepositoryRef.name"
	syncPolicyIndexManagedServerRef      = "spec.managedServerRef.name"
	syncPolicyIndexSecretStoreRef        = "spec.secretStoreRef.name"
)

// syncPolicyReconciliation holds the state and dependencies for a single reconciliation of a SyncPolicy.
type syncPolicyReconciliation struct {
	*SyncPolicyReconciler
	ctx         context.Context
	req         ctrl.Request
	logger      logr.Logger
	policy      *declarestv1alpha1.SyncPolicy
	repo        *declarestv1alpha1.ResourceRepository
	server      *declarestv1alpha1.ManagedServer
	secret      *declarestv1alpha1.SecretStore
	resultLabel string
	reasonLabel string
}

func (r *SyncPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	recon := &syncPolicyReconciliation{
		SyncPolicyReconciler: r,
		ctx:                  ctx,
		req:                  req,
		logger:               log.FromContext(ctx).WithValues("syncPolicy", req.NamespacedName.String(), "reconcile_id", uuidString()),
		policy:               &declarestv1alpha1.SyncPolicy{},
		resultLabel:          "success",
		reasonLabel:          conditionReasonReady,
	}
	return recon.reconcile()
}

func (r *syncPolicyReconciliation) reconcile() (result ctrl.Result, reconcileErr error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		syncPolicyReconcileTotal.WithLabelValues(r.req.Namespace, r.req.Name, r.resultLabel, r.reasonLabel).Inc()
		syncPolicyReconcileDurationSeconds.WithLabelValues(r.req.Namespace, r.req.Name, r.resultLabel).Observe(duration)
	}()

	if err := r.Get(r.ctx, r.req.NamespacedName, r.policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle finalizer logic
	if res, err := r.handleFinalizer(); err != nil || res != nil {
		return *res, err
	}

	r.policy = expandRuntimeSyncPolicy(r.policy)
	r.policy.Default()

	// Validate spec and check if suspended
	if res, err := r.validatePrerequisites(); err != nil || res != nil {
		return *res, err
	}

	// Load all dependencies
	if res, err := r.loadDependencies(); err != nil || res != nil {
		return *res, err
	}

	// Check if the policy is already in the desired state
	if res, err := r.checkSyncStatus(); err != nil || res != nil {
		return *res, err
	}

	// Perform the synchronization
	return r.performSync()
}

func (r *syncPolicyReconciliation) handleFinalizer() (*ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(r.policy, finalizerName) {
		controllerutil.AddFinalizer(r.policy, finalizerName)
		if err := r.Update(r.ctx, r.policy); err != nil {
			return &ctrl.Result{}, err
		}
	}

	if !r.policy.DeletionTimestamp.IsZero() {
		r.secretRefs.remove(r.req.NamespacedName)
		cacheDir := resolveCacheRootPath(r.policy.Namespace, r.policy.Name)
		if err := os.RemoveAll(cacheDir); err != nil {
			r.logger.Error(err, "failed to clean up cache directory", "path", cacheDir)
		}
		controllerutil.RemoveFinalizer(r.policy, finalizerName)
		if err := r.Update(r.ctx, r.policy); err != nil {
			return &ctrl.Result{}, err
		}
		return &ctrl.Result{}, nil
	}
	return nil, nil
}

func (r *syncPolicyReconciliation) validatePrerequisites() (*ctrl.Result, error) {
	if validationErr := r.policy.ValidateSpec(); validationErr != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonSpecInvalid
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonSpecInvalid, validationErr.Error(), 0, "SpecInvalid")
		return &res, err
	}

	if r.policy.Spec.Suspend {
		nowTime := now()
		r.policy.Status.ObservedGeneration = r.policy.Generation
		r.policy.Status.LastAttemptTime = &nowTime
		r.policy.Status.Conditions = setStatusCondition(r.policy.Status.Conditions, declarestv1alpha1.ConditionTypeReady, metav1.ConditionFalse, conditionReasonSuspended, "sync policy is suspended")
		r.policy.Status.Conditions = setStatusCondition(r.policy.Status.Conditions, declarestv1alpha1.ConditionTypeStalled, metav1.ConditionFalse, conditionReasonSuspended, "")
		r.policy.Status.Conditions = setStatusCondition(r.policy.Status.Conditions, declarestv1alpha1.ConditionTypeReconciling, metav1.ConditionFalse, conditionReasonSuspended, "")
		if err := r.Status().Update(r.ctx, r.policy); err != nil {
			r.resultLabel = "error"
			r.reasonLabel = conditionReasonSuspended
			return &ctrl.Result{}, err
		}
		emitEventf(r.Recorder, r.policy, corev1.EventTypeNormal, "Suspended", "sync policy is suspended")
		r.reasonLabel = conditionReasonSuspended
		return &ctrl.Result{}, nil
	}

	if overlapErr := r.validateNoOverlap(r.ctx, r.policy); overlapErr != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonOverlappingPolicy
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonOverlappingPolicy, overlapErr.Error(), 0, "OverlappingPolicy")
		return &res, err
	}

	return nil, nil
}

func (r *syncPolicyReconciliation) loadDependencies() (*ctrl.Result, error) {
	var err error
	r.repo, err = r.loadResourceRepository()
	if err != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonDependencyInvalid
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonDependencyInvalid, fmt.Sprintf("resolve resource repository: %v", err), 0, "DependencyInvalid")
		return &res, err
	}
	if !isDependencyReady(r.repo.Status.Conditions) {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonDependencyNotReady
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonDependencyNotReady,
			fmt.Sprintf("ResourceRepository %q is not ready", r.repo.Name), defaultTransientRequeueInterval, "DependencyNotReady")
		return &res, err
	}

	r.server, err = r.loadManagedServer()
	if err != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonDependencyInvalid
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonDependencyInvalid, fmt.Sprintf("resolve managed server: %v", err), 0, "DependencyInvalid")
		return &res, err
	}
	if !isDependencyReady(r.server.Status.Conditions) {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonDependencyNotReady
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonDependencyNotReady,
			fmt.Sprintf("ManagedServer %q is not ready", r.server.Name), defaultTransientRequeueInterval, "DependencyNotReady")
		return &res, err
	}

	r.secret, err = r.loadSecretStore()
	if err != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonDependencyInvalid
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonDependencyInvalid, fmt.Sprintf("resolve secret store: %v", err), 0, "DependencyInvalid")
		return &res, err
	}
	if !isDependencyReady(r.secret.Status.Conditions) {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonDependencyNotReady
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonDependencyNotReady,
			fmt.Sprintf("SecretStore %q is not ready", r.secret.Name), defaultTransientRequeueInterval, "DependencyNotReady")
		return &res, err
	}

	r.secretRefs.update(r.req.NamespacedName, collectSecretNames(r.repo, r.server, r.secret))
	return nil, nil
}

func (r *syncPolicyReconciliation) checkSyncStatus() (*ctrl.Result, error) {
	repoRevision := strings.TrimSpace(r.repo.Status.LastFetchedRevision)
	if repoRevision == "" {
		r.reasonLabel = conditionReasonDependencyInvalid
		r.resultLabel = "error"
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonDependencyInvalid, "resource repository has no fetched revision yet", r.repo.Spec.PollInterval.Duration, "DependencyInvalid")
		return &res, err
	}

	secretHash := computeSecretVersionHash(r.ctx, r.Client, r.policy.Namespace, r.repo, r.server, r.secret)
	secretHashChanged := r.policy.Status.LastSecretResourceVersionHash != secretHash
	currentTime := time.Now().UTC()
	fullResyncDue, fullResyncErr := shouldRunFullResync(r.policy.Spec.FullResyncCron, r.policy.Status.LastFullResyncTime, currentTime)
	if fullResyncErr != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonSpecInvalid
		res, err := r.failWithStatus(r.ctx, r.policy, conditionReasonSpecInvalid, fmt.Sprintf("invalid full resync cron: %v", fullResyncErr), 0, "SpecInvalid")
		return &res, err
	}

	if r.policy.Status.ObservedGeneration == r.policy.Generation &&
		r.policy.Status.LastAppliedRepoRevision == repoRevision &&
		!secretHashChanged &&
		!fullResyncDue {
		r.logger.Info("sync policy is already at desired state", "revision", repoRevision)
		return &ctrl.Result{RequeueAfter: syncPolicyRequeueAfter(r.policy, currentTime)}, nil
	}
	return nil, nil
}

func (r *syncPolicyReconciliation) performSync() (ctrl.Result, error) {
	runtimeBuild, runtimeErr := buildRuntimeContext(r.ctx, r.Client, r.policy, r.repo, r.server, r.secret)
	if runtimeErr != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonRepositoryUnavailable
		return r.failWithStatus(r.ctx, r.policy, conditionReasonRepositoryUnavailable, runtimeErr.Error(), 0, "RuntimeContextFailed")
	}
	if runtimeBuild.Cleanup != nil {
		defer runtimeBuild.Cleanup()
	}

	session, sessionErr := bootstrap.NewSessionFromResolvedContext(runtimeBuild.ResolvedContext)
	if sessionErr != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonSessionBootstrapFailed
		return r.failWithStatus(r.ctx, r.policy, conditionReasonSessionBootstrapFailed, sessionErr.Error(), 0, "SessionBootstrapFailed")
	}

	nowTime := now()
	r.policy.Status.ObservedGeneration = r.policy.Generation
	r.policy.Status.LastAttemptTime = &nowTime
	r.policy.Status.LastAttemptedRepoRevision = r.repo.Status.LastFetchedRevision
	r.policy.Status.Conditions = setStatusCondition(r.policy.Status.Conditions, declarestv1alpha1.ConditionTypeReconciling, metav1.ConditionTrue, conditionReasonReconciling, "applying repository state")
	if err := r.Status().Update(r.ctx, r.policy); err != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonReconcileFailed
		return ctrl.Result{}, err
	}

	executionPlan, err := r.buildExecutionPlan(runtimeBuild.RepositoryLocalPath, r.repo.Status.LastFetchedRevision)
	if err != nil {
		r.logger.Info("falling back to full sync plan", "reason", err.Error())
		recursive := r.policy.Spec.Source.Recursive == nil || *r.policy.Spec.Source.Recursive
		plan := fullSyncExecutionPlan(r.policy.Spec.Source.Path, recursive)
		executionPlan = &plan
	}

	targetedCount, appliedCount, err := r.applyChanges(session, executionPlan)
	if err != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonReconcileFailed
		return r.failWithStatus(r.ctx, r.policy, conditionReasonReconcileFailed, err.Error(), 0, "SyncFailed")
	}

	prunedCount, err := r.pruneChanges(session, executionPlan)
	if err != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonReconcileFailed
		return r.failWithStatus(r.ctx, r.policy, conditionReasonReconcileFailed, err.Error(), 0, "SyncFailed")
	}

	return r.updateSuccessStatus(executionPlan, targetedCount, appliedCount, prunedCount)
}

func (r *syncPolicyReconciliation) updateSuccessStatus(plan *syncExecutionPlan, targeted, applied, pruned int32) (ctrl.Result, error) {
	nowTime := now()
	secretHash := computeSecretVersionHash(r.ctx, r.Client, r.policy.Namespace, r.repo, r.server, r.secret)
	repoRevision := r.repo.Status.LastFetchedRevision

	r.policy.Status.LastAppliedRepoRevision = repoRevision
	r.policy.Status.LastSecretResourceVersionHash = secretHash
	r.policy.Status.LastSuccessfulSyncTime = &nowTime
	if plan.Mode == syncModeFull {
		r.policy.Status.LastFullResyncTime = &nowTime
	}
	r.policy.Status.LastSyncMode = string(plan.Mode)
	r.policy.Status.ResourceStats = declarestv1alpha1.SyncPolicyResourceStats{
		Targeted: targeted,
		Applied:  applied,
		Pruned:   pruned,
		Failed:   targeted - applied,
	}
	r.policy.Status.Conditions = setStatusCondition(r.policy.Status.Conditions, declarestv1alpha1.ConditionTypeReconciling, metav1.ConditionFalse, conditionReasonReady, "")
	r.policy.Status.Conditions = setStatusCondition(r.policy.Status.Conditions, declarestv1alpha1.ConditionTypeReady, metav1.ConditionTrue, conditionReasonReady, "sync successful")
	r.policy.Status.Conditions = setStatusCondition(r.policy.Status.Conditions, declarestv1alpha1.ConditionTypeStalled, metav1.ConditionFalse, conditionReasonReady, "")
	if err := r.Status().Update(r.ctx, r.policy); err != nil {
		r.resultLabel = "error"
		r.reasonLabel = conditionReasonReconcileFailed
		return ctrl.Result{}, err
	}

	syncPolicyResourcesAppliedTotal.WithLabelValues(r.req.Namespace, r.req.Name).Add(float64(applied))
	syncPolicyResourcesPrunedTotal.WithLabelValues(r.req.Namespace, r.req.Name).Add(float64(pruned))
	emitEventf(
		r.Recorder,
		r.policy,
		corev1.EventTypeNormal,
		"SyncSucceeded",
		"sync completed at revision %s (mode=%s applied=%d pruned=%d targeted=%d)",
		repoRevision,
		plan.Mode,
		applied,
		pruned,
		targeted,
	)
	r.logger.Info("sync policy reconciled", "mode", plan.Mode, "applied", applied, "pruned", pruned, "repo_revision", repoRevision)
	return ctrl.Result{RequeueAfter: syncPolicyRequeueAfter(r.policy, time.Now().UTC())}, nil
}

func (r *syncPolicyReconciliation) loadResourceRepository() (*declarestv1alpha1.ResourceRepository, error) {
	repo := &declarestv1alpha1.ResourceRepository{}
	if err := r.Get(r.ctx, types.NamespacedName{Namespace: r.policy.Namespace, Name: r.policy.Spec.ResourceRepositoryRef.Name}, repo); err != nil {
		return nil, err
	}
	repo = expandRuntimeResourceRepository(repo)
	repo.Default()
	if err := repo.ValidateSpec(); err != nil {
		return nil, fmt.Errorf("invalid referenced resource repository: %w", err)
	}
	return repo, nil
}

func (r *syncPolicyReconciliation) loadManagedServer() (*declarestv1alpha1.ManagedServer, error) {
	server := &declarestv1alpha1.ManagedServer{}
	if err := r.Get(r.ctx, types.NamespacedName{Namespace: r.policy.Namespace, Name: r.policy.Spec.ManagedServerRef.Name}, server); err != nil {
		return nil, err
	}
	server = expandRuntimeManagedServer(server)
	server.Default()
	if err := server.ValidateSpec(); err != nil {
		return nil, fmt.Errorf("invalid referenced managed server: %w", err)
	}
	return server, nil
}

func (r *syncPolicyReconciliation) loadSecretStore() (*declarestv1alpha1.SecretStore, error) {
	secret := &declarestv1alpha1.SecretStore{}
	if err := r.Get(r.ctx, types.NamespacedName{Namespace: r.policy.Namespace, Name: r.policy.Spec.SecretStoreRef.Name}, secret); err != nil {
		return nil, err
	}
	secret = expandRuntimeSecretStore(secret)
	if err := secret.ValidateSpec(); err != nil {
		return nil, fmt.Errorf("invalid referenced secret store: %w", err)
	}
	return secret, nil
}

func (r *syncPolicyReconciliation) buildExecutionPlan(repoPath, repoRevision string) (*syncExecutionPlan, error) {
	secretHash := computeSecretVersionHash(r.ctx, r.Client, r.policy.Namespace, r.repo, r.server, r.secret)
	secretHashChanged := r.policy.Status.LastSecretResourceVersionHash != secretHash
	currentTime := time.Now().UTC()
	fullResyncDue, err := shouldRunFullResync(r.policy.Spec.FullResyncCron, r.policy.Status.LastFullResyncTime, currentTime)
	if err != nil {
		return nil, err
	}

	plan, err := buildSyncExecutionPlan(
		r.ctx,
		r.policy,
		repoPath,
		repoRevision,
		secretHashChanged,
		fullResyncDue,
	)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *syncPolicyReconciliation) applyChanges(session bootstrap.Session, plan *syncExecutionPlan) (int32, int32, error) {
	var targetedCount, appliedCount int32
	for _, target := range plan.ApplyTargets {
		mutationResult, mutateErr := mutateapp.Execute(r.ctx, mutateapp.Dependencies{
			Orchestrator: session.Orchestrator,
			Repository:   session.Services.RepositoryStore(),
			Metadata:     session.Services.MetadataService(),
			Secrets:      session.Services.SecretProvider(),
		}, mutateapp.Request{
			Operation:        mutateapp.OperationApply,
			LogicalPath:      target.Path,
			Recursive:        target.Recursive,
			Force:            r.policy.Spec.Sync.Force,
			HasExplicitInput: false,
			RefreshLocal:     false,
		})
		if mutateErr != nil {
			return 0, 0, mutateErr
		}
		targetedCount += int32(mutationResult.TargetedCount)
		appliedCount += int32(len(mutationResult.Items))
	}
	return targetedCount, appliedCount, nil
}

func (r *syncPolicyReconciliation) pruneChanges(session bootstrap.Session, plan *syncExecutionPlan) (int32, error) {
	if !r.policy.Spec.Sync.Prune {
		return 0, nil
	}

	var deleted int
	var pruneErr error
	if plan.Mode == syncModeIncremental {
		deleted, pruneErr = r.pruneRemovedPaths(r.ctx, session.Orchestrator, plan.PruneTargets)
	} else {
		recursive := r.policy.Spec.Source.Recursive == nil || *r.policy.Spec.Source.Recursive
		deleted, pruneErr = r.pruneRemote(r.ctx, session.Orchestrator, r.policy.Spec.Source.Path, recursive)
	}
	if pruneErr != nil {
		return 0, pruneErr
	}
	return int32(deleted), nil
}

func (r *SyncPolicyReconciler) pruneRemote(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
	recursive bool,
) (int, error) {
	localResources, err := orchestratorService.ListLocal(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: recursive})
	if err != nil {
		return 0, fmt.Errorf("list local resources for prune: %w", err)
	}
	remoteResources, err := orchestratorService.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: recursive})
	if err != nil {
		return 0, fmt.Errorf("list remote resources for prune: %w", err)
	}

	localPaths := make(map[string]struct{}, len(localResources))
	for _, item := range localResources {
		localPaths[item.LogicalPath] = struct{}{}
	}

	candidates := make([]string, 0)
	for _, remote := range remoteResources {
		if _, exists := localPaths[remote.LogicalPath]; exists {
			continue
		}
		candidates = append(candidates, remote.LogicalPath)
	}
	sort.Strings(candidates)

	deleted := 0
	var errs []error
	for _, candidate := range candidates {
		if err := orchestratorService.Delete(ctx, candidate, orchestratordomain.DeletePolicy{}); err != nil {
			errs = append(errs, fmt.Errorf("prune %q: %w", candidate, err))
			continue
		}
		deleted++
	}
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return deleted, fmt.Errorf("prune completed with %d errors (deleted=%d): %s", len(errs), deleted, strings.Join(msgs, "; "))
	}
	return deleted, nil
}

func (r *SyncPolicyReconciler) pruneRemovedPaths(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPaths []string,
) (int, error) {
	candidates := stringSet(logicalPaths)
	if len(candidates) == 0 {
		return 0, nil
	}

	deleted := 0
	var errs []error
	for _, candidate := range candidates {
		if err := orchestratorService.Delete(ctx, candidate, orchestratordomain.DeletePolicy{}); err != nil {
			if faults.IsCategory(err, faults.NotFoundError) {
				continue
			}
			errs = append(errs, fmt.Errorf("prune %q: %w", candidate, err))
			continue
		}
		deleted++
	}

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, item := range errs {
			msgs[i] = item.Error()
		}
		return deleted, fmt.Errorf("prune completed with %d errors (deleted=%d): %s", len(errs), deleted, strings.Join(msgs, "; "))
	}
	return deleted, nil
}

func (r *SyncPolicyReconciler) validateNoOverlap(ctx context.Context, syncPolicy *declarestv1alpha1.SyncPolicy) error {
	policies := &declarestv1alpha1.SyncPolicyList{}
	if err := r.List(ctx, policies, client.InNamespace(syncPolicy.Namespace)); err != nil {
		return err
	}
	for idx := range policies.Items {
		item := &policies.Items[idx]
		expandedItem := expandRuntimeSyncPolicy(item)
		if item.Name == syncPolicy.Name {
			continue
		}
		if item.DeletionTimestamp != nil {
			continue
		}
		if hasPathOverlap(expandedItem.Spec.Source.Path, syncPolicy.Spec.Source.Path) {
			return fmt.Errorf(
				"sync policy scope overlaps with %s/%s (%q)",
				item.Namespace,
				item.Name,
				normalizeOverlapPath(expandedItem.Spec.Source.Path),
			)
		}
	}
	return nil
}

func (r *SyncPolicyReconciler) setNotReady(
	ctx context.Context,
	syncPolicy *declarestv1alpha1.SyncPolicy,
	reason string,
	message string,
) error {
	nowTime := now()
	syncPolicy.Status.ObservedGeneration = syncPolicy.Generation
	syncPolicy.Status.LastAttemptTime = &nowTime
	syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeReconciling, metav1.ConditionFalse, reason, "")
	syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeReady, metav1.ConditionFalse, reason, message)
	syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeStalled, metav1.ConditionTrue, reason, message)
	return r.Status().Update(ctx, syncPolicy)
}

func (r *SyncPolicyReconciler) failWithStatus(
	ctx context.Context,
	syncPolicy *declarestv1alpha1.SyncPolicy,
	reason string,
	message string,
	requeueAfter time.Duration,
	eventReason string,
) (ctrl.Result, error) {
	emitEventf(r.Recorder, syncPolicy, corev1.EventTypeWarning, eventReason, "%s", strings.TrimSpace(message))
	return returnAfterSetNotReady(
		ctx,
		func(innerCtx context.Context, statusReason string, statusMessage string) error {
			return r.setNotReady(innerCtx, syncPolicy, statusReason, statusMessage)
		},
		reason,
		message,
		requeueAfter,
	)
}

func (r *SyncPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&declarestv1alpha1.SyncPolicy{},
		syncPolicyIndexResourceRepositoryRef,
		func(obj client.Object) []string {
			syncPolicy, ok := obj.(*declarestv1alpha1.SyncPolicy)
			if !ok {
				return nil
			}
			name := strings.TrimSpace(syncPolicy.Spec.ResourceRepositoryRef.Name)
			if name == "" {
				return nil
			}
			return []string{name}
		},
	); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&declarestv1alpha1.SyncPolicy{},
		syncPolicyIndexManagedServerRef,
		func(obj client.Object) []string {
			syncPolicy, ok := obj.(*declarestv1alpha1.SyncPolicy)
			if !ok {
				return nil
			}
			name := strings.TrimSpace(syncPolicy.Spec.ManagedServerRef.Name)
			if name == "" {
				return nil
			}
			return []string{name}
		},
	); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&declarestv1alpha1.SyncPolicy{},
		syncPolicyIndexSecretStoreRef,
		func(obj client.Object) []string {
			syncPolicy, ok := obj.(*declarestv1alpha1.SyncPolicy)
			if !ok {
				return nil
			}
			name := strings.TrimSpace(syncPolicy.Spec.SecretStoreRef.Name)
			if name == "" {
				return nil
			}
			return []string{name}
		},
	); err != nil {
		return err
	}

	bld := ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.SyncPolicy{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&declarestv1alpha1.ResourceRepository{},
			handler.EnqueueRequestsFromMapFunc(r.syncPoliciesForResourceRepository),
		).
		Watches(
			&declarestv1alpha1.ManagedServer{},
			handler.EnqueueRequestsFromMapFunc(r.syncPoliciesForManagedServer),
		).
		Watches(
			&declarestv1alpha1.SecretStore{},
			handler.EnqueueRequestsFromMapFunc(r.syncPoliciesForSecretStore),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.syncPoliciesForSecret),
		)
	if r.MaxConcurrentReconciles > 0 {
		bld = bld.WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	}
	return bld.Complete(r)
}

func (r *SyncPolicyReconciler) syncPoliciesForResourceRepository(ctx context.Context, obj client.Object) []reconcile.Request {
	resourceRepository, ok := obj.(*declarestv1alpha1.ResourceRepository)
	if !ok {
		return nil
	}
	policies := &declarestv1alpha1.SyncPolicyList{}
	if err := r.List(
		ctx,
		policies,
		client.InNamespace(resourceRepository.Namespace),
		client.MatchingFields{syncPolicyIndexResourceRepositoryRef: resourceRepository.Name},
	); err != nil {
		log.FromContext(ctx).Error(err, "failed to list SyncPolicies for watch mapper", "trigger_kind", "ResourceRepository", "trigger_name", resourceRepository.Name)
		return nil
	}
	return reconcileRequestsForSyncPolicies(policies.Items)
}

func (r *SyncPolicyReconciler) syncPoliciesForManagedServer(ctx context.Context, obj client.Object) []reconcile.Request {
	managedServer, ok := obj.(*declarestv1alpha1.ManagedServer)
	if !ok {
		return nil
	}
	policies := &declarestv1alpha1.SyncPolicyList{}
	if err := r.List(
		ctx,
		policies,
		client.InNamespace(managedServer.Namespace),
		client.MatchingFields{syncPolicyIndexManagedServerRef: managedServer.Name},
	); err != nil {
		log.FromContext(ctx).Error(err, "failed to list SyncPolicies for watch mapper", "trigger_kind", "ManagedServer", "trigger_name", managedServer.Name)
		return nil
	}
	return reconcileRequestsForSyncPolicies(policies.Items)
}

func (r *SyncPolicyReconciler) syncPoliciesForSecretStore(ctx context.Context, obj client.Object) []reconcile.Request {
	secretStore, ok := obj.(*declarestv1alpha1.SecretStore)
	if !ok {
		return nil
	}
	policies := &declarestv1alpha1.SyncPolicyList{}
	if err := r.List(
		ctx,
		policies,
		client.InNamespace(secretStore.Namespace),
		client.MatchingFields{syncPolicyIndexSecretStoreRef: secretStore.Name},
	); err != nil {
		log.FromContext(ctx).Error(err, "failed to list SyncPolicies for watch mapper", "trigger_kind", "SecretStore", "trigger_name", secretStore.Name)
		return nil
	}
	return reconcileRequestsForSyncPolicies(policies.Items)
}

func (r *SyncPolicyReconciler) syncPoliciesForSecret(_ context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	return r.secretRefs.policiesForSecret(secret.Namespace, secret.Name)
}

func reconcileRequestsForSyncPolicies(items []declarestv1alpha1.SyncPolicy) []reconcile.Request {
	requests := make([]reconcile.Request, 0, len(items))
	for idx := range items {
		item := &items[idx]
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.Namespace,
				Name:      item.Name,
			},
		})
	}
	return requests
}
