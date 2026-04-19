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

package webhookreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	hookPathPrefix  = "/hooks/v1/repositorywebhooks/"
	maxPayloadBytes = int64(1 << 20) // 1 MiB
)

// Handler serves webhook deliveries for RepositoryWebhook objects.
type Handler struct {
	Client    client.Client
	Providers WebhookProviderRegistry
	Dedupe    *DedupeCache
}

// ServeHTTP handles incoming webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	namespace, name, err := parsePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	logger := log.FromContext(ctx).WithValues("namespace", namespace, "name", name)

	// Fetch the RepositoryWebhook object.
	rwh := &declarestv1alpha1.RepositoryWebhook{}
	if err := h.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, rwh); err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, "webhook not found", http.StatusNotFound)
			return
		}
		logger.Error(err, "failed to get RepositoryWebhook")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if rwh.Spec.Suspend {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("suspended"))
		return
	}

	// Look up provider adapter.
	provider, ok := h.Providers[rwh.Spec.Provider]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	// Read and limit body.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadBytes+1))
	if err != nil {
		http.Error(w, "failed to read payload", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > maxPayloadBytes {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Read the webhook secret.
	secret := &corev1.Secret{}
	if err := h.Client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      rwh.Spec.SecretRef.Name,
	}, secret); err != nil {
		logger.Error(err, "failed to get webhook secret")
		http.Error(w, "failed to resolve webhook secret", http.StatusInternalServerError)
		return
	}
	secretValue := string(secret.Data["token"])
	if secretValue == "" {
		secretValue = string(secret.Data["webhook-secret"])
	}
	if secretValue == "" {
		http.Error(w, "webhook secret is empty", http.StatusInternalServerError)
		return
	}

	// Verify signature.
	if err := provider.VerifySignature(r, body, secretValue); err != nil {
		logger.Info("signature verification failed", "provider", rwh.Spec.Provider, "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse event.
	evt, err := provider.ParseEvent(r, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logger = logger.WithValues(
		"provider", evt.Provider,
		"event", evt.EventType,
		"deliveryID", evt.DeliveryID,
		"ref", evt.Ref,
	)

	// Deduplicate.
	if h.Dedupe != nil && h.Dedupe.IsDuplicate(evt.DeliveryID) {
		logger.Info("duplicate delivery, ignoring")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("duplicate"))
		return
	}

	// Handle ping events: update status but don't reconcile.
	if evt.IsPing {
		logger.Info("ping event received")
		h.updateWebhookStatus(ctx, rwh, evt)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("pong"))
		return
	}

	// Only process push events.
	if !evt.IsPush {
		logger.Info("ignoring non-push event")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ignored"))
		return
	}

	// Apply branch filter.
	branch := RefToBranch(evt.Ref)
	if !MatchBranchFilter(branch, rwh.Spec.BranchFilter) {
		logger.Info("branch filtered out", "branch", branch)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("filtered"))
		return
	}

	// Enqueue reconciliation by patching the referenced ResourceRepository.
	repo := &declarestv1alpha1.ResourceRepository{}
	repoKey := types.NamespacedName{
		Namespace: namespace,
		Name:      rwh.Spec.RepositoryRef.Name,
	}
	if err := h.Client.Get(ctx, repoKey, repo); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("referenced ResourceRepository not found")
			http.Error(w, "repository not found", http.StatusNotFound)
			return
		}
		logger.Error(err, "failed to get ResourceRepository")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Patch annotation to trigger reconcile.
	annotations := map[string]string{
		"declarest.io/webhook-last-received-at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if evt.DeliveryID != "" {
		annotations["declarest.io/webhook-last-event-id"] = evt.DeliveryID
	}
	patchData, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{"annotations": annotations},
	})
	if err := h.Client.Patch(ctx, repo, client.RawPatch(types.MergePatchType, patchData)); err != nil {
		logger.Error(err, "failed to patch ResourceRepository")
		http.Error(w, "failed to enqueue reconciliation", http.StatusInternalServerError)
		return
	}

	// Update RepositoryWebhook status.
	h.updateWebhookStatus(ctx, rwh, evt)

	logger.Info("webhook accepted, reconciliation enqueued", "repository", repoKey.Name, "branch", branch)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("accepted"))
}

func (h *Handler) updateWebhookStatus(ctx context.Context, rwh *declarestv1alpha1.RepositoryWebhook, evt WebhookEvent) {
	now := metav1.Now()
	rwh.Status.LastEventTime = &now
	if evt.DeliveryID != "" {
		rwh.Status.LastDeliveryID = evt.DeliveryID
	}
	if err := h.Client.Status().Update(ctx, rwh); err != nil {
		log.FromContext(ctx).Error(err, "failed to update RepositoryWebhook status after event")
	}
}

func parsePath(rawPath string) (string, string, error) {
	if !strings.HasPrefix(rawPath, hookPathPrefix) {
		return "", "", fmt.Errorf("invalid webhook path")
	}
	trimmed := strings.Trim(strings.TrimPrefix(rawPath, hookPathPrefix), "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("webhook path must include namespace and name")
	}
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("webhook path must be %s<namespace>/<name>", hookPathPrefix)
	}
	namespace := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if namespace == "" || name == "" {
		return "", "", fmt.Errorf("namespace and name are required")
	}
	return namespace, name, nil
}
