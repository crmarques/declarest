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
	"fmt"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition type specific to CRDGenerator indicating that one or more generated
// resources are shadowed by or shadow a SyncPolicy-managed resource.
const ConditionTypeConflicting = "Conflicting"

var (
	crdGroupPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`)
	crdKindPattern    = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	crdNamePattern    = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	crdShortPattern   = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	crdVersionPattern = regexp.MustCompile(`^v[1-9][0-9]*([a-z]+[1-9][0-9]*)?$`)
)

// reserved group suffixes the CRDGenerator MUST refuse so users cannot shadow
// operator-owned or core Kubernetes CRDs.
var reservedGroupSuffixes = []string{
	".declarest.io",
	"declarest.io",
	"k8s.io",
	".k8s.io",
	"apiextensions.k8s.io",
}

// reserved kinds within the declarest.io group that the generator MUST refuse
// to take over. Matching is case-insensitive on the value.
var reservedKinds = map[string]struct{}{
	"managedservice":     {},
	"resourcerepository": {},
	"secretstore":        {},
	"syncpolicy":         {},
	"repositorywebhook":  {},
	"metadatabundle":     {},
	"crdgenerator":       {},
}

type CRDGeneratorNames struct {
	// +kubebuilder:validation:MinLength=1
	Kind     string `json:"kind"`
	ListKind string `json:"listKind,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Plural string `json:"plural"`
	// +kubebuilder:validation:MinLength=1
	Singular   string   `json:"singular"`
	ShortNames []string `json:"shortNames,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

// +kubebuilder:validation:Enum=Cluster;Namespaced
type CRDGeneratorScope string

const (
	CRDGeneratorScopeCluster    CRDGeneratorScope = "Cluster"
	CRDGeneratorScopeNamespaced CRDGeneratorScope = "Namespaced"
)

type CRDGeneratorVersion struct {
	// +kubebuilder:validation:MinLength=1
	Name              string                    `json:"name"`
	Served            bool                      `json:"served"`
	Storage           bool                      `json:"storage"`
	MetadataBundleRef NamespacedObjectReference `json:"metadataBundleRef"`
	// +kubebuilder:validation:MinLength=1
	CollectionPath           string                      `json:"collectionPath"`
	AdditionalPrinterColumns []CRDGeneratorPrinterColumn `json:"additionalPrinterColumns,omitempty"`
}

type CRDGeneratorPrinterColumn struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`
	// +kubebuilder:validation:MinLength=1
	JSONPath    string `json:"jsonPath"`
	Description string `json:"description,omitempty"`
	Priority    int32  `json:"priority,omitempty"`
}

// +kubebuilder:validation:Enum=None
type CRDGeneratorConversionStrategy string

const CRDGeneratorConversionNone CRDGeneratorConversionStrategy = "None"

type CRDGeneratorConversion struct {
	Strategy CRDGeneratorConversionStrategy `json:"strategy,omitempty"`
}

type CRDGeneratorSpec struct {
	// +kubebuilder:validation:MinLength=1
	Group      string                  `json:"group"`
	Scope      CRDGeneratorScope       `json:"scope"`
	Names      CRDGeneratorNames       `json:"names"`
	Versions   []CRDGeneratorVersion   `json:"versions"`
	Conversion *CRDGeneratorConversion `json:"conversion,omitempty"`
}

type CRDGeneratorResolvedVersion struct {
	MetadataBundle  CRDGeneratorResolvedBundle `json:"metadataBundle,omitempty"`
	LastAppliedTime *metav1.Time               `json:"lastAppliedTime,omitempty"`
}

