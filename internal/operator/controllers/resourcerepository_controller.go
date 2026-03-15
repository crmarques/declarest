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
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"
	sshauth "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ResourceRepositoryReconciler reconciles ResourceRepository resources.
type ResourceRepositoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *ResourceRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("resourceRepository", req.NamespacedName.String(), "reconcile_id", uuid.NewString())

	resourceRepository := &declarestv1alpha1.ResourceRepository{}
	if err := r.Get(ctx, req.NamespacedName, resourceRepository); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(resourceRepository, finalizerName) {
		controllerutil.AddFinalizer(resourceRepository, finalizerName)
		if err := r.Update(ctx, resourceRepository); err != nil {
			return ctrl.Result{}, err
		}
	}
	if !resourceRepository.DeletionTimestamp.IsZero() {
		localPath := resolveRepoRootPath(resourceRepository.Namespace, resourceRepository.Name)
		if err := os.RemoveAll(localPath); err != nil {
			logger.Error(err, "failed to clean up repository directory", "path", localPath)
			emitEventf(r.Recorder, resourceRepository, corev1.EventTypeWarning, "CleanupFailed", "failed to remove repository directory: %v", err)
		}
		cacheDir := resolveCacheRootPath(resourceRepository.Namespace, resourceRepository.Name)
		if err := os.RemoveAll(cacheDir); err != nil {
			logger.Error(err, "failed to clean up cache directory", "path", cacheDir)
		}
		controllerutil.RemoveFinalizer(resourceRepository, finalizerName)
		return ctrl.Result{}, r.Update(ctx, resourceRepository)
	}

	runtimeRepository := expandRuntimeResourceRepository(resourceRepository)
	runtimeRepository.Default()
	if validationErr := runtimeRepository.ValidateSpec(); validationErr != nil {
		logger.Error(validationErr, "resource repository spec validation failed")
		emitEventf(r.Recorder, resourceRepository, corev1.EventTypeWarning, "SpecInvalid", "validation failed: %v", validationErr)
		resourceRepositoryPollTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, resourceRepository, reason, message)
			},
			conditionReasonSpecInvalid,
			validationErr.Error(),
			runtimeRepository.Spec.PollInterval.Duration,
		)
	}

	if err := r.ensurePVC(ctx, runtimeRepository); err != nil {
		emitEventf(r.Recorder, resourceRepository, corev1.EventTypeWarning, "DependencyInvalid", "dependency validation failed: %v", err)
		resourceRepositoryPollTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, resourceRepository, reason, message)
			},
			conditionReasonDependencyInvalid,
			err.Error(),
			runtimeRepository.Spec.PollInterval.Duration,
		)
	}

	localPath := resolveRepoRootPath(resourceRepository.Namespace, resourceRepository.Name)
	revision, syncErr := r.syncRepository(ctx, runtimeRepository, localPath)
	if syncErr != nil {
		resourceRepositoryPollTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		logger.Error(syncErr, "repository poll failed", "git_url", sanitizeURL(runtimeRepository.Spec.Git.URL))
		emitEventf(r.Recorder, resourceRepository, corev1.EventTypeWarning, "SyncFailed", "repository sync failed: %v", syncErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, resourceRepository, reason, message)
			},
			conditionReasonReconcileFailed,
			syncErr.Error(),
			runtimeRepository.Spec.PollInterval.Duration,
		)
	}

	resourceRepositoryPollTotal.WithLabelValues(req.Namespace, req.Name, "success").Inc()
	if revision != "" && revision != resourceRepository.Status.LastFetchedRevision {
		resourceRepositoryRevisionChangesTotal.WithLabelValues(req.Namespace, req.Name).Inc()
		emitEventf(r.Recorder, resourceRepository, corev1.EventTypeNormal, "RevisionChanged", "new revision: %s", revision)
	}

	nowTime := now()
	resourceRepository.Status.ObservedGeneration = resourceRepository.Generation
	resourceRepository.Status.LocalPath = localPath
	resourceRepository.Status.LastFetchedRevision = revision
	resourceRepository.Status.LastFetchedTime = &nowTime
	resourceRepository.Status.Conditions = setStatusCondition(
		resourceRepository.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionTrue,
		conditionReasonReady,
		"repository fetched successfully",
	)
	resourceRepository.Status.Conditions = setStatusCondition(
		resourceRepository.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionFalse,
		conditionReasonReady,
		"",
	)

	if err := r.Status().Update(ctx, resourceRepository); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info(
		"repository reconciled",
		"revision", revision,
		"poll_interval", runtimeRepository.Spec.PollInterval.Duration.String(),
	)
	return ctrl.Result{RequeueAfter: runtimeRepository.Spec.PollInterval.Duration}, nil
}

