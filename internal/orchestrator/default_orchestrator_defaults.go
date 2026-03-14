package orchestrator

import (
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func effectiveMetadataDefaults(
	md metadata.ResourceMetadata,
	fallback resource.PayloadDescriptor,
) (resource.Content, bool, error) {
	if !metadata.HasDefaultsSpecDirectives(md.Defaults) {
		return resource.Content{}, false, nil
	}

	value, err := metadata.ResolveEffectiveDefaults(md.Defaults)
	if err != nil {
		return resource.Content{}, false, err
	}

	descriptor := fallback
	if !resource.IsPayloadDescriptorExplicit(descriptor) && strings.TrimSpace(md.PayloadType) != "" {
		descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: md.PayloadType})
	}
	if !resource.IsPayloadDescriptorExplicit(descriptor) {
		descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}

	return resource.Content{
		Value:      value,
		Descriptor: descriptor,
	}, true, nil
}

func mergeContentWithMetadataDefaults(
	content resource.Content,
	md metadata.ResourceMetadata,
) (resource.Content, bool, error) {
	defaultsContent, found, err := effectiveMetadataDefaults(md, content.Descriptor)
	if err != nil || !found {
		return content, found, err
	}

	mergedValue, err := resource.MergeWithDefaults(defaultsContent.Value, content.Value)
	if err != nil {
		return resource.Content{}, false, err
	}
	return resource.Content{
		Value:      mergedValue,
		Descriptor: content.Descriptor,
	}, true, nil
}

func compactContentWithMetadataDefaults(
	content resource.Content,
	md metadata.ResourceMetadata,
	preservePresence bool,
) (resource.Content, bool, error) {
	defaultsContent, found, err := effectiveMetadataDefaults(md, content.Descriptor)
	if err != nil || !found {
		return content, found, err
	}

	compactedValue, err := resource.CompactAgainstDefaults(content.Value, defaultsContent.Value)
	if err != nil {
		return resource.Content{}, false, err
	}
	if compactedValue == nil && preservePresence {
		compactedValue = map[string]any{}
	}
	return resource.Content{
		Value:      compactedValue,
		Descriptor: content.Descriptor,
	}, true, nil
}
