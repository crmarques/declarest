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

package input

import (
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

// DecodeOptionalPayloadInput decodes a payload value from --payload or stdin.
// It returns hasInput=false when no input source was provided.
func DecodeOptionalPayloadInput(
	command *cobra.Command,
	flags cliutil.InputFlags,
) (resource.Content, bool, error) {
	data, err := cliutil.ReadOptionalInput(command, flags)
	if err != nil {
		return resource.Content{}, false, err
	}
	if data == nil {
		return resource.Content{}, false, nil
	}
	value, err := cliutil.DecodeResourceContentInputData(data, flags.ContentType, flags.Payload)
	if err != nil {
		return resource.Content{}, false, err
	}
	return value, true, nil
}

// DecodeRequiredPayloadInput decodes a required payload value from --payload or stdin.
func DecodeRequiredPayloadInput(
	command *cobra.Command,
	flags cliutil.InputFlags,
) (resource.Content, error) {
	value, hasInput, err := DecodeOptionalPayloadInput(command, flags)
	if err != nil {
		return resource.Content{}, err
	}
	if !hasInput {
		return resource.Content{}, cliutil.ValidationError(cliutil.MissingInputMessage, nil)
	}
	return value, nil
}
