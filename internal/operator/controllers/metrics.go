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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	resourceRepositoryPollTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "declarest",
			Subsystem: "operator",
			Name:      "resource_repository_poll_total",
			Help:      "Total number of resource repository poll attempts.",
		},
		[]string{"namespace", "name", "result"},
	)
	resourceRepositoryRevisionChangesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "declarest",
			Subsystem: "operator",
			Name:      "resource_repository_revision_changes_total",
			Help:      "Total number of observed repository revision changes.",
		},
		[]string{"namespace", "name"},
	)

	syncPolicyReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "declarest",
			Subsystem: "operator",
			Name:      "syncpolicy_reconcile_total",
			Help:      "Total number of SyncPolicy reconcile attempts.",
		},
		[]string{"namespace", "name", "result", "reason"},
	)
	syncPolicyReconcileDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "declarest",
			Subsystem: "operator",
			Name:      "syncpolicy_reconcile_duration_seconds",
			Help:      "Duration of SyncPolicy reconciliation in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"namespace", "name", "result"},
	)

	syncPolicyResourcesAppliedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "declarest",
			Subsystem: "operator",
			Name:      "syncpolicy_resources_applied_total",
			Help:      "Total number of resources applied by SyncPolicy reconciliations.",
		},
		[]string{"namespace", "name"},
	)
	syncPolicyResourcesPrunedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "declarest",
			Subsystem: "operator",
			Name:      "syncpolicy_resources_pruned_total",
			Help:      "Total number of resources pruned by SyncPolicy reconciliations.",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		resourceRepositoryPollTotal,
		resourceRepositoryRevisionChangesTotal,
		syncPolicyReconcileTotal,
		syncPolicyReconcileDurationSeconds,
		syncPolicyResourcesAppliedTotal,
		syncPolicyResourcesPrunedTotal,
	)
}
