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
