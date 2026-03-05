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
	c.index[key] = sets.New[string](secretNames...)
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
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	secretRefs secretRefCache
}

const (
	syncPolicyIndexResourceRepositoryRef = "spec.resourceRepositoryRef.name"
	syncPolicyIndexManagedServerRef      = "spec.managedServerRef.name"
	syncPolicyIndexSecretStoreRef        = "spec.secretStoreRef.name"
)

func (r *SyncPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reconcileErr error) {
	start := time.Now()
	logger := log.FromContext(ctx).WithValues("syncPolicy", req.NamespacedName.String(), "reconcile_id", uuidString())

	syncPolicy := &declarestv1alpha1.SyncPolicy{}
	if err := r.Get(ctx, req.NamespacedName, syncPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(syncPolicy, finalizerName) {
		controllerutil.AddFinalizer(syncPolicy, finalizerName)
		if err := r.Update(ctx, syncPolicy); err != nil {
			return ctrl.Result{}, err
		}
	}
	if !syncPolicy.DeletionTimestamp.IsZero() {
		r.secretRefs.remove(req.NamespacedName)
		cacheDir := resolveCacheRootPath(syncPolicy.Namespace, syncPolicy.Name)
		if err := os.RemoveAll(cacheDir); err != nil {
			logger.Error(err, "failed to clean up cache directory", "path", cacheDir)
		}
		controllerutil.RemoveFinalizer(syncPolicy, finalizerName)
		return ctrl.Result{}, r.Update(ctx, syncPolicy)
	}

	syncPolicy.Default()

	resultLabel := "success"
	reasonLabel := conditionReasonReady
	defer func() {
		duration := time.Since(start).Seconds()
		syncPolicyReconcileTotal.WithLabelValues(req.Namespace, req.Name, resultLabel, reasonLabel).Inc()
		syncPolicyReconcileDurationSeconds.WithLabelValues(req.Namespace, req.Name, resultLabel).Observe(duration)
	}()

	if validationErr := syncPolicy.ValidateSpec(); validationErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonSpecInvalid
		return r.failWithStatus(ctx, syncPolicy, conditionReasonSpecInvalid, validationErr.Error(), 0, "SpecInvalid")
	}

	if syncPolicy.Spec.Suspend {
		nowTime := now()
		syncPolicy.Status.ObservedGeneration = syncPolicy.Generation
		syncPolicy.Status.LastAttemptTime = &nowTime
		syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeReady, metav1.ConditionFalse, conditionReasonSuspended, "sync policy is suspended")
		syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeStalled, metav1.ConditionFalse, conditionReasonSuspended, "")
		syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeReconciling, metav1.ConditionFalse, conditionReasonSuspended, "")
		if err := r.Status().Update(ctx, syncPolicy); err != nil {
			resultLabel = "error"
			reasonLabel = conditionReasonSuspended
			return ctrl.Result{}, err
		}
		emitEventf(r.Recorder, syncPolicy, corev1.EventTypeNormal, "Suspended", "sync policy is suspended")
		reasonLabel = conditionReasonSuspended
		return ctrl.Result{}, nil
	}

	if overlapErr := r.validateNoOverlap(ctx, syncPolicy); overlapErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonOverlappingPolicy
		return r.failWithStatus(ctx, syncPolicy, conditionReasonOverlappingPolicy, overlapErr.Error(), 0, "OverlappingPolicy")
	}

	repo := &declarestv1alpha1.ResourceRepository{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: syncPolicy.Namespace, Name: syncPolicy.Spec.ResourceRepositoryRef.Name}, repo); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(
			ctx,
			syncPolicy,
			conditionReasonDependencyInvalid,
			fmt.Sprintf("resolve resource repository: %v", err),
			0,
			"DependencyInvalid",
		)
	}
	repo.Default()
	if err := repo.ValidateSpec(); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(
			ctx,
			syncPolicy,
			conditionReasonDependencyInvalid,
			fmt.Sprintf("invalid referenced resource repository: %v", err),
			0,
			"DependencyInvalid",
		)
	}

	managedServer := &declarestv1alpha1.ManagedServer{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: syncPolicy.Namespace, Name: syncPolicy.Spec.ManagedServerRef.Name}, managedServer); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(
			ctx,
			syncPolicy,
			conditionReasonDependencyInvalid,
			fmt.Sprintf("resolve managed server: %v", err),
			0,
			"DependencyInvalid",
		)
	}
	managedServer.Default()
	if err := managedServer.ValidateSpec(); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(
			ctx,
			syncPolicy,
			conditionReasonDependencyInvalid,
			fmt.Sprintf("invalid referenced managed server: %v", err),
			0,
			"DependencyInvalid",
		)
	}

	secretStore := &declarestv1alpha1.SecretStore{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: syncPolicy.Namespace, Name: syncPolicy.Spec.SecretStoreRef.Name}, secretStore); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(
			ctx,
			syncPolicy,
			conditionReasonDependencyInvalid,
			fmt.Sprintf("resolve secret store: %v", err),
			0,
			"DependencyInvalid",
		)
	}
	if err := secretStore.ValidateSpec(); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(
			ctx,
			syncPolicy,
			conditionReasonDependencyInvalid,
			fmt.Sprintf("invalid referenced secret store: %v", err),
			0,
			"DependencyInvalid",
		)
	}

	// Update the secret reference cache so the Secret watch mapper can look
	// up affected SyncPolicies without resolving dependency CRDs each time.
	r.secretRefs.update(req.NamespacedName, collectSecretNames(repo, managedServer, secretStore))

	repoRevision := strings.TrimSpace(repo.Status.LastFetchedRevision)
	if repoRevision == "" {
		reasonLabel = conditionReasonDependencyInvalid
		resultLabel = "error"
		return r.failWithStatus(
			ctx,
			syncPolicy,
			conditionReasonDependencyInvalid,
			"resource repository has no fetched revision yet",
			repo.Spec.PollInterval.Duration,
			"DependencyInvalid",
		)
	}

	secretHash := computeSecretVersionHash(ctx, r.Client, syncPolicy.Namespace, repo, managedServer, secretStore)
	secretHashChanged := syncPolicy.Status.LastSecretResourceVersionHash != secretHash
	currentTime := time.Now().UTC()
	fullResyncDue, fullResyncErr := shouldRunFullResync(syncPolicy.Spec.FullResyncCron, syncPolicy.Status.LastFullResyncTime, currentTime)
	if fullResyncErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonSpecInvalid
		return r.failWithStatus(ctx, syncPolicy, conditionReasonSpecInvalid, fmt.Sprintf("invalid full resync cron: %v", fullResyncErr), 0, "SpecInvalid")
	}
	if syncPolicy.Status.ObservedGeneration == syncPolicy.Generation &&
		syncPolicy.Status.LastAppliedRepoRevision == repoRevision &&
		!secretHashChanged &&
		!fullResyncDue {
		logger.Info("sync policy is already at desired state", "revision", repoRevision)
		return ctrl.Result{RequeueAfter: syncPolicyRequeueAfter(syncPolicy, currentTime)}, nil
	}

	runtimeBuild, runtimeErr := buildRuntimeContext(ctx, r.Client, syncPolicy, repo, managedServer, secretStore)
	if runtimeErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonRepositoryUnavailable
		return r.failWithStatus(ctx, syncPolicy, conditionReasonRepositoryUnavailable, runtimeErr.Error(), 0, "RuntimeContextFailed")
	}
	if runtimeBuild.Cleanup != nil {
		defer runtimeBuild.Cleanup()
	}

	session, sessionErr := bootstrap.NewSessionFromResolvedContext(runtimeBuild.ResolvedContext)
	if sessionErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonSessionBootstrapFailed
		return r.failWithStatus(ctx, syncPolicy, conditionReasonSessionBootstrapFailed, sessionErr.Error(), 0, "SessionBootstrapFailed")
	}

	nowTime := now()
	syncPolicy.Status.ObservedGeneration = syncPolicy.Generation
	syncPolicy.Status.LastAttemptTime = &nowTime
	syncPolicy.Status.LastAttemptedRepoRevision = repoRevision
	syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeReconciling, metav1.ConditionTrue, conditionReasonReconciling, "applying repository state")
	if err := r.Status().Update(ctx, syncPolicy); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonReconcileFailed
		return ctrl.Result{}, err
	}

	recursive := true
	if syncPolicy.Spec.Source.Recursive != nil {
		recursive = *syncPolicy.Spec.Source.Recursive
	}
	executionPlan, planErr := buildSyncExecutionPlan(
		ctx,
		syncPolicy,
		runtimeBuild.RepositoryLocalPath,
		repoRevision,
		secretHashChanged,
		fullResyncDue,
	)
	if planErr != nil {
		logger.Info("falling back to full sync plan", "reason", planErr.Error())
		executionPlan = fullSyncExecutionPlan(syncPolicy.Spec.Source.Path, recursive)
	}

	targetedCount := int32(0)
	appliedCount := int32(0)
	for _, target := range executionPlan.ApplyTargets {
		mutationResult, mutateErr := mutateapp.Execute(ctx, mutateapp.Dependencies{
			Orchestrator: session.Orchestrator,
			Repository:   session.Services.RepositoryStore(),
			Metadata:     session.Services.MetadataService(),
			Secrets:      session.Services.SecretProvider(),
		}, mutateapp.Request{
			Operation:        mutateapp.OperationApply,
			LogicalPath:      target.Path,
			Recursive:        target.Recursive,
			Force:            syncPolicy.Spec.Sync.Force,
			HasExplicitInput: false,
			RefreshLocal:     false,
		})
		if mutateErr != nil {
			resultLabel = "error"
			reasonLabel = conditionReasonReconcileFailed
			return r.failWithStatus(ctx, syncPolicy, conditionReasonReconcileFailed, mutateErr.Error(), 0, "SyncFailed")
		}
		targetedCount += int32(mutationResult.TargetedCount)
		appliedCount += int32(len(mutationResult.Items))
	}

	prunedCount := int32(0)
	if syncPolicy.Spec.Sync.Prune {
		var deleted int
		var pruneErr error
		if executionPlan.Mode == syncModeIncremental {
			deleted, pruneErr = r.pruneRemovedPaths(ctx, session.Orchestrator, executionPlan.PruneTargets)
		} else {
			deleted, pruneErr = r.pruneRemote(ctx, session.Orchestrator, syncPolicy.Spec.Source.Path, recursive)
		}
		if pruneErr != nil {
			resultLabel = "error"
			reasonLabel = conditionReasonReconcileFailed
			return r.failWithStatus(ctx, syncPolicy, conditionReasonReconcileFailed, pruneErr.Error(), 0, "SyncFailed")
		}
		prunedCount = int32(deleted)
	}

	syncPolicy.Status.LastAppliedRepoRevision = repoRevision
	syncPolicy.Status.LastSecretResourceVersionHash = secretHash
	syncPolicy.Status.LastSuccessfulSyncTime = &nowTime
	if executionPlan.Mode == syncModeFull {
		syncPolicy.Status.LastFullResyncTime = &nowTime
	}
	syncPolicy.Status.LastSyncMode = string(executionPlan.Mode)
	syncPolicy.Status.ResourceStats = declarestv1alpha1.SyncPolicyResourceStats{
		Targeted: targetedCount,
		Applied:  appliedCount,
		Pruned:   prunedCount,
		Failed:   targetedCount - appliedCount,
	}
	syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeReconciling, metav1.ConditionFalse, conditionReasonReady, "")
	syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeReady, metav1.ConditionTrue, conditionReasonReady, "sync successful")
	syncPolicy.Status.Conditions = setStatusCondition(syncPolicy.Status.Conditions, declarestv1alpha1.ConditionTypeStalled, metav1.ConditionFalse, conditionReasonReady, "")
	if err := r.Status().Update(ctx, syncPolicy); err != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonReconcileFailed
		return ctrl.Result{}, err
	}

	syncPolicyResourcesAppliedTotal.WithLabelValues(req.Namespace, req.Name).Add(float64(appliedCount))
	syncPolicyResourcesPrunedTotal.WithLabelValues(req.Namespace, req.Name).Add(float64(prunedCount))
	emitEventf(
		r.Recorder,
		syncPolicy,
		corev1.EventTypeNormal,
		"SyncSucceeded",
		"sync completed at revision %s (mode=%s applied=%d pruned=%d targeted=%d)",
		repoRevision,
		executionPlan.Mode,
		appliedCount,
		prunedCount,
		targetedCount,
	)
	logger.Info("sync policy reconciled", "mode", executionPlan.Mode, "applied", appliedCount, "pruned", prunedCount, "repo_revision", repoRevision)
	return ctrl.Result{RequeueAfter: syncPolicyRequeueAfter(syncPolicy, time.Now().UTC())}, nil
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
		if item.Name == syncPolicy.Name {
			continue
		}
		if item.DeletionTimestamp != nil {
			continue
		}
		if hasPathOverlap(item.Spec.Source.Path, syncPolicy.Spec.Source.Path) {
			return fmt.Errorf(
				"sync policy scope overlaps with %s/%s (%q)",
				item.Namespace,
				item.Name,
				normalizeOverlapPath(item.Spec.Source.Path),
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

	return ctrl.NewControllerManagedBy(mgr).
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
		).
		Complete(r)
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
