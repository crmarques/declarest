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
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sevents "k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	crdGeneratorReasonCRDApplyFailed  = "CRDApplyFailed"
	crdGeneratorReasonBundleNotReady  = "BundleNotReady"
	crdGeneratorReasonCRDPresent      = "GeneratedCRDPresent"
	crdGeneratorReasonCRDEstablishing = "CRDEstablishing"

	crdGeneratorGroupKindIndex = "spec.group|spec.names.kind"
)

// dynamicWatchRegistry is the plumbing contract the CRDGenerator reconciler
// uses to drive the metacontroller that watches generated CRs. The concrete
// implementation lives in the GeneratedResourceReconciler; here we depend on
// a narrow interface so the controller can be constructed before the dynamic
// runtime is ready.
type dynamicWatchRegistry interface {
	EnsureWatch(gvr GeneratedGVR, generator types.NamespacedName) error
	StopWatch(gvr GeneratedGVR)
}

// GeneratedGVR identifies a dynamically generated CRD by group/version/resource.
type GeneratedGVR struct {
	Group    string
	Version  string
	Resource string
	Kind     string
}

// CRDGeneratorReconciler reconciles CRDGenerator resources by owning the
// lifecycle of `apiextensions.k8s.io/v1 CustomResourceDefinition` objects plus
// a companion aggregated ClusterRole for dynamic RBAC. Cross-scope
// ownerReferences are impossible (CRDGenerator is namespaced, CRD is cluster
// scoped), so ownership is tracked via a label + annotation pair.
type CRDGeneratorReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                k8sevents.EventRecorder
	MaxConcurrentReconciles int

	RESTMapper meta.RESTMapper
	Watches    dynamicWatchRegistry
	Conflicts  *ConflictIndex
}

func (r *CRDGeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("crdGenerator", req.String(), "reconcile_id", uuidString())

	generator := &declarestv1alpha1.CRDGenerator{}
	if err := r.Get(ctx, req.NamespacedName, generator); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(generator, finalizerName) {
		controllerutil.AddFinalizer(generator, finalizerName)
		if err := r.Update(ctx, generator); err != nil {
			return ctrl.Result{}, err
		}
	}

	if !generator.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, generator)
	}

	runtimeGenerator := generator.DeepCopy()
	runtimeGenerator.Default()
	if err := runtimeGenerator.ValidateSpec(); err != nil {
		emitEventf(r.Recorder, generator, corev1.EventTypeWarning, "SpecInvalid", "validation failed: %v", err)
		return returnAfterSetNotReady(ctx, func(innerCtx context.Context, reason, message string) error {
			return r.setNotReady(innerCtx, generator, reason, message)
		}, conditionReasonSpecInvalid, err.Error(), 0)
	}

	// Verify every referenced MetadataBundle is Ready and records resolved versions in status.
	resolvedVersions, err := r.resolveReferencedBundles(ctx, generator, runtimeGenerator)
	if err != nil {
		emitEventf(r.Recorder, generator, corev1.EventTypeWarning, crdGeneratorReasonBundleNotReady, "%v", err)
		return returnAfterSetNotReady(ctx, func(innerCtx context.Context, reason, message string) error {
			return r.setNotReady(innerCtx, generator, reason, message)
		}, crdGeneratorReasonBundleNotReady, err.Error(), defaultTransientRequeueInterval)
	}

	desired := buildGeneratedCRD(runtimeGenerator)
	applied, appliedErr := r.applyCRD(ctx, generator, desired)
	if appliedErr != nil {
		emitEventf(r.Recorder, generator, corev1.EventTypeWarning, crdGeneratorReasonCRDApplyFailed, "%v", appliedErr)
		return returnAfterSetNotReady(ctx, func(innerCtx context.Context, reason, message string) error {
			return r.setNotReady(innerCtx, generator, reason, message)
		}, crdGeneratorReasonCRDApplyFailed, appliedErr.Error(), defaultTransientRequeueInterval)
	}

	if !crdEstablished(applied) {
		if err := r.setNotReady(ctx, generator, crdGeneratorReasonCRDEstablishing, "waiting for CustomResourceDefinition to reach Established=True"); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: defaultTransientRequeueInterval}, nil
	}

	resetRESTMapper(r.RESTMapper)

	if err := r.applyAggregatedClusterRole(ctx, generator, runtimeGenerator); err != nil {
		emitEventf(r.Recorder, generator, corev1.EventTypeWarning, "ClusterRoleApplyFailed", "%v", err)
		return returnAfterSetNotReady(ctx, func(innerCtx context.Context, reason, message string) error {
			return r.setNotReady(innerCtx, generator, reason, message)
		}, "ClusterRoleApplyFailed", err.Error(), defaultTransientRequeueInterval)
	}

	if r.Watches != nil {
		for _, version := range runtimeGenerator.Spec.Versions {
			if !version.Served {
				continue
			}
			gvr := GeneratedGVR{
				Group:    runtimeGenerator.Spec.Group,
				Version:  version.Name,
				Resource: runtimeGenerator.Spec.Names.Plural,
				Kind:     runtimeGenerator.Spec.Names.Kind,
			}
			if watchErr := r.Watches.EnsureWatch(gvr, types.NamespacedName{Namespace: generator.Namespace, Name: generator.Name}); watchErr != nil {
				logger.Error(watchErr, "ensure dynamic watch", "gvr", gvr)
			}
		}
	}

	generator.Status.ObservedGeneration = generator.Generation
	generator.Status.GeneratedCRDName = applied.Name
	generator.Status.GeneratedCRDUID = string(applied.UID)
	generator.Status.ResolvedVersions = resolvedVersions
	generator.Status.Conditions = setStatusCondition(
		generator.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionTrue,
		conditionReasonReady,
		"generated CRD is established",
	)
	generator.Status.Conditions = setStatusCondition(
		generator.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionFalse,
		conditionReasonReady,
		"",
	)
	if err := r.Status().Update(ctx, generator); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("crd generator reconciled", "crd", applied.Name, "versions", versionNames(runtimeGenerator))
	return ctrl.Result{}, nil
}

