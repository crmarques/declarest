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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ConditionTypeReady       = "Ready"
	ConditionTypeReconciling = "Reconciling"
	ConditionTypeStalled     = "Stalled"
)

// Finalizer attached to every DeclaREST-owned CR. Shared so controllers and
// webhook delete-gates agree on the name.
const FinalizerName = "declarest.io/cleanup"

// Label and annotation keys used by the CRDGenerator controller to adopt and
// track CustomResourceDefinitions it generates. Label values cannot contain
// "/", so the full owner coordinates live in the annotation.
const (
	CRDGeneratorOwnerLabel      = "declarest.io/owned-by"
	CRDGeneratorOwnerLabelValue = "crdgenerator"
	CRDGeneratorOwnerAnnotation = "declarest.io/crdgenerator"

	// Label applied to aggregated ClusterRoles generated for CRDGenerator CRDs
	// so the operator ClusterRole aggregates them via matchLabels.
	AggregateToOperatorLabel      = "declarest.io/aggregate-to-operator"
	AggregateToOperatorLabelValue = "true"
)

var (
	GroupVersion = schema.GroupVersion{Group: "declarest.io", Version: "v1alpha1"}

	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&ResourceRepository{},
		&ResourceRepositoryList{},
		&ManagedService{},
		&ManagedServiceList{},
		&SecretStore{},
		&SecretStoreList{},
		&SyncPolicy{},
		&SyncPolicyList{},
		&RepositoryWebhook{},
		&RepositoryWebhookList{},
		&MetadataBundle{},
		&MetadataBundleList{},
		&CRDGenerator{},
		&CRDGeneratorList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
