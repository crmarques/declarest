package controllers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	repositoryWebhookPathPrefix            = "/webhooks/repository/"
	repositoryWebhookAnnotationLastEventAt = "declarest.io/webhook-last-received-at"
	repositoryWebhookAnnotationLastEventID = "declarest.io/webhook-last-event-id"
	defaultWebhookBodyLimit                = int64(1 << 20)
)

type RepositoryWebhookServer struct {
	Client          client.Client
	Recorder        record.EventRecorder
	BindAddress     string
	WatchNamespace  string
	MaxBodyBytes    int64
	ReadHeaderLimit time.Duration
}

func (s *RepositoryWebhookServer) Start(ctx context.Context) error {
	if s == nil || s.Client == nil {
		return nil
	}
	addr := strings.TrimSpace(s.BindAddress)
	if addr == "" {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc(repositoryWebhookPathPrefix, s.handleRepositoryWebhook)

	readHeaderTimeout := s.ReadHeaderLimit
	if readHeaderTimeout <= 0 {
		readHeaderTimeout = 5 * time.Second
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *RepositoryWebhookServer) NeedLeaderElection() bool {
	return false
}

func (s *RepositoryWebhookServer) handleRepositoryWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	namespace, repositoryName, err := s.parseRepositoryWebhookTarget(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	repo := &declarestv1alpha1.ResourceRepository{}
	if err := s.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: repositoryName}, repo); err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, "repository not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to resolve repository", http.StatusInternalServerError)
		return
	}
	if repo.Spec.Git == nil || repo.Spec.Git.Webhook == nil {
		http.Error(w, "repository webhook is not configured", http.StatusNotFound)
		return
	}

	maxBody := s.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = defaultWebhookBodyLimit
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		http.Error(w, "failed to read webhook payload", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > maxBody {
		http.Error(w, "webhook payload is too large", http.StatusRequestEntityTooLarge)
		return
	}

	secret, err := readSecretValueFromClient(ctx, s.Client, namespace, repo.Spec.Git.Webhook.SecretRef)
	if err != nil {
		http.Error(w, "failed to resolve webhook secret", http.StatusInternalServerError)
		return
	}

	authErr := validateRepositoryWebhookAuth(r, repo.Spec.Git.Webhook.Provider, secret, body)
	if authErr != nil {
		http.Error(w, authErr.Error(), http.StatusUnauthorized)
		return
	}

	if !isPushWebhookEvent(r, repo.Spec.Git.Webhook.Provider) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ignored non-push event"))
		return
	}

	branchMatches, branchErr := webhookBranchMatches(body, strings.TrimSpace(repo.Spec.Git.Branch))
	if branchErr != nil {
		http.Error(w, branchErr.Error(), http.StatusBadRequest)
		return
	}
	if !branchMatches {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ignored branch"))
		return
	}

	if err := s.patchRepositoryWebhookReceipt(ctx, repo, webhookEventID(r, repo.Spec.Git.Webhook.Provider)); err != nil {
		log.FromContext(ctx).Error(err, "failed to patch repository after webhook", "repository", repo.Name, "namespace", repo.Namespace)
		http.Error(w, "failed to record webhook event", http.StatusInternalServerError)
		return
	}

	emitEventf(
		s.Recorder,
		repo,
		corev1.EventTypeNormal,
		"WebhookReceived",
		"repository webhook received provider=%s",
		repo.Spec.Git.Webhook.Provider,
	)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("accepted"))
}