type CRDGeneratorResolvedBundle struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type CRDGeneratorStatus struct {
	ObservedGeneration int64                                  `json:"observedGeneration,omitempty"`
	GeneratedCRDName   string                                 `json:"generatedCRDName,omitempty"`
	GeneratedCRDUID    string                                 `json:"generatedCRDUID,omitempty"`
	ResolvedVersions   map[string]CRDGeneratorResolvedVersion `json:"resolvedVersions,omitempty"`
	Conditions         []metav1.Condition                     `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=crdg,categories=declarest
// +kubebuilder:printcolumn:name="Group",type="string",JSONPath=".spec.group"
// +kubebuilder:printcolumn:name="Kind",type="string",JSONPath=".spec.names.kind"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".status.generatedCRDName"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type CRDGenerator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CRDGeneratorSpec   `json:"spec,omitempty"`
	Status CRDGeneratorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type CRDGeneratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CRDGenerator `json:"items"`
}

// GeneratedCRDName returns the name a CRD generated from this CRDGenerator
// MUST carry, following the Kubernetes convention `<plural>.<group>`.
func (g *CRDGenerator) GeneratedCRDName() string {
	if g == nil {
		return ""
	}
	plural := strings.TrimSpace(g.Spec.Names.Plural)
	group := strings.TrimSpace(g.Spec.Group)
	if plural == "" || group == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s", plural, group)
}

func (g *CRDGenerator) Default() {
	if g == nil {
		return
	}
	if strings.TrimSpace(g.Spec.Names.ListKind) == "" {
		g.Spec.Names.ListKind = g.Spec.Names.Kind + "List"
	}
	if g.Spec.Conversion == nil {
		g.Spec.Conversion = &CRDGeneratorConversion{Strategy: CRDGeneratorConversionNone}
	} else if strings.TrimSpace(string(g.Spec.Conversion.Strategy)) == "" {
		g.Spec.Conversion.Strategy = CRDGeneratorConversionNone
	}
}

func (g *CRDGenerator) ValidateSpec() error {
	if g == nil {
		return fmt.Errorf("crd generator is required")
	}
	return validateCRDGeneratorSpec(&g.Spec)
}

func validateCRDGeneratorSpec(spec *CRDGeneratorSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is required")
	}
	group := strings.TrimSpace(spec.Group)
	if group == "" {
		return fmt.Errorf("spec.group is required")
	}
	if !crdGroupPattern.MatchString(group) {
		return fmt.Errorf("spec.group %q is not a valid DNS subdomain", group)
	}
	for _, suffix := range reservedGroupSuffixes {
		if group == strings.TrimPrefix(suffix, ".") {
			return fmt.Errorf("spec.group %q is reserved", group)
		}
		if strings.HasSuffix(group, suffix) && suffix != group {
			return fmt.Errorf("spec.group %q is reserved", group)
		}
	}
	if spec.Scope != CRDGeneratorScopeCluster && spec.Scope != CRDGeneratorScopeNamespaced {
		return fmt.Errorf("spec.scope must be Cluster or Namespaced")
	}
	if err := validateCRDGeneratorNames(&spec.Names); err != nil {
		return err
	}
	if _, reserved := reservedKinds[strings.ToLower(spec.Names.Kind)]; reserved && strings.HasSuffix(group, "declarest.io") {
		return fmt.Errorf("spec.names.kind %q is reserved within the declarest.io group", spec.Names.Kind)
	}
	if len(spec.Versions) == 0 {
		return fmt.Errorf("spec.versions must define at least one version")
	}
	storageCount := 0
	servedCount := 0
	seenNames := make(map[string]struct{}, len(spec.Versions))
	for idx := range spec.Versions {
		version := &spec.Versions[idx]
		path := fmt.Sprintf("spec.versions[%d]", idx)
		if err := validateCRDGeneratorVersion(version, path); err != nil {
			return err
		}
		if _, dup := seenNames[version.Name]; dup {
			return fmt.Errorf("%s.name %q is duplicated", path, version.Name)
		}
		seenNames[version.Name] = struct{}{}
		if version.Storage {
			storageCount++
		}
		if version.Served {
			servedCount++
		}
	}
	if storageCount != 1 {
		return fmt.Errorf("spec.versions must declare exactly one storage version, got %d", storageCount)
	}
	if servedCount == 0 {
		return fmt.Errorf("spec.versions must declare at least one served version")
	}
	if spec.Conversion != nil && spec.Conversion.Strategy != "" && spec.Conversion.Strategy != CRDGeneratorConversionNone {
		return fmt.Errorf("spec.conversion.strategy must be None")
	}
	return nil
}

func validateCRDGeneratorNames(names *CRDGeneratorNames) error {
	if names == nil {
		return fmt.Errorf("spec.names is required")
	}
	if !crdKindPattern.MatchString(names.Kind) {
		return fmt.Errorf("spec.names.kind %q must start with uppercase and contain only alphanumeric characters", names.Kind)
	}
	if !crdNamePattern.MatchString(names.Plural) {
		return fmt.Errorf("spec.names.plural %q must be lowercase alphanumeric", names.Plural)
	}
	if !crdNamePattern.MatchString(names.Singular) {
		return fmt.Errorf("spec.names.singular %q must be lowercase alphanumeric", names.Singular)
	}
	if names.ListKind != "" && !crdKindPattern.MatchString(names.ListKind) {
		return fmt.Errorf("spec.names.listKind %q must start with uppercase and contain only alphanumeric characters", names.ListKind)
	}
	for idx, short := range names.ShortNames {
		if !crdShortPattern.MatchString(short) {
			return fmt.Errorf("spec.names.shortNames[%d] %q must be lowercase alphanumeric", idx, short)
		}
	}
	return nil
}

func validateCRDGeneratorVersion(version *CRDGeneratorVersion, path string) error {
	if !crdVersionPattern.MatchString(version.Name) {
		return fmt.Errorf("%s.name %q must match the Kubernetes version regex v[1-9][0-9]*([a-z]+[1-9][0-9]*)?", path, version.Name)
	}
	if strings.TrimSpace(version.MetadataBundleRef.Name) == "" {
		return fmt.Errorf("%s.metadataBundleRef.name is required", path)
	}
	if _, err := normalizePath(version.CollectionPath); err != nil {
		return fmt.Errorf("%s.collectionPath is invalid: %w", path, err)
	}
	for colIdx, column := range version.AdditionalPrinterColumns {
		colPath := fmt.Sprintf("%s.additionalPrinterColumns[%d]", path, colIdx)
		if strings.TrimSpace(column.Name) == "" {
			return fmt.Errorf("%s.name is required", colPath)
		}
		if strings.TrimSpace(column.Type) == "" {
			return fmt.Errorf("%s.type is required", colPath)
		}
		if strings.TrimSpace(column.JSONPath) == "" {
			return fmt.Errorf("%s.jsonPath is required", colPath)
		}
	}
	return nil
}
