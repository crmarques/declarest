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

type ManagedServerAuth struct {
	OAuth2        *ManagedServerOAuth2Auth  `json:"oauth2,omitempty"`
	BasicAuth     *ManagedServerBasicAuth   `json:"basicAuth,omitempty"`
	CustomHeaders []ManagedServerHeaderAuth `json:"customHeaders,omitempty"`
}

type ManagedServerOAuth2Auth struct {
	TokenURL        string                    `json:"tokenURL"`
	GrantType       string                    `json:"grantType,omitempty"`
	ClientIDRef     *corev1.SecretKeySelector `json:"clientIDRef,omitempty"`
	ClientSecretRef *corev1.SecretKeySelector `json:"clientSecretRef,omitempty"`
	UsernameRef     *corev1.SecretKeySelector `json:"usernameRef,omitempty"`
	PasswordRef     *corev1.SecretKeySelector `json:"passwordRef,omitempty"`
	Scope           string                    `json:"scope,omitempty"`
	Audience        string                    `json:"audience,omitempty"`
}

type ManagedServerBasicAuth struct {
	UsernameRef *corev1.SecretKeySelector `json:"usernameRef,omitempty"`
	PasswordRef *corev1.SecretKeySelector `json:"passwordRef,omitempty"`
}

type ManagedServerHeaderAuth struct {
	Header   string                    `json:"header"`
	Prefix   string                    `json:"prefix,omitempty"`
	ValueRef *corev1.SecretKeySelector `json:"valueRef,omitempty"`
}

type ManagedServerRequestThrottling struct {
	// +kubebuilder:validation:Minimum=0
	MaxConcurrentRequests int32 `json:"maxConcurrentRequests,omitempty"`
	// +kubebuilder:validation:Minimum=0
	QueueSize int32 `json:"queueSize,omitempty"`
	// +kubebuilder:validation:Minimum=0
	RequestsPerSecond int32 `json:"requestsPerSecond,omitempty"`
	// +kubebuilder:validation:Minimum=0
	Burst int32 `json:"burst,omitempty"`
}

type ManagedServerHTTP struct {
	BaseURL           string                          `json:"baseURL"`
	HealthCheck       string                          `json:"healthCheck,omitempty"`
	DefaultHeaders    map[string]string               `json:"defaultHeaders,omitempty"`
	Auth              ManagedServerAuth               `json:"auth"`
	TLS               *TLSSpec                        `json:"tls,omitempty"`
	Proxy             *HTTPProxySpec                  `json:"proxy,omitempty"`
	RequestThrottling *ManagedServerRequestThrottling `json:"requestThrottling,omitempty"`
}

type ManagedServerSpec struct {
	HTTP         ManagedServerHTTP         `json:"http"`
	OpenAPI      DeclaRESTExternalArtifact `json:"openapi,omitempty"`
	Metadata     DeclaRESTMetadataArtifact `json:"metadata,omitempty"`
	PollInterval *metav1.Duration          `json:"pollInterval,omitempty"`
}

type ManagedServerStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	OpenAPICachePath   string             `json:"openapiCachePath,omitempty"`
	MetadataCachePath  string             `json:"metadataCachePath,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ms
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
type ManagedServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedServerSpec   `json:"spec,omitempty"`
	Status ManagedServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ManagedServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedServer `json:"items"`
}

const defaultManagedServerPollInterval = 10 * time.Minute

func (m *ManagedServer) Default() {
	if m.Spec.HTTP.Auth.OAuth2 != nil && strings.TrimSpace(m.Spec.HTTP.Auth.OAuth2.GrantType) == "" {
		m.Spec.HTTP.Auth.OAuth2.GrantType = "client_credentials"
	}
	if m.Spec.PollInterval == nil {
		m.Spec.PollInterval = &metav1.Duration{Duration: defaultManagedServerPollInterval}
	}
}

func (m *ManagedServer) ValidateSpec() error {
	if m == nil {
		return fmt.Errorf("managed server is required")
	}
	return validateManagedServerSpec(&m.Spec)
}

func validateManagedServerSpec(spec *ManagedServerSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is required")
	}

	if err := validateHTTPURL(spec.HTTP.BaseURL, "spec.http.baseURL"); err != nil {
		return err
	}
	hasOAuth2 := spec.HTTP.Auth.OAuth2 != nil
	hasBasic := spec.HTTP.Auth.BasicAuth != nil
	hasHeaders := len(spec.HTTP.Auth.CustomHeaders) > 0
	if countTrue(hasOAuth2, hasBasic, hasHeaders) != 1 {
		return fmt.Errorf("spec.http.auth must define exactly one of oauth2, basicAuth, or customHeaders")
	}
	if hasOAuth2 {
		oauth2 := spec.HTTP.Auth.OAuth2
		if err := validateHTTPURL(oauth2.TokenURL, "spec.http.auth.oauth2.tokenURL"); err != nil {
			return err
		}
		if strings.TrimSpace(oauth2.GrantType) == "" {
			return fmt.Errorf("spec.http.auth.oauth2.grantType is required")
		}
		if err := validateSecretRef(oauth2.ClientIDRef, "spec.http.auth.oauth2.clientIDRef"); err != nil {
			return err
		}
		if err := validateSecretRef(oauth2.ClientSecretRef, "spec.http.auth.oauth2.clientSecretRef"); err != nil {
			return err
		}
		if oauth2.UsernameRef != nil {
			if err := validateSecretRef(oauth2.UsernameRef, "spec.http.auth.oauth2.usernameRef"); err != nil {
				return err
			}
		}
		if oauth2.PasswordRef != nil {
			if err := validateSecretRef(oauth2.PasswordRef, "spec.http.auth.oauth2.passwordRef"); err != nil {
				return err
			}
		}
	}
	if hasBasic {
		if err := validateSecretRef(spec.HTTP.Auth.BasicAuth.UsernameRef, "spec.http.auth.basicAuth.usernameRef"); err != nil {
			return err
		}
		if err := validateSecretRef(spec.HTTP.Auth.BasicAuth.PasswordRef, "spec.http.auth.basicAuth.passwordRef"); err != nil {
			return err
		}
	}
	if hasHeaders {
		for idx, item := range spec.HTTP.Auth.CustomHeaders {
			if strings.TrimSpace(item.Header) == "" {
				return fmt.Errorf("spec.http.auth.customHeaders[%d].header is required", idx)
			}
			if err := validateSecretRef(item.ValueRef, fmt.Sprintf("spec.http.auth.customHeaders[%d].valueRef", idx)); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(spec.OpenAPI.URL) != "" {
		if err := validateHTTPURL(spec.OpenAPI.URL, "spec.openapi.url"); err != nil {
			return err
		}
	}
	metadataURL := strings.TrimSpace(spec.Metadata.URL)
	metadataBundle := strings.TrimSpace(spec.Metadata.Bundle)
	if metadataURL != "" && metadataBundle != "" {
		return fmt.Errorf("spec.metadata must define at most one of url or bundle")
	}
	if metadataURL != "" {
		if err := validateHTTPURL(metadataURL, "spec.metadata.url"); err != nil {
			return err
		}
	}
	if spec.HTTP.Proxy != nil {
		if spec.HTTP.Proxy.Auth != nil {
			if err := validateSecretRef(spec.HTTP.Proxy.Auth.UsernameRef, "spec.http.proxy.auth.usernameRef"); err != nil {
				return err
			}
			if err := validateSecretRef(spec.HTTP.Proxy.Auth.PasswordRef, "spec.http.proxy.auth.passwordRef"); err != nil {
				return err
			}
		}
	}
	if spec.HTTP.RequestThrottling != nil {
		throttling := spec.HTTP.RequestThrottling
		if throttling.MaxConcurrentRequests <= 0 && throttling.RequestsPerSecond <= 0 {
			return fmt.Errorf("spec.http.requestThrottling must define at least one of maxConcurrentRequests or requestsPerSecond")
		}
		if throttling.MaxConcurrentRequests < 0 {
			return fmt.Errorf("spec.http.requestThrottling.maxConcurrentRequests must be greater than zero when set")
		}
		if throttling.QueueSize < 0 {
			return fmt.Errorf("spec.http.requestThrottling.queueSize must be greater than or equal to zero")
		}
		if throttling.QueueSize > 0 && throttling.MaxConcurrentRequests <= 0 {
			return fmt.Errorf("spec.http.requestThrottling.queueSize requires maxConcurrentRequests to be greater than zero")
		}
		if throttling.RequestsPerSecond < 0 {
			return fmt.Errorf("spec.http.requestThrottling.requestsPerSecond must be greater than zero when set")
		}
		if throttling.Burst < 0 {
			return fmt.Errorf("spec.http.requestThrottling.burst must be greater than zero when set")
		}
		if throttling.Burst > 0 && throttling.RequestsPerSecond <= 0 {
			return fmt.Errorf("spec.http.requestThrottling.burst requires requestsPerSecond to be greater than zero")
		}
	}
	return nil
}

func countTrue(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
