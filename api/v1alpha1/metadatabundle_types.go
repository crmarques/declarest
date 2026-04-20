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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:XValidation:rule="(has(self.shorthand) ? 1 : 0) + (has(self.url) ? 1 : 0) + (has(self.file) ? 1 : 0) == 1",message="source must define exactly one of shorthand, url, or file"
type MetadataBundleSource struct {
	Shorthand string                    `json:"shorthand,omitempty"`
	URL       string                    `json:"url,omitempty"`
	File      *MetadataBundleFileSource `json:"file,omitempty"`
}

type MetadataBundleFileSource struct {
	Storage StorageSpec `json:"storage"`
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`
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
	hasShorthand := strings.TrimSpace(spec.Source.Shorthand) != ""
	hasURL := strings.TrimSpace(spec.Source.URL) != ""
	hasFile := spec.Source.File != nil
	if countTrue(hasShorthand, hasURL, hasFile) != 1 {
		return fmt.Errorf("spec.source must define exactly one of shorthand, url, or file")
	}
	if hasURL {
		if err := validateHTTPURL(spec.Source.URL, "spec.source.url"); err != nil {
			return err
		}
	}
	if hasFile {
		file := spec.Source.File
		if err := file.Storage.validate("spec.source.file.storage"); err != nil {
			return err
		}
		if strings.TrimSpace(file.Path) == "" {
			return fmt.Errorf("spec.source.file.path is required")
		}
		if strings.HasPrefix(file.Path, "/") {
			return fmt.Errorf("spec.source.file.path must be relative to storage root")
		}
		if strings.Contains(file.Path, "..") {
			return fmt.Errorf("spec.source.file.path must not traverse parents")
		}
	}
	if hasShorthand {
		value := strings.TrimSpace(spec.Source.Shorthand)
		if !strings.Contains(value, ":") {
			return fmt.Errorf("spec.source.shorthand must be in the form name:version")
		}
	}
	if spec.PollInterval != nil && spec.PollInterval.Duration < 0 {
		return fmt.Errorf("spec.pollInterval must be non-negative")
	}
	return nil
}