func (r *CRDGeneratorReconciler) handleDeletion(ctx context.Context, generator *declarestv1alpha1.CRDGenerator) (ctrl.Result, error) {
	crdName := generator.GeneratedCRDName()
	if crdName == "" && strings.TrimSpace(generator.Status.GeneratedCRDName) != "" {
		crdName = generator.Status.GeneratedCRDName
	}
	if crdName == "" {
		controllerutil.RemoveFinalizer(generator, finalizerName)
		return ctrl.Result{}, r.Update(ctx, generator)
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := r.Get(ctx, types.NamespacedName{Name: crdName}, crd)
	switch {
	case apierrors.IsNotFound(err):
		if r.Watches != nil {
			for _, version := range generator.Spec.Versions {
				r.Watches.StopWatch(GeneratedGVR{
					Group:    generator.Spec.Group,
					Version:  version.Name,
					Resource: generator.Spec.Names.Plural,
				})
			}
		}
		if r.Conflicts != nil {
			r.Conflicts.Unregister(generator.Namespace, generator.Name, "")
		}
		controllerutil.RemoveFinalizer(generator, finalizerName)
		return ctrl.Result{}, r.Update(ctx, generator)
	case err != nil:
		return ctrl.Result{}, err
	}

	message := fmt.Sprintf(
		"generated CustomResourceDefinition %q still exists; delete it before removing the CRDGenerator",
		crdName,
	)
	emitEventf(r.Recorder, generator, corev1.EventTypeWarning, crdGeneratorReasonCRDPresent, "%s", message)
	if err := r.setNotReady(ctx, generator, crdGeneratorReasonCRDPresent, message); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: defaultTransientRequeueInterval}, nil
}

func (r *CRDGeneratorReconciler) resolveReferencedBundles(
	ctx context.Context,
	generator *declarestv1alpha1.CRDGenerator,
	runtimeGenerator *declarestv1alpha1.CRDGenerator,
) (map[string]declarestv1alpha1.CRDGeneratorResolvedVersion, error) {
	resolved := make(map[string]declarestv1alpha1.CRDGeneratorResolvedVersion, len(runtimeGenerator.Spec.Versions))
	for _, version := range runtimeGenerator.Spec.Versions {
		bundle := &declarestv1alpha1.MetadataBundle{}
		key := types.NamespacedName{Namespace: generator.Namespace, Name: strings.TrimSpace(version.MetadataBundleRef.Name)}
		if err := r.Get(ctx, key, bundle); err != nil {
			return nil, fmt.Errorf("resolve metadata bundle %s/%s: %w", key.Namespace, key.Name, err)
		}
		if !metadataBundleReady(bundle) {
			return nil, fmt.Errorf("metadata bundle %s/%s is not Ready", key.Namespace, key.Name)
		}
		entry := declarestv1alpha1.CRDGeneratorResolvedVersion{}
		if bundle.Status.Manifest != nil {
			entry.MetadataBundle = declarestv1alpha1.CRDGeneratorResolvedBundle{
				Name:    bundle.Status.Manifest.Name,
				Version: bundle.Status.Manifest.Version,
			}
		}
		now := metav1.NewTime(time.Now().UTC())
		entry.LastAppliedTime = &now
		resolved[version.Name] = entry
	}
	return resolved, nil
}

