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

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Credential struct {
	Name     string          `json:"name" yaml:"name"`
	Username CredentialValue `json:"username" yaml:"username"`
	Password CredentialValue `json:"password" yaml:"password"`
}

type CredentialsRef struct {
	Name string `json:"name" yaml:"name"`
}

type CredentialValue struct {
	Value  string            `json:"-" yaml:"-"`
	Prompt *CredentialPrompt `json:"-" yaml:"-"`
}

type CredentialPrompt struct {
	Prompt           bool `json:"prompt" yaml:"prompt"`
	PersistInSession bool `json:"persistInSession,omitempty" yaml:"persistInSession,omitempty"`
}

func LiteralCredential(value string) CredentialValue {
	return CredentialValue{Value: strings.TrimSpace(value)}
}

func (v CredentialValue) IsPrompt() bool {
	return v.Prompt != nil && v.Prompt.Prompt
}

func (v CredentialValue) PersistInSession() bool {
	return v.Prompt != nil && v.Prompt.PersistInSession
}

func (v CredentialValue) Literal() string {
	return strings.TrimSpace(v.Value)
}

func (a *BasicAuth) CredentialName() string {
	if a == nil || a.CredentialsRef == nil {
		return ""
	}
	return strings.TrimSpace(a.CredentialsRef.Name)
}

func (a *BasicAuth) UsesPrompt() bool {
	if a == nil {
		return false
	}
	return a.Username.IsPrompt() || a.Password.IsPrompt()
}

func (a *BasicAuth) HasResolvedCredentials() bool {
	if a == nil {
		return false
	}
	return a.Username.Literal() != "" && a.Password.Literal() != ""
}

func (a *VaultUserPasswordAuth) CredentialName() string {
	if a == nil || a.CredentialsRef == nil {
		return ""
	}
	return strings.TrimSpace(a.CredentialsRef.Name)
}

func (a *VaultUserPasswordAuth) UsesPrompt() bool {
	if a == nil {
		return false
	}
	return a.Username.IsPrompt() || a.Password.IsPrompt()
}

func (a *VaultUserPasswordAuth) HasResolvedCredentials() bool {
	if a == nil {
		return false
	}
	return a.Username.Literal() != "" && a.Password.Literal() != ""
}

func (v CredentialValue) MarshalJSON() ([]byte, error) {
	if v.Prompt != nil {
		return json.Marshal(v.Prompt)
	}
	return json.Marshal(v.Value)
}

func (v *CredentialValue) UnmarshalJSON(data []byte) error {
	if v == nil {
		return nil
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*v = CredentialValue{}
		return nil
	}
	if trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
		*v = CredentialValue{Value: value}
		return nil
	}

	var prompt CredentialPrompt
	if err := json.Unmarshal(trimmed, &prompt); err != nil {
		return err
	}
	*v = CredentialValue{Prompt: &prompt}
	return nil
}

func (v CredentialValue) MarshalYAML() (any, error) {
	if v.Prompt != nil {
		return v.Prompt, nil
	}
	return v.Value, nil
}

func (v *CredentialValue) UnmarshalYAML(node *yaml.Node) error {
	if v == nil {
		return nil
	}
	if node == nil {
		*v = CredentialValue{}
		return nil
	}

	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return err
		}
		*v = CredentialValue{Value: value}
		return nil
	case yaml.MappingNode:
		var prompt CredentialPrompt
		if err := node.Decode(&prompt); err != nil {
			return err
		}
		*v = CredentialValue{Prompt: &prompt}
		return nil
	default:
		return fmt.Errorf("credential value must be a string or prompt object")
	}
}
