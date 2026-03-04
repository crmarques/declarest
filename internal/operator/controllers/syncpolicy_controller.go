package controllers

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/internal/bootstrap"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SyncPolicyReconciler reconciles SyncPolicy resources.
type SyncPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const (
	syncPolicyIndexResourceRepositoryRef = "spec.resourceRepositoryRef.name"
	syncPolicyIndexManagedServerRef      = "spec.managedServerRef.name"
	syncPolicyIndexSecretStoreRef        = "spec.secretStoreRef.name"
)

func (r *SyncPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reconcileErr error) {
	registerMetrics()
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
		cacheDir := resolveCacheRootPath(syncPolicy.Namespace, syncPolicy.Name)
		_ = os.RemoveAll(cacheDir)
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
	if syncPolicy.Status.ObservedGeneration == syncPolicy.Generation &&
		syncPolicy.Status.LastAppliedRepoRevision == repoRevision &&
		syncPolicy.Status.LastSecretResourceVersionHash == secretHash {
		logger.Info("sync policy is already at desired state", "revision", repoRevision)
		return ctrl.Result{RequeueAfter: syncPolicy.Spec.SyncInterval.Duration}, nil
	}

	runtimeBuild, runtimeErr := buildRuntimeContext(ctx, r.Client, syncPolicy, repo, managedServer, secretStore)
	if runtimeErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(ctx, syncPolicy, conditionReasonDependencyInvalid, runtimeErr.Error(), 0, "DependencyInvalid")
	}
	if runtimeBuild.Cleanup != nil {
		defer runtimeBuild.Cleanup()
	}

	session, sessionErr := bootstrap.NewSessionFromResolvedContext(runtimeBuild.ResolvedContext)
	if sessionErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonDependencyInvalid
		return r.failWithStatus(ctx, syncPolicy, conditionReasonDependencyInvalid, sessionErr.Error(), 0, "DependencyInvalid")
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
	mutationResult, mutateErr := mutateapp.Execute(ctx, mutateapp.Dependencies{
		Orchestrator: session.Orchestrator,
		Repository:   session.Services.RepositoryStore(),
		Metadata:     session.Services.MetadataService(),
		Secrets:      session.Services.SecretProvider(),
	}, mutateapp.Request{
		Operation:        mutateapp.OperationApply,
		LogicalPath:      syncPolicy.Spec.Source.Path,
		Recursive:        recursive,
		Force:            syncPolicy.Spec.Sync.Force,
		HasExplicitInput: false,
		RefreshLocal:     false,
	})
	if mutateErr != nil {
		resultLabel = "error"
		reasonLabel = conditionReasonReconcileFailed
		return r.failWithStatus(ctx, syncPolicy, conditionReasonReconcileFailed, mutateErr.Error(), 0, "SyncFailed")
	}

	targetedCount := int32(mutationResult.TargetedCount)
	appliedCount := int32(len(mutationResult.Items))
	prunedCount := int32(0)
	if syncPolicy.Spec.Sync.Prune {
		deleted, pruneErr := r.pruneRemote(ctx, session.Orchestrator, syncPolicy.Spec.Source.Path, recursive)
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
		"sync completed at revision %s (applied=%d pruned=%d targeted=%d)",
		repoRevision,
		appliedCount,
		prunedCount,
		targetedCount,
	)
	logger.Info("sync policy reconciled", "applied", appliedCount, "pruned", prunedCount, "repo_revision", repoRevision)
	return ctrl.Result{RequeueAfter: syncPolicy.Spec.SyncInterval.Duration}, nil
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
	for _, candidate := range candidates {
		if err := orchestratorService.Delete(ctx, candidate, orchestratordomain.DeletePolicy{}); err != nil {
			return deleted, fmt.Errorf("prune remote resource %q: %w", candidate, err)
		}
		deleted++
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
		For(&declarestv1alpha1.SyncPolicy{}).
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

func (r *SyncPolicyReconciler) syncPoliciesForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}
	policies := &declarestv1alpha1.SyncPolicyList{}
	if err := r.List(ctx, policies, client.InNamespace(secret.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "failed to list SyncPolicies for watch mapper", "trigger_kind", "Secret", "trigger_name", secret.Name)
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for idx := range policies.Items {
		item := &policies.Items[idx]
		// Resolve dependency CRDs and check if any references this Secret.
		repo := &declarestv1alpha1.ResourceRepository{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: item.Namespace, Name: item.Spec.ResourceRepositoryRef.Name}, repo); err != nil {
			continue
		}
		ms := &declarestv1alpha1.ManagedServer{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: item.Namespace, Name: item.Spec.ManagedServerRef.Name}, ms); err != nil {
			continue
		}
		ss := &declarestv1alpha1.SecretStore{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: item.Namespace, Name: item.Spec.SecretStoreRef.Name}, ss); err != nil {
			continue
		}
		secretNames := collectSecretNames(repo, ms, ss)
		for _, name := range secretNames {
			if name == secret.Name {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: item.Namespace, Name: item.Name}})
				break
			}
		}
	}
	return requests
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
