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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetadataBundleSource identifies where the operator fetches a metadata
// bundle from. Exactly one of url, pvc, or configMap must be set. The
// pullSecretRef sibling only takes effect when url carries an `oci://` scheme.
// +kubebuilder:validation:XValidation:rule="(has(self.url) ? 1 : 0) + (has(self.pvc) ? 1 : 0) + (has(self.configMap) ? 1 : 0) == 1",message="source must define exactly one of url, pvc, or configMap"
type MetadataBundleSource struct {
	// URL is a scheme-dispatched reference to the bundle archive. Supported
	// schemes:
	//   oci://<registry>/<repo>:<tag>            pulled via ORAS; auth via pullSecretRef
	//   oci://<registry>/<repo>@sha256:<hex>     pinned by digest
	//   https:// | http://                       HTTP(S) tarball download
	//   file:///absolute/path                    controller-pod filesystem (tarball or directory)
	//   <name>:<version>                         GitHub release shorthand (legacy; accepted as-is)
	URL string `json:"url,omitempty"`

	// PullSecretRef names a Secret of type kubernetes.io/dockerconfigjson in
	// the same namespace as the MetadataBundle. Consulted only when URL uses
	// the oci:// scheme. Rotation of the referenced Secret triggers a
	// reconcile.
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`

	// PVC-backed source. Path may point at a .tar.gz archive or at an
	// unpacked directory containing bundle.yaml at its root.
	PVC *MetadataBundlePVCSource `json:"pvc,omitempty"`

	// ConfigMap-backed source. The referenced ConfigMap key carries a
	// gzipped tarball (binaryData preferred; data is base64-decoded).
	ConfigMap *MetadataBundleConfigMapSource `json:"configMap,omitempty"`
}

type MetadataBundlePVCSource struct {
	Storage StorageSpec `json:"storage"`
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`
}

type MetadataBundleConfigMapSource struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	// Key inside binaryData (preferred) or data (base64) holding the
	// gzipped tarball bytes.
	Key string `json:"key"`
}

type MetadataBundleSpec struct {
	Source       MetadataBundleSource `json:"source"`
	PollInterval *metav1.Duration     `json:"pollInterval,omitempty"`
}

type MetadataBundleCompatibility struct {
	Product  string `json:"product,omitempty"`
	Versions string `json:"versions,omitempty"`
}

type MetadataBundleManifest struct {
	Name                     string                       `json:"name,omitempty"`
	Version                  string                       `json:"version,omitempty"`
	Description              string                       `json:"description,omitempty"`
	MetadataRoot             string                       `json:"metadataRoot,omitempty"`
	OpenAPI                  string                       `json:"openapi,omitempty"`
	CompatibleDeclarest      string                       `json:"compatibleDeclarest,omitempty"`
	CompatibleManagedService *MetadataBundleCompatibility `json:"compatibleManagedService,omitempty"`
	Deprecated               bool                         `json:"deprecated,omitempty"`
}

type MetadataBundleStatus struct {
	ObservedGeneration int64                   `json:"observedGeneration,omitempty"`
	Manifest           *MetadataBundleManifest `json:"manifest,omitempty"`
	CachePath          string                  `json:"cachePath,omitempty"`
	OpenAPIPath        string                  `json:"openAPIPath,omitempty"`
	LastResolvedTime   *metav1.Time            `json:"lastResolvedTime,omitempty"`
	Conditions         []metav1.Condition      `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mb,categories=declarest
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.manifest.version"
// +kubebuilder:printcolumn:name="Product",type="string",JSONPath=".status.manifest.compatibleManagedService.product"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MetadataBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetadataBundleSpec   `json:"spec,omitempty"`
	Status MetadataBundleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type MetadataBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetadataBundle `json:"items"`
}

const defaultMetadataBundlePollInterval = time.Hour

func (b *MetadataBundle) Default() {
	if b == nil {
		return
	}
	if b.Spec.PollInterval == nil {
		b.Spec.PollInterval = &metav1.Duration{Duration: defaultMetadataBundlePollInterval}
	}
}

func (b *MetadataBundle) ValidateSpec() error {
	if b == nil {
		return fmt.Errorf("metadata bundle is required")
	}
	return validateMetadataBundleSpec(&b.Spec)
}

func validateMetadataBundleSpec(spec *MetadataBundleSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is required")
	}
	hasURL := strings.TrimSpace(spec.Source.URL) != ""
	hasPVC := spec.Source.PVC != nil
	hasConfigMap := spec.Source.ConfigMap != nil
	if countTrue(hasURL, hasPVC, hasConfigMap) != 1 {
		return fmt.Errorf("spec.source must define exactly one of url, pvc, or configMap")
	}
	if hasURL {
		if err := validateMetadataBundleURL(spec.Source.URL); err != nil {
			return err
		}
	}
	if spec.Source.PullSecretRef != nil {
		if !hasURL || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(spec.Source.URL)), "oci://") {
			return fmt.Errorf("spec.source.pullSecretRef is only valid with an oci:// url")
		}
		if strings.TrimSpace(spec.Source.PullSecretRef.Name) == "" {
			return fmt.Errorf("spec.source.pullSecretRef.name is required")
		}
	}
	if hasPVC {
		pvc := spec.Source.PVC
		if err := pvc.Storage.validate("spec.source.pvc.storage"); err != nil {
			return err
		}
		if strings.TrimSpace(pvc.Path) == "" {
			return fmt.Errorf("spec.source.pvc.path is required")
		}
		if strings.HasPrefix(pvc.Path, "/") {
			return fmt.Errorf("spec.source.pvc.path must be relative to storage root")
		}
		if strings.Contains(pvc.Path, "..") {
			return fmt.Errorf("spec.source.pvc.path must not traverse parents")
		}
	}
	if hasConfigMap {
		cm := spec.Source.ConfigMap
		if strings.TrimSpace(cm.Name) == "" {
			return fmt.Errorf("spec.source.configMap.name is required")
		}
		if strings.TrimSpace(cm.Key) == "" {
			return fmt.Errorf("spec.source.configMap.key is required")
		}
	}
	if spec.PollInterval != nil && spec.PollInterval.Duration < 0 {
		return fmt.Errorf("spec.pollInterval must be non-negative")
	}
	return nil
}

// validateMetadataBundleURL accepts any URL scheme supported by the resolver:
// oci://, https://, http://, file:///, or the legacy GitHub release shorthand
// `<name>:<version>`. Deep per-scheme validation happens inside the provider
// resolver during reconcile; admission only rejects obvious mistakes so the
// user gets a fast signal.
func validateMetadataBundleURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("spec.source.url is required")
	}
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "oci://"):
		if strings.TrimSpace(value[len("oci://"):]) == "" {
			return fmt.Errorf("spec.source.url oci reference is empty")
		}
		return nil
	case strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "http://"):
		return validateHTTPURL(value, "spec.source.url")
	case strings.HasPrefix(lower, "file://"):
		if strings.TrimSpace(value[len("file://"):]) == "" {
			return fmt.Errorf("spec.source.url file path is empty")
		}
		return nil
	}
	if strings.Contains(value, ":") && !strings.Contains(value, "/") {
		return nil
	}
	return fmt.Errorf("spec.source.url must use oci://, https://, http://, file://, or <name>:<version> form")
}
