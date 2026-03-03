package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/faults"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	finalizerName = "declarest.io/cleanup"

	conditionReasonReady             = "Ready"
	conditionReasonSpecInvalid       = "SpecInvalid"
	conditionReasonDependencyInvalid = "DependencyInvalid"
	conditionReasonSuspended         = "Suspended"
	conditionReasonReconciling       = "Reconciling"
	conditionReasonReconcileFailed   = "ReconcileFailed"
	conditionReasonOverlappingPolicy = "OverlappingPolicy"
)

func now() metav1.Time {
	return metav1.NewTime(time.Now().UTC())
}

func setStatusCondition(
	conditions []metav1.Condition,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) []metav1.Condition {
	return declarestv1alpha1.SetCondition(
		conditions,
		metav1.Condition{
			Type:               conditionType,
			Status:             status,
			Reason:             strings.TrimSpace(reason),
			Message:            strings.TrimSpace(message),
			LastTransitionTime: now(),
		},
	)
}

func emitEventf(recorder record.EventRecorder, object runtime.Object, eventType string, reason string, messageFmt string, args ...any) {
	if recorder == nil || object == nil {
		return
	}
	recorder.Eventf(object, eventType, strings.TrimSpace(reason), messageFmt, args...)
}

func returnAfterSetNotReady(
	ctx context.Context,
	setNotReady func(context.Context, string, string) error,
	reason string,
	message string,
	requeueAfter time.Duration,
) (ctrl.Result, error) {
	if err := setNotReady(ctx, reason, message); err != nil {
		return ctrl.Result{}, err
	}
	if requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory %q: %w", path, err)
	}
	return nil
}

func resolveRepoRootPath(namespace string, name string) string {
	base := strings.TrimSpace(os.Getenv("DECLAREST_OPERATOR_REPO_BASE_DIR"))
	if base == "" {
		base = "/tmp/declarest-operator/repos"
	}
	return filepath.Join(base, namespace, name)
}

func resolveCacheRootPath(namespace string, name string) string {
	base := strings.TrimSpace(os.Getenv("DECLAREST_OPERATOR_CACHE_BASE_DIR"))
	if base == "" {
		base = "/tmp/declarest-operator/cache"
	}
	return filepath.Join(base, namespace, name)
}

func sanitizeURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	if parsed.User != nil {
		parsed.User = url.User("***")
	}
	return parsed.String()
}

func readSecretValueFromClient(ctx context.Context, reader client.Reader, namespace string, ref *corev1.SecretKeySelector) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("secret reference is nil")
	}
	secret := &corev1.Secret{}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: strings.TrimSpace(ref.Name)}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("secret %q not found", ref.Name), err)
		}
		return "", err
	}
	value, ok := secret.Data[strings.TrimSpace(ref.Key)]
	if !ok {
		return "", faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("secret key %q not found in %s/%s", ref.Key, namespace, ref.Name), nil)
	}
	if len(value) == 0 {
		return "", faults.NewValidationError(fmt.Sprintf("secret key %q in %s/%s is empty", ref.Key, namespace, ref.Name), nil)
	}
	return string(value), nil
}

// cleanupRegistry collects cleanup functions and runs them in reverse order.
type cleanupRegistry struct {
	fns []func()
}

func (c *cleanupRegistry) add(fn func()) {
	c.fns = append(c.fns, fn)
}

func (c *cleanupRegistry) run() {
	for i := len(c.fns) - 1; i >= 0; i-- {
		c.fns[i]()
	}
}

func writeSecretValueToFile(baseDir string, name string, value string) (string, error) {
	hash := sha256.Sum256([]byte(name))
	fileName := hex.EncodeToString(hash[:])
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return "", fmt.Errorf("create secure directory %q: %w", baseDir, err)
	}
	path := filepath.Join(baseDir, fileName)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func writeSecretValueToFileWithCleanup(registry *cleanupRegistry, baseDir string, name string, value string) (string, error) {
	path, err := writeSecretValueToFile(baseDir, name, value)
	if err != nil {
		return "", err
	}
	registry.add(func() {
		_ = os.Remove(path)
	})
	return path, nil
}

func hasPathOverlap(a string, b string) bool {
	left := normalizeOverlapPath(a)
	right := normalizeOverlapPath(b)
	if left == right {
		return true
	}
	if strings.HasPrefix(left, right) && boundaryMatch(left, right) {
		return true
	}
	if strings.HasPrefix(right, left) && boundaryMatch(right, left) {
		return true
	}
	return false
}

func normalizeOverlapPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." {
		value = "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if value != "/" {
		value = strings.TrimSuffix(value, "/")
	}
	return value
}

func boundaryMatch(candidate string, prefix string) bool {
	if prefix == "/" {
		return true
	}
	if len(candidate) <= len(prefix) {
		return false
	}
	return candidate[len(prefix)] == '/'
}

func stringSet(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := sets.New[string]()
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set.Insert(trimmed)
	}
	out := set.UnsortedList()
	sort.Strings(out)
	return out
}