func (s *RepositoryWebhookServer) parseRepositoryWebhookTarget(rawPath string) (string, string, error) {
	if !strings.HasPrefix(rawPath, repositoryWebhookPathPrefix) {
		return "", "", fmt.Errorf("invalid webhook path")
	}
	trimmed := strings.Trim(strings.TrimPrefix(rawPath, repositoryWebhookPathPrefix), "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("repository webhook path must include namespace and repository name")
	}

	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		namespace := strings.TrimSpace(s.WatchNamespace)
		if namespace == "" {
			return "", "", fmt.Errorf("repository webhook path must include namespace")
		}
		repoName := strings.TrimSpace(parts[0])
		if errs := validation.IsDNS1123Subdomain(repoName); len(errs) > 0 {
			return "", "", fmt.Errorf("invalid repository name")
		}
		return namespace, repoName, nil
	case 2:
		namespace := strings.TrimSpace(parts[0])
		repoName := strings.TrimSpace(parts[1])
		if namespace == "" || repoName == "" {
			return "", "", fmt.Errorf("repository webhook path must include namespace and repository name")
		}
		if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
			return "", "", fmt.Errorf("invalid namespace")
		}
		if errs := validation.IsDNS1123Subdomain(repoName); len(errs) > 0 {
			return "", "", fmt.Errorf("invalid repository name")
		}
		if watchNamespace := strings.TrimSpace(s.WatchNamespace); watchNamespace != "" && namespace != watchNamespace {
			return "", "", fmt.Errorf("repository namespace is not watched by this operator")
		}
		return namespace, repoName, nil
	default:
		return "", "", fmt.Errorf("repository webhook path must be /webhooks/repository/<namespace>/<name>")
	}
}

func validateRepositoryWebhookAuth(
	req *http.Request,
	provider declarestv1alpha1.GitWebhookProvider,
	secret string,
	body []byte,
) error {
	switch provider {
	case declarestv1alpha1.GitWebhookProviderGitea:
		signature := strings.TrimSpace(req.Header.Get("X-Gitea-Signature"))
		if signature == "" {
			return fmt.Errorf("missing gitea signature header")
		}
		expected := hmac.New(sha256.New, []byte(secret))
		_, _ = expected.Write(body)
		expectedSum := expected.Sum(nil)

		provided, err := hex.DecodeString(signature)
		if err != nil {
			return fmt.Errorf("invalid gitea signature format")
		}
		if subtle.ConstantTimeCompare(provided, expectedSum) != 1 {
			return fmt.Errorf("invalid gitea signature")
		}
		return nil
	case declarestv1alpha1.GitWebhookProviderGitLab:
		token := strings.TrimSpace(req.Header.Get("X-Gitlab-Token"))
		if token == "" {
			return fmt.Errorf("missing gitlab token header")
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
			return fmt.Errorf("invalid gitlab token")
		}
		return nil
	default:
		return fmt.Errorf("unsupported webhook provider")
	}
}

func isPushWebhookEvent(req *http.Request, provider declarestv1alpha1.GitWebhookProvider) bool {
	switch provider {
	case declarestv1alpha1.GitWebhookProviderGitea:
		return strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Gitea-Event")), "push")
	case declarestv1alpha1.GitWebhookProviderGitLab:
		return strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Gitlab-Event")), "Push Hook")
	default:
		return false
	}
}

func webhookEventID(req *http.Request, provider declarestv1alpha1.GitWebhookProvider) string {
	switch provider {
	case declarestv1alpha1.GitWebhookProviderGitea:
		return strings.TrimSpace(req.Header.Get("X-Gitea-Delivery"))
	case declarestv1alpha1.GitWebhookProviderGitLab:
		return strings.TrimSpace(req.Header.Get("X-Gitlab-Event-UUID"))
	default:
		return ""
	}
}

func webhookBranchMatches(body []byte, branch string) (bool, error) {
	if strings.TrimSpace(branch) == "" {
		return true, nil
	}
	var payload struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, fmt.Errorf("invalid webhook payload")
	}
	if strings.TrimSpace(payload.Ref) == "" {
		return false, nil
	}
	ref := strings.TrimSpace(payload.Ref)
	expected := fmt.Sprintf("refs/heads/%s", strings.TrimSpace(branch))
	return ref == expected || ref == branch, nil
}

func (s *RepositoryWebhookServer) patchRepositoryWebhookReceipt(
	ctx context.Context,
	repo *declarestv1alpha1.ResourceRepository,
	eventID string,
) error {
	annotations := map[string]string{
		repositoryWebhookAnnotationLastEventAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if eventID != "" {
		annotations[repositoryWebhookAnnotationLastEventID] = eventID
	}
	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}
	raw, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	return s.Client.Patch(ctx, repo, client.RawPatch(types.MergePatchType, raw))
}
