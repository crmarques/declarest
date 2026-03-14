package fsmetadata

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

var _ metadatadomain.MetadataService = (*LayeredMetadataService)(nil)
var _ metadatadomain.ResourceOperationSpecRenderer = (*LayeredMetadataService)(nil)
var _ metadatadomain.DefaultsArtifactStore = (*LayeredMetadataService)(nil)
var _ metadatadomain.CollectionChildrenResolver = (*LayeredMetadataService)(nil)
var _ metadatadomain.CollectionWildcardResolver = (*LayeredMetadataService)(nil)

type LayeredMetadataService struct {
	shared   *FSMetadataService
	local    *FSMetadataService
	writable *FSMetadataService
}

func NewLayeredFSMetadataService(sharedBaseDir string, localBaseDir string) *LayeredMetadataService {
	sharedBaseDir = strings.TrimSpace(sharedBaseDir)
	localBaseDir = strings.TrimSpace(localBaseDir)

	var shared *FSMetadataService
	if sharedBaseDir != "" {
		shared = NewFSMetadataService(sharedBaseDir)
	}

	var local *FSMetadataService
	if localBaseDir != "" {
		local = NewFSMetadataService(localBaseDir)
	}

	writable := local
	if writable == nil {
		writable = shared
	}

	return &LayeredMetadataService{
		shared:   shared,
		local:    local,
		writable: writable,
	}
}

func (s *LayeredMetadataService) Get(ctx context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if s == nil || s.writable == nil {
		return metadatadomain.ResourceMetadata{}, notFoundError("metadata service is not configured")
	}
	return s.writable.Get(ctx, logicalPath)
}

func (s *LayeredMetadataService) Set(ctx context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	if s == nil || s.writable == nil {
		return faults.NewValidationError("metadata service is not configured", nil)
	}
	return s.writable.Set(ctx, logicalPath, metadata)
}

func (s *LayeredMetadataService) Unset(ctx context.Context, logicalPath string) error {
	if s == nil || s.writable == nil {
		return nil
	}
	return s.writable.Unset(ctx, logicalPath)
}

func (s *LayeredMetadataService) ResolveForPath(ctx context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	merged := metadatadomain.ResourceMetadata{}

	if s != nil && s.shared != nil {
		resolved, err := s.shared.ResolveForPath(ctx, logicalPath)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
		merged = metadatadomain.MergeResourceMetadata(merged, resolved)
	}
	if s != nil && s.local != nil {
		resolved, err := s.local.ResolveForPath(ctx, logicalPath)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
		merged = metadatadomain.MergeResourceMetadata(merged, resolved)
	}

	return merged, nil
}

func (s *LayeredMetadataService) RenderOperationSpec(
	ctx context.Context,
	logicalPath string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	target, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	resolvedMetadata, err := s.ResolveForPath(ctx, logicalPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	templateValue, err := buildTemplateValue(target.path, resolvedMetadata, value, operation)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}
	metadatadomain.ApplyPayloadTemplateScope(templateValue, resolvedMetadata, value, resource.PayloadDescriptor{})

	return metadatadomain.ResolveOperationSpecWithScope(ctx, resolvedMetadata, operation, templateValue)
}

func (s *LayeredMetadataService) RenderOperationSpecForResource(
	ctx context.Context,
	input metadatadomain.ResourceOperationSpecInput,
	operation metadatadomain.Operation,
) (metadatadomain.OperationSpec, error) {
	resolvedResource := resource.Resource{
		LogicalPath:       input.LogicalPath,
		CollectionPath:    input.CollectionPath,
		LocalAlias:        input.LocalAlias,
		RemoteID:          input.RemoteID,
		Payload:           input.Payload,
		PayloadDescriptor: input.PayloadDescriptor,
	}

	target, err := normalizeResolvePath(resolvedResource.LogicalPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	resolvedMetadata := metadatadomain.CloneResourceMetadata(input.Metadata)
	if !metadatadomain.HasResourceMetadataDirectives(resolvedMetadata) {
		resolvedMetadata, err = s.ResolveForPath(ctx, resolvedResource.LogicalPath)
		if err != nil {
			if faults.IsCategory(err, faults.NotFoundError) {
				resolvedMetadata = metadatadomain.ResourceMetadata{}
			} else {
				return metadatadomain.OperationSpec{}, err
			}
		}
	}

	templateScope, err := buildTemplateScopeForResource(target.path, resolvedMetadata, resolvedResource, operation)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}
	metadatadomain.ApplyPayloadTemplateScope(
		templateScope,
		resolvedMetadata,
		resolvedResource.Payload,
		resolvedResource.PayloadDescriptor,
	)

	return metadatadomain.ResolveOperationSpecWithScope(ctx, resolvedMetadata, operation, templateScope)
}

func (s *LayeredMetadataService) ReadDefaultsArtifact(ctx context.Context, logicalPath string, file string) (resource.Content, error) {
	if s == nil || s.writable == nil {
		return resource.Content{}, notFoundError("resource defaults artifacts are not configured")
	}
	return s.writable.ReadDefaultsArtifact(ctx, logicalPath, file)
}

func (s *LayeredMetadataService) WriteDefaultsArtifact(ctx context.Context, logicalPath string, file string, content resource.Content) error {
	if s == nil || s.writable == nil {
		return faults.NewValidationError("resource defaults artifacts are not configured", nil)
	}
	return s.writable.WriteDefaultsArtifact(ctx, logicalPath, file, content)
}

func (s *LayeredMetadataService) DeleteDefaultsArtifact(ctx context.Context, logicalPath string, file string) error {
	if s == nil || s.writable == nil {
		return nil
	}
	return s.writable.DeleteDefaultsArtifact(ctx, logicalPath, file)
}

func (s *LayeredMetadataService) ResolveCollectionChildren(ctx context.Context, logicalPath string) ([]string, error) {
	children := map[string]struct{}{}

	if s != nil && s.shared != nil {
		values, err := s.shared.ResolveCollectionChildren(ctx, logicalPath)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			children[value] = struct{}{}
		}
	}
	if s != nil && s.local != nil {
		values, err := s.local.ResolveCollectionChildren(ctx, logicalPath)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			children[value] = struct{}{}
		}
	}

	return sortedSelectorKeys(children), nil
}

func (s *LayeredMetadataService) HasCollectionWildcardChild(ctx context.Context, logicalPath string) (bool, error) {
	if s != nil && s.local != nil {
		ok, err := s.local.HasCollectionWildcardChild(ctx, logicalPath)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	if s != nil && s.shared != nil {
		return s.shared.HasCollectionWildcardChild(ctx, logicalPath)
	}
	return false, nil
}