func (r *CRDGeneratorReconciler) applyCRD(
	ctx context.Context,
	generator *declarestv1alpha1.CRDGenerator,
	desired *apiextensionsv1.CustomResourceDefinition,
) (*apiextensionsv1.CustomResourceDefinition, error) {
	existing := &apiextensionsv1.CustomResourceDefinition{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)
	switch {
	case apierrors.IsNotFound(err):
		if err := r.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("create CRD %s: %w", desired.Name, err)
		}
		return desired, nil
	case err != nil:
		return nil, err
	}

	// Adoption check: operator refuses to mutate a CRD it does not own.
	labelValue := existing.Labels[declarestv1alpha1.CRDGeneratorOwnerLabel]
	annotationValue := existing.Annotations[declarestv1alpha1.CRDGeneratorOwnerAnnotation]
	ownerAnnotation := fmt.Sprintf("%s/%s", generator.Namespace, generator.Name)
	if labelValue != declarestv1alpha1.CRDGeneratorOwnerLabelValue || annotationValue != ownerAnnotation {
		return nil, fmt.Errorf(
			"CustomResourceDefinition %q is not owned by this CRDGenerator (label=%q, annotation=%q)",
			desired.Name, labelValue, annotationValue,
		)
	}

	existing.Spec = desired.Spec
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range desired.Labels {
		existing.Labels[k] = v
	}
	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	for k, v := range desired.Annotations {
		existing.Annotations[k] = v
	}
	if err := r.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update CRD %s: %w", desired.Name, err)
	}
	return existing, nil
}

func (r *CRDGeneratorReconciler) applyAggregatedClusterRole(
	ctx context.Context,
	generator *declarestv1alpha1.CRDGenerator,
	runtimeGenerator *declarestv1alpha1.CRDGenerator,
) error {
	name := fmt.Sprintf("declarest-crdgenerator-%s-%s", generator.Namespace, generator.Name)
	desired := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				declarestv1alpha1.CRDGeneratorOwnerLabel:   declarestv1alpha1.CRDGeneratorOwnerLabelValue,
				declarestv1alpha1.AggregateToOperatorLabel: declarestv1alpha1.AggregateToOperatorLabelValue,
			},
			Annotations: map[string]string{
				declarestv1alpha1.CRDGeneratorOwnerAnnotation: fmt.Sprintf("%s/%s", generator.Namespace, generator.Name),
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{runtimeGenerator.Spec.Group},
				Resources: []string{
					runtimeGenerator.Spec.Names.Plural,
					runtimeGenerator.Spec.Names.Plural + "/status",
					runtimeGenerator.Spec.Names.Plural + "/finalizers",
				},
				Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	existing := &rbacv1.ClusterRole{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	switch {
	case apierrors.IsNotFound(err):
		return r.Create(ctx, desired)
	case err != nil:
		return err
	}
	existing.Rules = desired.Rules
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range desired.Labels {
		existing.Labels[k] = v
	}
	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	for k, v := range desired.Annotations {
		existing.Annotations[k] = v
	}
	return r.Update(ctx, existing)
}

func (r *CRDGeneratorReconciler) setNotReady(
	ctx context.Context,
	generator *declarestv1alpha1.CRDGenerator,
	reason string,
	message string,
) error {
	generator.Status.ObservedGeneration = generator.Generation
	generator.Status.Conditions = setStatusCondition(
		generator.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionFalse,
		reason,
		message,
	)
	generator.Status.Conditions = setStatusCondition(
		generator.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionTrue,
		reason,
		message,
	)
	return r.Status().Update(ctx, generator)
}

func (r *CRDGeneratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.RESTMapper == nil {
		r.RESTMapper = mgr.GetRESTMapper()
	}
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&declarestv1alpha1.CRDGenerator{},
		crdGeneratorGroupKindIndex,
		func(obj client.Object) []string {
			gen, ok := obj.(*declarestv1alpha1.CRDGenerator)
			if !ok {
				return nil
			}
			return []string{fmt.Sprintf("%s|%s", strings.ToLower(gen.Spec.Group), strings.ToLower(gen.Spec.Names.Kind))}
		},
	); err != nil {
		return fmt.Errorf("index CRDGenerator group/kind: %w", err)
	}

	bld := ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.CRDGenerator{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	if r.MaxConcurrentReconciles > 0 {
		bld = bld.WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	}
	return bld.Complete(r)
}

func buildGeneratedCRD(generator *declarestv1alpha1.CRDGenerator) *apiextensionsv1.CustomResourceDefinition {
	versions := make([]apiextensionsv1.CustomResourceDefinitionVersion, 0, len(generator.Spec.Versions))
	for _, version := range generator.Spec.Versions {
		v := apiextensionsv1.CustomResourceDefinitionVersion{
			Name:    version.Name,
			Served:  version.Served,
			Storage: version.Storage,
			Subresources: &apiextensionsv1.CustomResourceSubresources{
				Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
			},
			Schema: &apiextensionsv1.CustomResourceValidation{
				OpenAPIV3Schema: generatedCRDSchema(version.CollectionPath),
			},
		}
		for _, col := range version.AdditionalPrinterColumns {
			v.AdditionalPrinterColumns = append(v.AdditionalPrinterColumns, apiextensionsv1.CustomResourceColumnDefinition{
				Name:        col.Name,
				Type:        col.Type,
				JSONPath:    col.JSONPath,
				Description: col.Description,
				Priority:    col.Priority,
			})
		}
		versions = append(versions, v)
	}
	scope := apiextensionsv1.NamespaceScoped
	if generator.Spec.Scope == declarestv1alpha1.CRDGeneratorScopeCluster {
		scope = apiextensionsv1.ClusterScoped
	}
	ownerAnnotation := fmt.Sprintf("%s/%s", generator.Namespace, generator.Name)
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: generator.GeneratedCRDName(),
			Labels: map[string]string{
				declarestv1alpha1.CRDGeneratorOwnerLabel: declarestv1alpha1.CRDGeneratorOwnerLabelValue,
			},
			Annotations: map[string]string{
				declarestv1alpha1.CRDGeneratorOwnerAnnotation: ownerAnnotation,
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: generator.Spec.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:     generator.Spec.Names.Plural,
				Singular:   generator.Spec.Names.Singular,
				Kind:       generator.Spec.Names.Kind,
				ListKind:   generator.Spec.Names.ListKind,
				ShortNames: append([]string{}, generator.Spec.Names.ShortNames...),
				Categories: append([]string{}, generator.Spec.Names.Categories...),
			},
			Scope:                 scope,
			Versions:              versions,
			PreserveUnknownFields: false,
		},
	}
}

