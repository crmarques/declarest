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
	"testing"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestResourceRepositoryReconcilePredicateOnGenerationChange(t *testing.T) {
	t.Parallel()

	predicate := resourceRepositoryReconcilePredicate()
	oldRepo := &declarestv1alpha1.ResourceRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "repo",
			Namespace:  "default",
			Generation: 1,
		},
	}
	newRepo := oldRepo.DeepCopy()
	newRepo.Generation = 2

	if !predicate.Update(event.UpdateEvent{ObjectOld: oldRepo, ObjectNew: newRepo}) {
		t.Fatal("expected predicate to reconcile on generation change")
	}
}

func TestResourceRepositoryReconcilePredicateOnWebhookAnnotationChange(t *testing.T) {
	t.Parallel()

	predicate := resourceRepositoryReconcilePredicate()
	oldRepo := &declarestv1alpha1.ResourceRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "repo",
			Namespace:  "default",
			Generation: 3,
			Annotations: map[string]string{
				repositoryWebhookAnnotationLastEventAt: "2026-03-04T12:00:00Z",
			},
		},
	}
	newRepo := oldRepo.DeepCopy()
	newRepo.Annotations[repositoryWebhookAnnotationLastEventAt] = "2026-03-04T12:05:00Z"

	if !predicate.Update(event.UpdateEvent{ObjectOld: oldRepo, ObjectNew: newRepo}) {
		t.Fatal("expected predicate to reconcile on webhook annotation change")
	}
}

func TestResourceRepositoryReconcilePredicateIgnoresStatusOnlyUpdate(t *testing.T) {
	t.Parallel()

	predicate := resourceRepositoryReconcilePredicate()
	oldRepo := &declarestv1alpha1.ResourceRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "repo",
			Namespace:  "default",
			Generation: 4,
		},
		Status: declarestv1alpha1.ResourceRepositoryStatus{
			LastFetchedRevision: "abc123",
		},
	}
	newRepo := oldRepo.DeepCopy()
	newRepo.Status.LastFetchedRevision = "def456"

	if predicate.Update(event.UpdateEvent{ObjectOld: oldRepo, ObjectNew: newRepo}) {
		t.Fatal("expected predicate to ignore status-only updates")
	}
}
