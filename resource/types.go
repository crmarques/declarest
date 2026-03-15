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

type Value = any

type PayloadDescriptor struct {
	PayloadType string
	MediaType   string
	Extension   string
}

type Content struct {
	Value      Value
	Descriptor PayloadDescriptor
}

type Resource struct {
	LogicalPath        string
	CollectionPath     string
	LocalAlias         string
	RemoteID           string
	ResolvedRemotePath string
	Payload            Value
	PayloadDescriptor  PayloadDescriptor
}

type DiffEntry struct {
	ResourcePath string
	Path         string
	Operation    string
	Local        Value
	Remote       Value
}
