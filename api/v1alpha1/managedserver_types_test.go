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
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestManagedServerValidateSpecAllowsRequestThrottling(t *testing.T) {
	t.Parallel()

	server := &ManagedServer{
		Spec: ManagedServerSpec{
			HTTP: ManagedServerHTTP{
				BaseURL: "https://managed-server.example.com",
				Auth: ManagedServerAuth{
					BasicAuth: &ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "username"},
						PasswordRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "password"},
					},
				},
				RequestThrottling: &ManagedServerRequestThrottling{
					MaxConcurrentRequests: 2,
					QueueSize:             4,
					RequestsPerSecond:     10,
					Burst:                 20,
				},
			},
		},
	}

	if err := server.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestManagedServerValidateSpecRejectsInvalidRequestThrottling(t *testing.T) {
	t.Parallel()

	server := &ManagedServer{
		Spec: ManagedServerSpec{
			HTTP: ManagedServerHTTP{
				BaseURL: "https://managed-server.example.com",
				Auth: ManagedServerAuth{
					BasicAuth: &ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "username"},
						PasswordRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "password"},
					},
				},
				RequestThrottling: &ManagedServerRequestThrottling{
					QueueSize: 2,
				},
			},
		},
	}

	if err := server.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected throttling validation error, got nil")
	}
}

func TestManagedServerValidateSpecAllowsMetadataBundle(t *testing.T) {
	t.Parallel()

	server := &ManagedServer{
		Spec: ManagedServerSpec{
			HTTP: ManagedServerHTTP{
				BaseURL: "https://managed-server.example.com",
				Auth: ManagedServerAuth{
					BasicAuth: &ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "username"},
						PasswordRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "password"},
					},
				},
			},
			Metadata: DeclaRESTMetadataArtifact{
				Bundle: "keycloak-bundle:0.0.1",
			},
		},
	}

	if err := server.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestManagedServerValidateSpecAllowsSparseProxyOverride(t *testing.T) {
	t.Parallel()

	server := &ManagedServer{
		Spec: ManagedServerSpec{
			HTTP: ManagedServerHTTP{
				BaseURL: "https://managed-server.example.com",
				Auth: ManagedServerAuth{
					BasicAuth: &ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "username"},
						PasswordRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "password"},
					},
				},
				Proxy: &HTTPProxySpec{
					NoProxy: "localhost,127.0.0.1",
				},
			},
		},
	}

	if err := server.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestManagedServerValidateSpecRejectsMetadataURLAndBundle(t *testing.T) {
	t.Parallel()

	server := &ManagedServer{
		Spec: ManagedServerSpec{
			HTTP: ManagedServerHTTP{
				BaseURL: "https://managed-server.example.com",
				Auth: ManagedServerAuth{
					BasicAuth: &ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "username"},
						PasswordRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "managed-server-auth"}, Key: "password"},
					},
				},
			},
			Metadata: DeclaRESTMetadataArtifact{
				URL:    "https://managed-server.example.com/metadata-bundle.tar.gz",
				Bundle: "keycloak-bundle:0.0.1",
			},
		},
	}

	if err := server.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected metadata source validation error, got nil")
	}
}
