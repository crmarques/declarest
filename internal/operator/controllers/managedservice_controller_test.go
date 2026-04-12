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
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManagedServicePollIntervalFallsBackToDefault(t *testing.T) {
	t.Parallel()

	server := &declarestv1alpha1.ManagedService{}

	if got := managedServicePollInterval(server); got != 10*time.Minute {
		t.Fatalf("expected default poll interval 10m, got %v", got)
	}
}

func TestManagedServicePollIntervalUsesConfiguredValue(t *testing.T) {
	t.Parallel()

	server := &declarestv1alpha1.ManagedService{
		Spec: declarestv1alpha1.ManagedServiceSpec{
			PollInterval: &metav1.Duration{Duration: 45 * time.Second},
		},
	}

	if got := managedServicePollInterval(server); got != 45*time.Second {
		t.Fatalf("expected configured poll interval 45s, got %v", got)
	}
}