func (r *ResourceRepositoryReconciler) ensurePVC(ctx context.Context, resourceRepository *declarestv1alpha1.ResourceRepository) error {
	if resourceRepository.Spec.Storage.ExistingPVC != nil {
		name := strings.TrimSpace(resourceRepository.Spec.Storage.ExistingPVC.Name)
		existing := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: resourceRepository.Namespace, Name: name}, existing); err != nil {
			return fmt.Errorf("resolve existing PVC %q: %w", name, err)
		}
		return nil
	}

	if resourceRepository.Spec.Storage.PVC == nil {
		return fmt.Errorf("storage pvc template is required")
	}
	pvcName := fmt.Sprintf("%s-repo", resourceRepository.Name)
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Namespace: resourceRepository.Namespace, Name: pvcName}, pvc)
	if apierrors.IsNotFound(err) {
		pvc.Namespace = resourceRepository.Namespace
		pvc.Name = pvcName
		pvc.Labels = mergeStringMap(nil, map[string]string{
			"app.kubernetes.io/name":     "declarest-operator",
			"declarest.io/resource-repo": resourceRepository.Name,
		})
		if refErr := controllerutil.SetControllerReference(resourceRepository, pvc, r.Scheme); refErr != nil {
			return refErr
		}
		pvc.Spec.AccessModes = append([]corev1.PersistentVolumeAccessMode(nil), resourceRepository.Spec.Storage.PVC.AccessModes...)
		pvc.Spec.Resources.Requests = resourceRepository.Spec.Storage.PVC.Requests.DeepCopy()
		pvc.Spec.StorageClassName = resourceRepository.Spec.Storage.PVC.StorageClassName
		if createErr := r.Create(ctx, pvc); createErr != nil {
			return fmt.Errorf("create managed PVC %q: %w", pvcName, createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get managed PVC %q: %w", pvcName, err)
	}
	return nil
}

const gitOperationTimeout = 5 * time.Minute

func (r *ResourceRepositoryReconciler) syncRepository(ctx context.Context, resourceRepository *declarestv1alpha1.ResourceRepository, localPath string) (string, error) {
	if err := ensureDir(filepath.Dir(localPath)); err != nil {
		return "", err
	}
	authMethod, cleanup, err := r.gitAuthMethod(ctx, resourceRepository)
	if err != nil {
		return "", err
	}
	defer cleanup()

	branch := strings.TrimSpace(resourceRepository.Spec.Git.Branch)
	if branch == "" {
		branch = "main"
	}

	// Apply an explicit timeout for git operations to prevent a slow or
	// unresponsive git server from blocking the controller's work queue.
	gitCtx, cancel := context.WithTimeout(ctx, gitOperationTimeout)
	defer cancel()

	// Try incremental fetch on an existing clone first. This avoids a full
	// re-clone on every reconciliation when nothing has changed.
	if rev, fetchErr := r.tryFetch(gitCtx, localPath, authMethod, branch); fetchErr == nil {
		return rev, nil
	}

	// Fall back to a full shallow clone if fetch failed (missing dir,
	// corrupted repo, branch mismatch, etc.).
	tmpPath := fmt.Sprintf("%s-tmp-%d", localPath, time.Now().UnixNano())
	cloneOptions := &gogit.CloneOptions{
		URL:           strings.TrimSpace(resourceRepository.Spec.Git.URL),
		Auth:          authMethod,
		SingleBranch:  true,
		Depth:         1,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Progress:      nil,
	}
	if _, err := gogit.PlainCloneContext(gitCtx, tmpPath, false, cloneOptions); err != nil {
		return "", fmt.Errorf("clone repository %s: %w", sanitizeURL(resourceRepository.Spec.Git.URL), err)
	}

	if removeErr := os.RemoveAll(localPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		_ = os.RemoveAll(tmpPath)
		return "", fmt.Errorf("remove stale local repository path: %w", removeErr)
	}
	if err := os.Rename(tmpPath, localPath); err != nil {
		_ = os.RemoveAll(tmpPath)
		return "", fmt.Errorf("move cloned repository into place: %w", err)
	}

	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("open synced repository: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("resolve synced repository head: %w", err)
	}
	return head.Hash().String(), nil
}

// tryFetch attempts an incremental fetch on an existing shallow clone,
// resetting the working tree to the latest remote HEAD. Returns the new
// HEAD revision on success or an error if a full re-clone is needed.
func (r *ResourceRepositoryReconciler) tryFetch(
	ctx context.Context,
	localPath string,
	authMethod transport.AuthMethod,
	branch string,
) (string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", err
	}

	fetchOpts := &gogit.FetchOptions{
		Auth:       authMethod,
		Depth:      1,
		Force:      true,
		RemoteName: "origin",
		RefSpecs:   []gogitconfig.RefSpec{gogitconfig.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch))},
	}
	if fetchErr := repo.FetchContext(ctx, fetchOpts); fetchErr != nil && fetchErr != gogit.NoErrAlreadyUpToDate {
		return "", fetchErr
	}

	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return "", err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	if err := worktree.Reset(&gogit.ResetOptions{Commit: remoteRef.Hash(), Mode: gogit.HardReset}); err != nil {
		return "", err
	}

	return remoteRef.Hash().String(), nil
}

