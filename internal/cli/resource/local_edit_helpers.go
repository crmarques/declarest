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

package resource

import (
	"context"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/metadata"
	resourcedomain "github.com/crmarques/declarest/resource"
)

func resolveActiveResourceContext(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
) (configdomain.Context, error) {
	contexts, err := cliutil.RequireContexts(deps)
	if err != nil {
		return configdomain.Context{}, err
	}

	contextName := ""
	if globalFlags != nil {
		contextName = strings.TrimSpace(globalFlags.Context)
	}
	if contextName == "" {
		contextName = strings.TrimSpace(cliutil.ContextName(ctx))
	}

	return contexts.ResolveContext(ctx, configdomain.ContextSelection{Name: contextName})
}

func resourcePayloadEditType(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	cfg configdomain.Context,
	logicalPath string,
	value resourcedomain.Value,
) (string, error) {
	_ = cfg
	if deps.Services != nil && deps.Services.MetadataService() != nil {
		md, err := deps.Services.MetadataService().ResolveForPath(ctx, logicalPath)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(md.Format) != "" {
			return metadata.EffectivePayloadType(md, resourcedomain.PayloadTypeJSON)
		}
	}

	if _, ok := resourcedomain.BinaryBytes(value); ok {
		return resourcedomain.PayloadTypeOctetStream, nil
	}
	if _, ok := value.(string); ok {
		return resourcedomain.PayloadTypeText, nil
	}

	return resourcedomain.PayloadTypeJSON, nil
}

func resourcePayloadEditFilename(payloadType string) string {
	extension, err := resourcedomain.PayloadExtension(payloadType)
	if err != nil {
		return "resource.json"
	}
	return "resource" + extension
}

func validateEditPayloadType(payloadType string) error {
	if resourcedomain.IsBinaryPayloadType(payloadType) {
		return cliutil.ValidationError("resource edit does not support octet-stream payloads; use file or stdin based mutation commands", nil)
	}
	return nil
}

func encodeResourcePayloadForEdit(payloadType string, value resourcedomain.Value) ([]byte, error) {
	if err := validateEditPayloadType(payloadType); err != nil {
		return nil, err
	}
	return resourcedomain.EncodePayloadPretty(value, payloadType)
}

func decodeResourcePayloadFromEdit(payloadType string, data []byte) (resourcedomain.Value, error) {
	if err := validateEditPayloadType(payloadType); err != nil {
		return nil, err
	}
	return resourcedomain.DecodePayload(data, payloadType)
}
