package controllers

import (
	"testing"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManagedServerPollIntervalFallsBackToDefault(t *testing.T) {
	t.Parallel()

	server := &declarestv1alpha1.ManagedServer{}

	if got := managedServerPollInterval(server); got != 10*time.Minute {
		t.Fatalf("expected default poll interval 10m, got %v", got)
	}
}

func TestManagedServerPollIntervalUsesConfiguredValue(t *testing.T) {
	t.Parallel()

	server := &declarestv1alpha1.ManagedServer{
		Spec: declarestv1alpha1.ManagedServerSpec{
			PollInterval: &metav1.Duration{Duration: 45 * time.Second},
		},
	}

	if got := managedServerPollInterval(server); got != 45*time.Second {
		t.Fatalf("expected configured poll interval 45s, got %v", got)
	}
}
