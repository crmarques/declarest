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

package secret

import (
	"encoding/json"
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func decodeDetectInput(command *cobra.Command, flags cliutil.InputFlags) (resource.Value, bool, error) {
	data, err := cliutil.ReadInput(command, flags)
	if err != nil {
		if isInputRequiredError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var value resource.Value
	switch resolveSecretInputPayloadType(flags.ContentType, flags.Payload) {
	case cliutil.OutputJSON:
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, false, cliutil.ValidationError("invalid json input", err)
		}
	case cliutil.OutputYAML:
		if err := yaml.Unmarshal(data, &value); err != nil {
			return nil, false, cliutil.ValidationError("invalid yaml input", err)
		}
	default:
		return nil, false, cliutil.ValidationError("invalid input content type: use json, yaml, application/json, or application/yaml", nil)
	}

	return value, true, nil
}

func resolveSecretInputPayloadType(contentType string, payloadPath string) string {
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	switch normalized {
	case "json", "application/json":
		return cliutil.OutputJSON
	case "yaml", "application/yaml", "text/yaml", "application/x-yaml", "text/x-yaml":
		return cliutil.OutputYAML
	}

	lowerPath := strings.ToLower(strings.TrimSpace(payloadPath))
	switch {
	case strings.HasSuffix(lowerPath, ".json"):
		return cliutil.OutputJSON
	case strings.HasSuffix(lowerPath, ".yaml"), strings.HasSuffix(lowerPath, ".yml"):
		return cliutil.OutputYAML
	default:
		return cliutil.OutputJSON
	}
}

func isInputRequiredError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "input is required")
}
