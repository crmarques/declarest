package v1alpha1

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespacedObjectReference struct {
	Name string `json:"name"`
}

type DeclaRESTExternalArtifact struct {
	URL string `json:"url,omitempty"`
}

type DeclaRESTMetadataArtifact struct {
	URL    string `json:"url,omitempty"`
	Bundle string `json:"bundle,omitempty"`
}

type HTTPProxySpec struct {
	HTTPURL  string         `json:"httpURL,omitempty"`
	HTTPSURL string         `json:"httpsURL,omitempty"`
	NoProxy  string         `json:"noProxy,omitempty"`
	Auth     *ProxyAuthSpec `json:"auth,omitempty"`
}

type ProxyAuthSpec struct {
	UsernameRef *corev1.SecretKeySelector `json:"usernameRef,omitempty"`
	PasswordRef *corev1.SecretKeySelector `json:"passwordRef,omitempty"`
}

type TLSSpec struct {
	CACertRef          *corev1.SecretKeySelector `json:"caCertRef,omitempty"`
	ClientCertRef      *corev1.SecretKeySelector `json:"clientCertRef,omitempty"`
	ClientKeyRef       *corev1.SecretKeySelector `json:"clientKeyRef,omitempty"`
	InsecureSkipVerify bool                      `json:"insecureSkipVerify,omitempty"`
}

type StorageSpec struct {
	ExistingPVC *corev1.LocalObjectReference `json:"existingPVC,omitempty"`
	PVC         *PVCTemplateSpec             `json:"pvc,omitempty"`
}

type PVCTemplateSpec struct {
	StorageClassName *string                             `json:"storageClassName,omitempty"`
	AccessModes      []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	Requests         corev1.ResourceList                 `json:"requests"`
}

func (s StorageSpec) validate(fieldPath string) error {
	hasExisting := s.ExistingPVC != nil && strings.TrimSpace(s.ExistingPVC.Name) != ""
	hasTemplate := s.PVC != nil
	if hasExisting == hasTemplate {
		return fmt.Errorf("%s must define exactly one of existingPVC or pvc", fieldPath)
	}
	if s.ExistingPVC != nil && strings.TrimSpace(s.ExistingPVC.Name) == "" {
		return fmt.Errorf("%s.existingPVC.name is required", fieldPath)
	}
	if s.PVC != nil {
		if len(s.PVC.Requests) == 0 {
			return fmt.Errorf("%s.pvc.requests is required", fieldPath)
		}
		if len(s.PVC.AccessModes) == 0 {
			return fmt.Errorf("%s.pvc.accessModes is required", fieldPath)
		}
	}
	return nil
}

func validateSecretRef(ref *corev1.SecretKeySelector, fieldPath string) error {
	if ref == nil {
		return fmt.Errorf("%s is required", fieldPath)
	}
	if strings.TrimSpace(ref.Name) == "" {
		return fmt.Errorf("%s.name is required", fieldPath)
	}
	if strings.TrimSpace(ref.Key) == "" {
		return fmt.Errorf("%s.key is required", fieldPath)
	}
	return nil
}

func validateHTTPURL(raw string, fieldPath string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("%s is required", fieldPath)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", fieldPath, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https scheme", fieldPath)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("%s host is required", fieldPath)
	}
	return nil
}

func validateGitURL(raw string, fieldPath string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("%s is required", fieldPath)
	}
	if strings.HasPrefix(value, "ssh://") || strings.HasPrefix(value, "git@") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid git URL: %w", fieldPath, err)
	}
	switch parsed.Scheme {
	case "https", "http":
	default:
		return fmt.Errorf("%s must use http, https, or ssh", fieldPath)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("%s host is required", fieldPath)
	}
	return nil
}

func normalizePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	normalized := path.Clean(trimmed)
	if normalized == "." {
		normalized = "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("path must normalize to absolute path")
	}
	return normalized, nil
}

func SetCondition(conditions []metav1.Condition, condition metav1.Condition) []metav1.Condition {
	found := false
	for idx := range conditions {
		if conditions[idx].Type == condition.Type {
			conditions[idx] = condition
			found = true
			break
		}
	}
	if !found {
		conditions = append(conditions, condition)
	}
	return conditions
}
