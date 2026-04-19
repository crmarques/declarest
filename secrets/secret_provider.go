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

package secrets

import (
	"context"

	"github.com/crmarques/declarest/resource"
)

type SecretProvider interface {
	Init(ctx context.Context) error
	Store(ctx context.Context, key string, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
	MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error)
	ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error)
	NormalizeSecretPlaceholders(ctx context.Context, value resource.Value) (resource.Value, error)
	DetectSecretCandidates(ctx context.Context, value resource.Value) ([]string, error)
}