// generatedCRDSchema returns the structural schema applied to every generated
// CRD. It encodes the declarest contract: spec.managedServiceRef is required,
// spec.payload accepts any structured payload via the field-level
// preserve-unknown-fields escape hatch, status tracks reconciliation progress.
func generatedCRDSchema(collectionPath string) *apiextensionsv1.JSONSchemaProps {
	return &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"apiVersion": {Type: "string"},
			"kind":       {Type: "string"},
			"metadata":   {Type: "object", XPreserveUnknownFields: ptrTrue()},
			"spec": {
				Type:     "object",
				Required: []string{"managedServiceRef", "payload"},
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"managedServiceRef": {
						Type:     "object",
						Required: []string{"name"},
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"name": {Type: "string", MinLength: ptrInt64(1)},
						},
					},
					"secretStoreRef": {
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"name": {Type: "string", MinLength: ptrInt64(1)},
						},
					},
					"payload": {
						Type:                   "object",
						XPreserveUnknownFields: ptrTrue(),
					},
					"collectionPathOverride": {
						Type:        "string",
						Description: "Optional override; defaults to " + collectionPath + " from the CRDGenerator version.",
					},
				},
			},
			"status": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"observedGeneration":  {Type: "integer", Format: "int64"},
					"lastAppliedTime":     {Type: "string", Format: "date-time"},
					"lastAppliedRevision": {Type: "string"},
					"remoteID":            {Type: "string"},
					"conditions": {
						Type: "array",
						Items: &apiextensionsv1.JSONSchemaPropsOrArray{
							Schema: &apiextensionsv1.JSONSchemaProps{
								Type:                   "object",
								XPreserveUnknownFields: ptrTrue(),
							},
						},
					},
				},
			},
		},
	}
}

func ptrTrue() *bool {
	t := true
	return &t
}

func ptrInt64(v int64) *int64 {
	return &v
}

func crdEstablished(crd *apiextensionsv1.CustomResourceDefinition) bool {
	if crd == nil {
		return false
	}
	established := false
	namesAccepted := false
	for _, cond := range crd.Status.Conditions {
		switch cond.Type {
		case apiextensionsv1.Established:
			established = cond.Status == apiextensionsv1.ConditionTrue
		case apiextensionsv1.NamesAccepted:
			namesAccepted = cond.Status == apiextensionsv1.ConditionTrue
		}
	}
	return established && namesAccepted
}

func resetRESTMapper(mapper meta.RESTMapper) {
	if resettable, ok := mapper.(meta.ResettableRESTMapper); ok {
		resettable.Reset()
	}
}

func versionNames(generator *declarestv1alpha1.CRDGenerator) []string {
	names := make([]string, 0, len(generator.Spec.Versions))
	for _, version := range generator.Spec.Versions {
		names = append(names, version.Name)
	}
	sort.Strings(names)
	return names
}
