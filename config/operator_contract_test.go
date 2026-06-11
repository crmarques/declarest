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

package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type objectMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type policyRule struct {
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

type subjectRef struct {
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type roleRef struct {
	APIGroup string `yaml:"apiGroup"`
	Kind     string `yaml:"kind"`
	Name     string `yaml:"name"`
}

type rbacManifest struct {
	Kind     string       `yaml:"kind"`
	Metadata objectMeta   `yaml:"metadata"`
	Rules    []policyRule `yaml:"rules"`
	Subjects []subjectRef `yaml:"subjects"`
	RoleRef  roleRef      `yaml:"roleRef"`
}

type kustomization struct {
	Resources []string `yaml:"resources"`
}

type validatingWebhookConfiguration struct {
	Webhooks []webhookDefinition `yaml:"webhooks"`
}

type clusterServiceVersion struct {
	Spec struct {
		WebhookDefinitions []webhookDefinition `yaml:"webhookdefinitions"`
	} `yaml:"spec"`
}

type webhookDefinition struct {
	Name         string        `yaml:"name"`
	GenerateName string        `yaml:"generateName"`
	Rules        []webhookRule `yaml:"rules"`
}

type webhookRule struct {
	Resources  []string `yaml:"resources"`
	Operations []string `yaml:"operations"`
}

func TestOperatorRBACSourcesStayAlignedWithManagerRuntime(t *testing.T) {
	t.Parallel()

	kustomizeConfig := kustomization{}
	loadYAMLDocument(t, repoPath("config", "rbac", "kustomization.yaml"), &kustomizeConfig)

	for _, resource := range []string{
		"service_account.yaml",
		"cluster_role.yaml",
		"cluster_role_binding.yaml",
		"generated_resources_cluster_role.yaml",
		"generated_resources_cluster_role_binding.yaml",
		"role.yaml",
		"role_binding.yaml",
	} {
		if !stringSliceContainsValue(kustomizeConfig.Resources, resource) {
			t.Fatalf("expected config/rbac/kustomization.yaml to include %q, got %#v", resource, kustomizeConfig.Resources)
		}
	}

	clusterRole := rbacManifest{}
	loadYAMLDocument(t, repoPath("config", "rbac", "cluster_role.yaml"), &clusterRole)
	if clusterRole.Kind != "ClusterRole" {
		t.Fatalf("expected cluster_role.yaml kind ClusterRole, got %q", clusterRole.Kind)
	}
	if clusterRole.Metadata.Name != "declarest-operator" {
		t.Fatalf("expected cluster role name declarest-operator, got %q", clusterRole.Metadata.Name)
	}
	if clusterRole.Metadata.Namespace != "" {
		t.Fatalf("expected cluster role namespace to be empty, got %q", clusterRole.Metadata.Namespace)
	}
	if !hasPolicyRule(clusterRole.Rules, []string{"declarest.io"}, []string{"resourcerepositories", "managedservices", "secretstores", "syncpolicies", "repositorywebhooks"}, []string{"get", "list", "watch", "create", "update", "patch", "delete"}) {
		t.Fatalf("expected cluster role to grant cluster-scope Declarest resource access, got %#v", clusterRole.Rules)
	}
	if !hasPolicyRule(clusterRole.Rules, []string{"declarest.io"}, []string{"resourcerepositories/status", "managedservices/status", "secretstores/status", "syncpolicies/status", "repositorywebhooks/status"}, []string{"get", "update", "patch"}) {
		t.Fatalf("expected cluster role to grant status updates, got %#v", clusterRole.Rules)
	}
	if !hasPolicyRule(clusterRole.Rules, []string{"declarest.io"}, []string{"resourcerepositories/finalizers", "managedservices/finalizers", "secretstores/finalizers", "syncpolicies/finalizers", "repositorywebhooks/finalizers"}, []string{"update"}) {
		t.Fatalf("expected cluster role to grant finalizer updates, got %#v", clusterRole.Rules)
	}
	if !hasPolicyRule(clusterRole.Rules, []string{""}, []string{"events"}, []string{"create", "patch"}) {
		t.Fatalf("expected cluster role to grant event recording, got %#v", clusterRole.Rules)
	}
	if !hasPolicyRule(clusterRole.Rules, []string{""}, []string{"secrets"}, []string{"get", "list", "watch"}) {
		t.Fatalf("expected cluster role to grant secret reads, got %#v", clusterRole.Rules)
	}
	if hasPolicyRule(clusterRole.Rules, []string{""}, []string{"persistentvolumeclaims"}, []string{"create", "update", "patch", "delete"}) {
		t.Fatalf("cluster role must not grant per-resource pvc management, got %#v", clusterRole.Rules)
	}
	if hasPolicyRule(clusterRole.Rules, []string{"apiextensions.k8s.io"}, []string{"customresourcedefinitions"}, []string{"create", "update", "patch", "delete"}) {
		t.Fatalf("cluster role must not include CRD generation admin permissions, got %#v", clusterRole.Rules)
	}

	generatedRole := rbacManifest{}
	loadYAMLDocument(t, repoPath("config", "rbac", "generated_resources_cluster_role.yaml"), &generatedRole)
	if generatedRole.Kind != "ClusterRole" || generatedRole.Metadata.Name != "declarest-operator-generated-resources" {
		t.Fatalf("expected generated resources aggregate ClusterRole, got %#v", generatedRole)
	}

	leaderElectionRole := rbacManifest{}
	loadYAMLDocument(t, repoPath("config", "rbac", "role.yaml"), &leaderElectionRole)
	if leaderElectionRole.Kind != "Role" {
		t.Fatalf("expected role.yaml kind Role, got %q", leaderElectionRole.Kind)
	}
	if leaderElectionRole.Metadata.Namespace != "declarest-system" {
		t.Fatalf("expected leader election role namespace declarest-system, got %q", leaderElectionRole.Metadata.Namespace)
	}
	if !hasPolicyRule(leaderElectionRole.Rules, []string{"coordination.k8s.io"}, []string{"leases"}, []string{"get", "list", "watch", "create", "update", "patch"}) {
		t.Fatalf("expected leader election role to grant lease access, got %#v", leaderElectionRole.Rules)
	}

	clusterRoleBinding := rbacManifest{}
	loadYAMLDocument(t, repoPath("config", "rbac", "cluster_role_binding.yaml"), &clusterRoleBinding)
	if clusterRoleBinding.Kind != "ClusterRoleBinding" {
		t.Fatalf("expected cluster_role_binding.yaml kind ClusterRoleBinding, got %q", clusterRoleBinding.Kind)
	}
	if clusterRoleBinding.RoleRef.Kind != "ClusterRole" || clusterRoleBinding.RoleRef.Name != "declarest-operator" {
		t.Fatalf("expected cluster role binding to reference declarest-operator ClusterRole, got %#v", clusterRoleBinding.RoleRef)
	}
	if len(clusterRoleBinding.Subjects) != 1 || clusterRoleBinding.Subjects[0].Kind != "ServiceAccount" || clusterRoleBinding.Subjects[0].Name != "declarest-operator" || clusterRoleBinding.Subjects[0].Namespace != "declarest-system" {
		t.Fatalf("expected cluster role binding to target declarest-system/declarest-operator, got %#v", clusterRoleBinding.Subjects)
	}

	roleBinding := rbacManifest{}
	loadYAMLDocument(t, repoPath("config", "rbac", "role_binding.yaml"), &roleBinding)
	if roleBinding.Kind != "RoleBinding" {
		t.Fatalf("expected role_binding.yaml kind RoleBinding, got %q", roleBinding.Kind)
	}
	if roleBinding.RoleRef.Kind != "Role" || roleBinding.RoleRef.Name != "declarest-operator" {
		t.Fatalf("expected role binding to reference declarest-operator Role, got %#v", roleBinding.RoleRef)
	}
	if len(roleBinding.Subjects) != 1 || roleBinding.Subjects[0].Kind != "ServiceAccount" || roleBinding.Subjects[0].Name != "declarest-operator" || roleBinding.Subjects[0].Namespace != "declarest-system" {
		t.Fatalf("expected role binding to target declarest-system/declarest-operator, got %#v", roleBinding.Subjects)
	}
}

func TestAdmissionWebhookManifestsCoverRegisteredAPIs(t *testing.T) {
	t.Parallel()

	webhookConfig := validatingWebhookConfiguration{}
	loadYAMLDocument(t, repoPath("config", "manifests", "webhooks.yaml"), &webhookConfig)
	assertWebhookCoverage(t, webhookConfig.Webhooks)

	csv := clusterServiceVersion{}
	loadYAMLDocument(t, repoPath("bundle", "manifests", "declarest-operator.clusterserviceversion.yaml"), &csv)
	assertWebhookCoverage(t, csv.Spec.WebhookDefinitions)
}

func TestResourceRepositoryCRDRequiresExplicitPVCAccessModes(t *testing.T) {
	t.Parallel()

	document := map[string]any{}
	loadYAMLDocument(t, repoPath("config", "crd", "bases", "declarest.io_resourcerepositories.yaml"), &document)

	spec := objectProperty(t, document, "spec")
	versions := arrayProperty(t, spec, "versions")
	version, ok := versions[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first CRD version to be an object, got %#v", versions[0])
	}

	schema := objectProperty(t, version, "schema")
	openAPIV3Schema := objectProperty(t, schema, "openAPIV3Schema")
	topProperties := objectProperty(t, openAPIV3Schema, "properties")
	specSchema := objectProperty(t, objectProperty(t, topProperties, "spec"), "properties")
	storageSchema := objectProperty(t, objectProperty(t, specSchema, "storage"), "properties")
	pvcSchema := objectProperty(t, storageSchema, "pvc")

	required := arrayProperty(t, pvcSchema, "required")
	if !arrayContainsString(required, "accessModes") {
		t.Fatalf("expected ResourceRepository CRD pvc.required to include accessModes, got %#v", required)
	}
	if !arrayContainsString(required, "requests") {
		t.Fatalf("expected ResourceRepository CRD pvc.required to include requests, got %#v", required)
	}

	accessModes := objectProperty(t, objectProperty(t, pvcSchema, "properties"), "accessModes")
	if value, ok := accessModes["minItems"]; !ok || numericValue(value) != 1 {
		t.Fatalf("expected ResourceRepository CRD pvc.accessModes minItems=1, got %#v", accessModes["minItems"])
	}
}

//nolint:staticcheck // Samples still carry deprecated v1alpha1 storage for compatibility validation.
func TestOperatorSamplesUseDeprecatedStorageCompatibilityShape(t *testing.T) {
	t.Parallel()

	repoSample := declarestv1alpha1.ResourceRepository{}
	loadYAMLDocument(t, repoPath("config", "samples", "declarest_v1alpha1_resourcerepository.yaml"), &repoSample)
	repoSample.Default()
	if repoSample.Spec.Storage.ExistingPVC == nil || repoSample.Spec.Storage.ExistingPVC.Name == "" {
		t.Fatal("expected ResourceRepository sample to use existingPVC compatibility storage")
	}
	if err := repoSample.ValidateSpec(); err != nil {
		t.Fatalf("expected ResourceRepository sample to validate, got %v", err)
	}

	secretStoreSample := declarestv1alpha1.SecretStore{}
	loadYAMLDocument(t, repoPath("config", "samples", "declarest_v1alpha1_secretstore.yaml"), &secretStoreSample)
	if secretStoreSample.Spec.File == nil || secretStoreSample.Spec.File.Storage.ExistingPVC == nil || secretStoreSample.Spec.File.Storage.ExistingPVC.Name == "" {
		t.Fatal("expected SecretStore sample to use existingPVC compatibility storage")
	}
	if err := secretStoreSample.ValidateSpec(); err != nil {
		t.Fatalf("expected SecretStore sample to validate, got %v", err)
	}
}

func repoPath(parts ...string) string {
	return filepath.Join(append([]string{".."}, parts...)...)
}

func loadYAMLDocument(t *testing.T, path string, out any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read yaml %s: %v", path, err)
	}
	jsonData, err := k8syaml.ToJSON(data)
	if err != nil {
		t.Fatalf("convert yaml %s to json: %v", path, err)
	}
	if err := json.Unmarshal(jsonData, out); err != nil {
		t.Fatalf("decode yaml %s: %v", path, err)
	}
}

func hasPolicyRule(rules []policyRule, apiGroups []string, resources []string, verbs []string) bool {
	for _, rule := range rules {
		if stringSliceContainsAll(rule.APIGroups, apiGroups) && stringSliceContainsAll(rule.Resources, resources) && stringSliceContainsAll(rule.Verbs, verbs) {
			return true
		}
	}
	return false
}

func stringSliceContainsAll(values []string, expected []string) bool {
	for _, item := range expected {
		if !stringSliceContainsValue(values, item) {
			return false
		}
	}
	return true
}

func stringSliceContainsValue(values []string, expected string) bool {
	for _, item := range values {
		if item == expected {
			return true
		}
	}
	return false
}

func arrayContainsString(values []any, expected string) bool {
	for _, item := range values {
		if text, ok := item.(string); ok && text == expected {
			return true
		}
	}
	return false
}

func numericValue(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return -1
	}
}

func assertWebhookCoverage(t *testing.T, webhooks []webhookDefinition) {
	t.Helper()

	expected := map[string][]string{
		"resourcerepositories": {"CREATE", "UPDATE", "DELETE"},
		"managedservices":      {"CREATE", "UPDATE", "DELETE"},
		"secretstores":         {"CREATE", "UPDATE", "DELETE"},
		"syncpolicies":         {"CREATE", "UPDATE"},
		"repositorywebhooks":   {"CREATE", "UPDATE", "DELETE"},
		"metadatabundles":      {"CREATE", "UPDATE", "DELETE"},
		"crdgenerators":        {"CREATE", "UPDATE", "DELETE"},
	}
	for resource, operations := range expected {
		if !webhookDefinitionsContainRule(webhooks, resource, operations) {
			t.Fatalf("expected webhook coverage for %s operations %v, got %#v", resource, operations, webhooks)
		}
	}
}

func webhookDefinitionsContainRule(webhooks []webhookDefinition, resource string, operations []string) bool {
	for _, webhook := range webhooks {
		for _, rule := range webhook.Rules {
			if stringSliceContainsValue(rule.Resources, resource) && stringSliceContainsAll(rule.Operations, operations) {
				return true
			}
		}
	}
	return false
}
