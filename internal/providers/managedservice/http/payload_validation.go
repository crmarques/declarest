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

package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	metadatavalidation "github.com/crmarques/declarest/metadata/validation"
	"github.com/crmarques/declarest/resource"
)

func (g *Client) validateOperationPayload(
	ctx context.Context,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
	spec metadata.OperationSpec,
) error {
	if spec.Validate == nil {
		return nil
	}

	payloadType := g.metadataPayloadDescriptor(md).PayloadType
	if descriptor := unwrapContentDescriptor(spec.Body); resource.IsPayloadDescriptorExplicit(descriptor) {
		payloadType = resource.NormalizePayloadDescriptor(descriptor).PayloadType
	}
	if strings.TrimSpace(md.Format) == "" || metadata.ResourceFormatAllowsMixedItems(md.Format) {
		if inferred, ok := resource.PayloadTypeForMediaType(spec.ContentType); ok {
			payloadType = inferred
		}
	}
	if !resource.IsStructuredPayloadType(payloadType) {
		return faults.Invalid(
			fmt.Sprintf("operation payload validation requires structured payloads, got %q", payloadType),
			nil,
		)
	}

	normalizedBody, err := resource.Normalize(unwrapContentValue(spec.Body))
	if err != nil {
		return err
	}
	derivedFields := metadatavalidation.DerivePathFields(resolvedResource, md)

	payloadView := normalizedBody
	if len(derivedFields) > 0 {
		payloadView = metadatavalidation.MergePayloadFields(normalizedBody, derivedFields)
	}

	if err := metadatavalidation.ValidateRequiredAttributes(
		payloadView,
		"validate.requiredAttributes",
		spec.Validate.RequiredAttributes,
		"operation payload validation",
	); err != nil {
		return err
	}
	if err := g.validateOperationAssertions(ctx, payloadView, spec.Validate.Assertions); err != nil {
		return err
	}
	if err := g.validateOperationSchemaRef(
		ctx,
		normalizedBody,
		derivedFields,
		spec.Path,
		spec.Method,
		spec.Validate.SchemaRef,
	); err != nil {
		return err
	}

	return nil
}

func (g *Client) validateResourceMutationPayload(
	operation metadata.Operation,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
	descriptor resource.PayloadDescriptor,
) error {
	if len(metadatavalidation.EffectiveResourceRequiredAttributesForOperation(md, operation)) == 0 {
		return nil
	}

	payloadType := resource.NormalizePayloadDescriptor(descriptor).PayloadType
	if !resource.IsStructuredPayloadType(payloadType) {
		return nil
	}

	return metadatavalidation.ValidateResourceRequiredAttributesForOperation(resolvedResource.Payload, md, operation)
}
