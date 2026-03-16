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
	"fmt"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

type singleContextEditView struct {
	DefaultEditor string                    `yaml:"defaultEditor,omitempty"`
	Credentials   []configdomain.Credential `yaml:"credentials,omitempty"`
	Context       configdomain.Context      `yaml:"context"`
}

func compactContextCatalogForView(catalog configdomain.ContextCatalog) configdomain.ContextCatalog {
	return configdomain.CompactContextCatalog(catalog)
}

func selectContextCatalogForShow(catalog configdomain.ContextCatalog, name string) (configdomain.ContextCatalog, error) {
	for _, item := range catalog.Contexts {
		if item.Name != name {
			continue
		}

		shown := catalog
		shown.Contexts = []configdomain.Context{item}
		shown.CurrentContext = item.Name
		return shown, nil
	}
	return configdomain.ContextCatalog{}, faults.NewTypedError(
		faults.NotFoundError,
		fmt.Sprintf("context %q not found", name),
		nil,
	)
}

func selectContextForView(contexts []configdomain.Context, name string) (configdomain.Context, int, error) {
	for idx, item := range contexts {
		if item.Name != name {
			continue
		}
		return configdomain.CompactContext(item), idx, nil
	}
	return configdomain.Context{}, -1, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("context %q not found", name), nil)
}

func selectSingleContextEditView(
	catalog configdomain.ContextCatalog,
	name string,
) (singleContextEditView, int, error) {
	viewContext, idx, err := selectContextForView(catalog.Contexts, name)
	if err != nil {
		return singleContextEditView{}, -1, err
	}

	return singleContextEditView{
		DefaultEditor: catalog.DefaultEditor,
		Credentials:   append([]configdomain.Credential(nil), catalog.Credentials...),
		Context:       viewContext,
	}, idx, nil
}

func applySingleContextEditView(
	catalog configdomain.ContextCatalog,
	idx int,
	edited singleContextEditView,
) configdomain.ContextCatalog {
	if idx < 0 || idx >= len(catalog.Contexts) {
		return catalog
	}

	oldName := catalog.Contexts[idx].Name
	catalog.DefaultEditor = edited.DefaultEditor
	catalog.Credentials = append([]configdomain.Credential(nil), edited.Credentials...)
	catalog.Contexts[idx] = edited.Context
	if catalog.CurrentContext == oldName && strings.TrimSpace(edited.Context.Name) != "" {
		catalog.CurrentContext = edited.Context.Name
	}
	return catalog
}
