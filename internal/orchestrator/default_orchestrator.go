package orchestrator

import (
	"strings"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

var _ orchestrator.Orchestrator = (*Orchestrator)(nil)

type Orchestrator struct {
	repository    repository.ResourceStore
	metadata      metadata.MetadataService
	server        managedserver.ManagedServerClient
	secrets       secrets.SecretProvider
	defaultFormat string
}

// Option configures optional orchestrator settings.
type Option func(*Orchestrator)

// WithDefaultFormat sets a default payload format for the
// orchestrator. This value is used during save operations when neither the
// resource content descriptor nor the per-collection metadata specifies a
// concrete default format.
func WithDefaultFormat(format string) Option {
	return func(o *Orchestrator) {
		o.defaultFormat = format
	}
}

func New(
	repo repository.ResourceStore,
	meta metadata.MetadataService,
	srv managedserver.ManagedServerClient,
	sec secrets.SecretProvider,
	opts ...Option,
) *Orchestrator {
	o := &Orchestrator{
		repository: repo,
		metadata:   meta,
		server:     srv,
		secrets:    sec,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (r *Orchestrator) RepositoryStore() repository.ResourceStore {
	if r == nil {
		return nil
	}
	return r.repository
}

func (r *Orchestrator) RepositorySync() repository.RepositorySync {
	if r == nil || r.repository == nil {
		return nil
	}
	if sync, ok := r.repository.(repository.RepositorySync); ok {
		return sync
	}
	return nil
}

func (r *Orchestrator) MetadataService() metadata.MetadataService {
	if r == nil {
		return nil
	}
	return r.metadata
}

func (r *Orchestrator) ManagedServerClient() managedserver.ManagedServerClient {
	if r == nil {
		return nil
	}
	return r.server
}

func (r *Orchestrator) SecretProvider() secrets.SecretProvider {
	if r == nil {
		return nil
	}
	return r.secrets
}

// applyDefaultFormat applies the default format to the content descriptor
// when the descriptor is not already explicitly set. The per-collection
// metadata DefaultFormat takes priority over the orchestrator-level default
// (which comes from context preferences or the bundle manifest).
func (r *Orchestrator) applyDefaultFormat(content resource.Content, md metadata.ResourceMetadata) resource.Content {
	if resource.IsPayloadDescriptorExplicit(content.Descriptor) {
		return content
	}

	format := r.resolveDefaultFormat(md)
	if format == "" || metadata.ResourceDefaultFormatAllowsMixedItems(format) {
		return content
	}

	content.Descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: format})
	return content
}

// resolveDefaultFormat returns the effective default format using the
// precedence: metadata file > orchestrator default (context preferences > bundle).
func (r *Orchestrator) resolveDefaultFormat(md metadata.ResourceMetadata) string {
	if candidate := strings.TrimSpace(md.DefaultFormat); candidate != "" {
		return candidate
	}
	if r != nil {
		return strings.TrimSpace(r.defaultFormat)
	}
	return ""
}