func uuidString() string {
	return uuid.NewString()
}

// collectSecretNames returns the deduplicated, sorted set of Kubernetes Secret
// names referenced by the three dependency CRDs of a SyncPolicy.
func collectSecretNames(
	repo *declarestv1alpha1.ResourceRepository,
	managedServer *declarestv1alpha1.ManagedServer,
	secretStore *declarestv1alpha1.SecretStore,
) []string {
	names := sets.New[string]()
	addRef := func(ref *corev1.SecretKeySelector) {
		if ref != nil && strings.TrimSpace(ref.Name) != "" {
			names.Insert(strings.TrimSpace(ref.Name))
		}
	}
	// ResourceRepository secret refs
	if repo.Spec.Git != nil {
		addRef(repo.Spec.Git.Auth.TokenSecretRef)
		if repo.Spec.Git.Auth.SSHSecretRef != nil {
			addRef(repo.Spec.Git.Auth.SSHSecretRef.PrivateKeyRef)
			addRef(repo.Spec.Git.Auth.SSHSecretRef.KnownHostsRef)
			addRef(repo.Spec.Git.Auth.SSHSecretRef.PassphraseRef)
		}
	}
	// ManagedServer secret refs
	if managedServer.Spec.HTTP.Auth.OAuth2 != nil {
		addRef(managedServer.Spec.HTTP.Auth.OAuth2.ClientIDRef)
		addRef(managedServer.Spec.HTTP.Auth.OAuth2.ClientSecretRef)
		addRef(managedServer.Spec.HTTP.Auth.OAuth2.UsernameRef)
		addRef(managedServer.Spec.HTTP.Auth.OAuth2.PasswordRef)
	}
	if managedServer.Spec.HTTP.Auth.BasicAuth != nil {
		addRef(managedServer.Spec.HTTP.Auth.BasicAuth.UsernameRef)
		addRef(managedServer.Spec.HTTP.Auth.BasicAuth.PasswordRef)
	}
	for _, h := range managedServer.Spec.HTTP.Auth.CustomHeaders {
		addRef(h.ValueRef)
	}
	if managedServer.Spec.HTTP.TLS != nil {
		addRef(managedServer.Spec.HTTP.TLS.CACertRef)
		addRef(managedServer.Spec.HTTP.TLS.ClientCertRef)
		addRef(managedServer.Spec.HTTP.TLS.ClientKeyRef)
	}
	if managedServer.Spec.HTTP.Proxy != nil && managedServer.Spec.HTTP.Proxy.Auth != nil {
		addRef(managedServer.Spec.HTTP.Proxy.Auth.UsernameRef)
		addRef(managedServer.Spec.HTTP.Proxy.Auth.PasswordRef)
	}
	// SecretStore secret refs
	if secretStore.Spec.Vault != nil {
		addRef(secretStore.Spec.Vault.Auth.TokenRef)
		addRef(secretStore.Spec.Vault.Auth.UsernameRef)
		addRef(secretStore.Spec.Vault.Auth.PasswordRef)
		addRef(secretStore.Spec.Vault.Auth.AppRoleRoleIDRef)
		addRef(secretStore.Spec.Vault.Auth.AppRoleSecretIDRef)
		if secretStore.Spec.Vault.TLS != nil {
			addRef(secretStore.Spec.Vault.TLS.CACertRef)
			addRef(secretStore.Spec.Vault.TLS.ClientCertRef)
			addRef(secretStore.Spec.Vault.TLS.ClientKeyRef)
		}
		if secretStore.Spec.Vault.Proxy != nil && secretStore.Spec.Vault.Proxy.Auth != nil {
			addRef(secretStore.Spec.Vault.Proxy.Auth.UsernameRef)
			addRef(secretStore.Spec.Vault.Proxy.Auth.PasswordRef)
		}
	}
	if secretStore.Spec.File != nil {
		addRef(secretStore.Spec.File.Encryption.KeyRef)
		addRef(secretStore.Spec.File.Encryption.PassphraseRef)
	}
	out := names.UnsortedList()
	sort.Strings(out)
	return out
}

// computeSecretVersionHash fetches referenced Secrets and hashes their
// resourceVersion values. This detects credential rotation without reading
// actual secret data.
func computeSecretVersionHash(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	repo *declarestv1alpha1.ResourceRepository,
	managedServer *declarestv1alpha1.ManagedServer,
	secretStore *declarestv1alpha1.SecretStore,
) string {
	secretNames := collectSecretNames(repo, managedServer, secretStore)
	if len(secretNames) == 0 {
		return ""
	}
	h := sha256.New()
	for _, name := range secretNames {
		secret := &corev1.Secret{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
			// Include the error in the hash so a missing secret produces a
			// different hash than a present one, forcing re-reconciliation.
			h.Write([]byte(name + ":error:" + err.Error()))
			continue
		}
		h.Write([]byte(name + ":" + secret.ResourceVersion))
	}
	return hex.EncodeToString(h.Sum(nil))
}
