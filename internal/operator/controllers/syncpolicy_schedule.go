package controllers

import (
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/internal/cronexpr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultSyncPolicyRequeueInterval = 5 * time.Minute

func shouldRunFullResync(cronRaw string, lastFullResyncTime *metav1.Time, currentTime time.Time) (bool, error) {
	value := strings.TrimSpace(cronRaw)
	if value == "" {
		return false, nil
	}

	schedule, err := cronexpr.Parse(value)
	if err != nil {
		return false, err
	}
	if lastFullResyncTime == nil {
		return true, nil
	}

	next, ok := schedule.Next(lastFullResyncTime.Time.UTC())
	if !ok {
		return false, nil
	}
	return !currentTime.UTC().Before(next), nil
}

func syncPolicyRequeueAfter(syncPolicy *declarestv1alpha1.SyncPolicy, currentTime time.Time) time.Duration {
	interval := syncPolicy.Spec.SyncInterval.Duration
	if interval <= 0 {
		interval = defaultSyncPolicyRequeueInterval
	}

	cronRaw := strings.TrimSpace(syncPolicy.Spec.FullResyncCron)
	if cronRaw == "" {
		return interval
	}

	schedule, err := cronexpr.Parse(cronRaw)
	if err != nil {
		return interval
	}
	next, ok := schedule.Next(currentTime.UTC())
	if !ok {
		return interval
	}

	cronInterval := next.Sub(currentTime.UTC())
	if cronInterval <= 0 {
		cronInterval = time.Second
	}
	if cronInterval < interval {
		return cronInterval
	}
	return interval
}