func (r *ResourceRepositoryReconciler) gitAuthMethod(
	ctx context.Context,
	resourceRepository *declarestv1alpha1.ResourceRepository,
) (transport.AuthMethod, func(), error) {
	cleanup := func() {}
	if resourceRepository.Spec.Git.Auth.TokenRef != nil {
		token, err := readSecretValueFromClient(ctx, r.Client, resourceRepository.Namespace, resourceRepository.Spec.Git.Auth.TokenRef)
		if err != nil {
			return nil, cleanup, err
		}
		username := "token"
		parsedURL, parseErr := url.Parse(strings.TrimSpace(resourceRepository.Spec.Git.URL))
		if parseErr == nil && parsedURL.User != nil {
			if candidate := strings.TrimSpace(parsedURL.User.Username()); candidate != "" {
				username = candidate
			}
		}
		return &httpauth.BasicAuth{Username: username, Password: token}, cleanup, nil
	}

	sshAuth := resourceRepository.Spec.Git.Auth.SSHSecretRef
	if sshAuth == nil || sshAuth.PrivateKeyRef == nil {
		return nil, cleanup, fmt.Errorf("ssh private key reference is required")
	}
	privateKey, err := readSecretValueFromClient(ctx, r.Client, resourceRepository.Namespace, sshAuth.PrivateKeyRef)
	if err != nil {
		return nil, cleanup, err
	}
	passphrase := ""
	if sshAuth.PassphraseRef != nil {
		passphrase, err = readSecretValueFromClient(ctx, r.Client, resourceRepository.Namespace, sshAuth.PassphraseRef)
		if err != nil {
			return nil, cleanup, err
		}
	}
	sshUser := strings.TrimSpace(sshAuth.User)
	if sshUser == "" {
		sshUser = "git"
	}
	publicKeys, err := sshauth.NewPublicKeys(sshUser, []byte(privateKey), passphrase)
	if err != nil {
		return nil, cleanup, fmt.Errorf("load ssh auth: %w", err)
	}

	if sshAuth.KnownHostsRef != nil {
		knownHostsValue, err := readSecretValueFromClient(ctx, r.Client, resourceRepository.Namespace, sshAuth.KnownHostsRef)
		if err != nil {
			return nil, cleanup, err
		}
		tmpDir := filepath.Join(resolveCacheRootPath(resourceRepository.Namespace, resourceRepository.Name), "knownhosts")
		knownHostsPath, writeErr := writeSecretValueToFile(tmpDir, "known_hosts", knownHostsValue)
		if writeErr != nil {
			return nil, cleanup, writeErr
		}
		cleanup = func() {
			_ = os.Remove(knownHostsPath)
		}
		hostKeyCallback, err := xknownhosts.New(knownHostsPath)
		if err != nil {
			return nil, cleanup, fmt.Errorf("load known_hosts: %w", err)
		}
		publicKeys.HostKeyCallback = hostKeyCallback
	} else if sshAuth.InsecureIgnoreHostKey {
		log.FromContext(ctx).Info("WARNING: SSH host key verification is disabled, connection is susceptible to MITM attacks",
			"resourceRepository", resourceRepository.Name,
			"namespace", resourceRepository.Namespace,
		)
		publicKeys.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		return nil, cleanup, fmt.Errorf("ssh known_hosts reference is required; set insecureIgnoreHostKey: true to skip host key verification (not recommended)")
	}

	return publicKeys, cleanup, nil
}

func (r *ResourceRepositoryReconciler) setNotReady(
	ctx context.Context,
	resourceRepository *declarestv1alpha1.ResourceRepository,
	reason string,
	message string,
) error {
	resourceRepository.Status.ObservedGeneration = resourceRepository.Generation
	resourceRepository.Status.Conditions = setStatusCondition(
		resourceRepository.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionFalse,
		reason,
		message,
	)
	resourceRepository.Status.Conditions = setStatusCondition(
		resourceRepository.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionTrue,
		reason,
		message,
	)
	return r.Status().Update(ctx, resourceRepository)
}

func (r *ResourceRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.ResourceRepository{}, builder.WithPredicates(resourceRepositoryReconcilePredicate())).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func resourceRepositoryReconcilePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(event.GenericEvent) bool {
			return true
		},
		UpdateFunc: func(update event.UpdateEvent) bool {
			if update.ObjectOld == nil || update.ObjectNew == nil {
				return true
			}
			if update.ObjectOld.GetGeneration() != update.ObjectNew.GetGeneration() {
				return true
			}
			oldAnnotations := update.ObjectOld.GetAnnotations()
			newAnnotations := update.ObjectNew.GetAnnotations()
			return strings.TrimSpace(oldAnnotations[repositoryWebhookAnnotationLastEventAt]) != strings.TrimSpace(newAnnotations[repositoryWebhookAnnotationLastEventAt]) ||
				strings.TrimSpace(oldAnnotations[repositoryWebhookAnnotationLastEventID]) != strings.TrimSpace(newAnnotations[repositoryWebhookAnnotationLastEventID])
		},
	}
}

func mergeStringMap(left map[string]string, right map[string]string) map[string]string {
	out := make(map[string]string, len(left)+len(right))
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}
