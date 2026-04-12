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

	next, ok := schedule.Next(lastFullResyncTime.UTC())
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
