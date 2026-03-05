package controllers

import (
	"testing"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldRunFullResync(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 4, 10, 20, 0, 0, time.UTC)
	last := metav1.NewTime(time.Date(2026, time.March, 4, 10, 0, 0, 0, time.UTC))

	run, err := shouldRunFullResync("15 * * * *", &last, now)
	if err != nil {
		t.Fatalf("shouldRunFullResync() error = %v", err)
	}
	if !run {
		t.Fatal("expected full resync to be due")
	}
}

func TestShouldRunFullResyncWithNoPreviousRun(t *testing.T) {
	t.Parallel()

	run, err := shouldRunFullResync("0 * * * *", nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("shouldRunFullResync() error = %v", err)
	}
	if !run {
		t.Fatal("expected full resync when there is no previous full run")
	}
}

func TestSyncPolicyRequeueAfterUsesSoonestSchedule(t *testing.T) {
	t.Parallel()

	syncPolicy := &declarestv1alpha1.SyncPolicy{
		Spec: declarestv1alpha1.SyncPolicySpec{
			SyncInterval:   &metav1.Duration{Duration: 30 * time.Minute},
			FullResyncCron: "15 * * * *",
		},
	}

	current := time.Date(2026, time.March, 4, 10, 7, 0, 0, time.UTC)
	requeueAfter := syncPolicyRequeueAfter(syncPolicy, current)

	expected := 8 * time.Minute
	if requeueAfter != expected {
		t.Fatalf("expected requeueAfter %s, got %s", expected, requeueAfter)
	}
}

func TestSyncPolicyRequeueAfterFallsBackToSyncInterval(t *testing.T) {
	t.Parallel()

	syncPolicy := &declarestv1alpha1.SyncPolicy{
		Spec: declarestv1alpha1.SyncPolicySpec{
			SyncInterval: &metav1.Duration{Duration: 5 * time.Minute},
		},
	}

	requeueAfter := syncPolicyRequeueAfter(syncPolicy, time.Now().UTC())
	if requeueAfter != 5*time.Minute {
		t.Fatalf("expected sync interval fallback, got %s", requeueAfter)
	}
}
