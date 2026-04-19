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

import "context"

// ContextCatalogEditor is an optional capability for commands that edit the
// full persisted catalog while preserving strict validation.
type ContextCatalogEditor interface {
	GetCatalog(ctx context.Context) (ContextCatalog, error)
	ReplaceCatalog(ctx context.Context, catalog ContextCatalog) error
}

type ContextService interface {
	Create(ctx context.Context, cfg Context) error
	Update(ctx context.Context, cfg Context) error
	Delete(ctx context.Context, name string) error
	Rename(ctx context.Context, fromName string, toName string) error
	SetCurrent(ctx context.Context, name string) error
	List(ctx context.Context) ([]Context, error)
	GetCurrent(ctx context.Context) (Context, error)
	ResolveContext(ctx context.Context, selection ContextSelection) (Context, error)
	Validate(ctx context.Context, cfg Context) error
}
