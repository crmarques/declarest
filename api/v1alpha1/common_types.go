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

package v1alpha1

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/crmarques/declarest/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespacedObjectReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type DeclaRESTExternalArtifact struct {
	URL string `json:"url,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!(has(self.url) && size(self.url) > 0 && has(self.bundleRef))",message="metadata must define at most one of url or bundleRef"
type DeclaRESTMetadataArtifact struct {
	URL       string                     `json:"url,omitempty"`
	BundleRef *NamespacedObjectReference `json:"bundleRef,omitempty"`
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

// +kubebuilder:validation:XValidation:rule="(has(self.existingPVC) ? 1 : 0) + (has(self.pvc) ? 1 : 0) == 1",message="storage must define exactly one of existingPVC or pvc"
type StorageSpec struct {
	ExistingPVC *corev1.LocalObjectReference `json:"existingPVC,omitempty"`
	PVC         *PVCTemplateSpec             `json:"pvc,omitempty"`
}

type PVCTemplateSpec struct {
	StorageClassName *string `json:"storageClassName,omitempty"`
	// +kubebuilder:validation:MinItems=1
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`
	Requests    corev1.ResourceList                 `json:"requests"`
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
	parsedPath, err := resource.ParseRawPathWithOptions(raw, resource.RawPathParseOptions{
		AllowMissingLeadingSlash: true,
	})
	if err != nil {
		if strings.TrimSpace(raw) == "" {
			return "", fmt.Errorf("path is required")
		}
		return "", err
	}

	normalized, err := resource.NormalizeLogicalPath(parsedPath.Normalized)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// HasPathOverlap returns true if two logical paths overlap (one is a prefix
// of the other or they are equal).
func HasPathOverlap(a string, b string) bool {
	left := NormalizeOverlapPath(a)
	right := NormalizeOverlapPath(b)
	if left == "" || right == "" {
		return false
	}
	overlap, err := resource.HasLogicalPathOverlap(left, right)
	if err != nil {
		return false
	}
	return overlap
}

// NormalizeOverlapPath normalizes a logical path for overlap comparison.
func NormalizeOverlapPath(raw string) string {
	normalized, err := normalizePath(raw)
	if err != nil {
		return ""
	}
	return normalized
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
