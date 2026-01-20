package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/reconciler"
)

var errOpenAPISpecNotConfigured = errors.New("openapi spec is not configured")

type openAPISpecProvider interface {
	OpenAPISpec() *openapi.Spec
}

func resolveOpenAPISpec(recon *reconciler.DefaultReconciler, specSource string) (*openapi.Spec, error) {
	if recon == nil {
		return nil, errors.New("reconciler is not configured")
	}

	trimmed := strings.TrimSpace(specSource)
	if trimmed == "" {
		if provider, ok := recon.ResourceRecordProvider.(openAPISpecProvider); ok {
			if spec := provider.OpenAPISpec(); spec != nil {
				return spec, nil
			}
		}
		return nil, errOpenAPISpecNotConfigured
	}
	return loadOpenAPISpecFromSource(recon, trimmed)
}

func loadOpenAPISpecFromSource(recon *reconciler.DefaultReconciler, source string) (*openapi.Spec, error) {
	if isHTTPURL(source) {
		httpManager, ok := recon.ResourceServerManager.(*managedserver.HTTPResourceServerManager)
		if !ok || httpManager == nil {
			return nil, errors.New("openapi source requires an http managed server")
		}
		data, err := httpManager.LoadOpenAPISpec(source)
		if err != nil {
			return nil, err
		}
		return parseOpenAPISpec(source, data)
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	return parseOpenAPISpec(source, data)
}

func parseOpenAPISpec(source string, data []byte) (*openapi.Spec, error) {
	spec, err := openapi.ParseSpec(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse openapi spec %q: %w", source, err)
	}
	return spec, nil
}

func isHTTPURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}
